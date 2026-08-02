package service

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"aimoc-backend/internal/domain"
	"aimoc-backend/pkg/aivision"
	"aimoc-backend/pkg/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AIVisionService — jembatan ke pipeline AI Vision baru tim AI (excavator_vlm, Gracia
// BCS): YOLO+BoT-SORT+LSTM asli yang mendeteksi truk secara visual, dijalankan sebagai
// job upload+async GPU queue di service Python eksternal (pkg/aivision). Beda dari
// loading_cycles kita sendiri yang cuma mengelompokkan bucket_events dari jeda waktu.
//
// Dua jalur trigger:
//   - TriggerAsync: otomatis, dipanggil CCTVService.RecordLoadingCycle tiap 1 siklus
//     loading selesai -- ambil klip dari RecordingsDir (VOD Sync), fire-and-forget.
//   - TriggerManual: dipanggil handler POST /ai-vision/manual, video sudah disiapkan
//     user (upload atau file test yang sudah ada).
//
// Pola sama VODSyncService/RecordLoadingCycle: goroutine terpisah, tidak blocking
// caller, gagal-diam-diam-tapi-tercatat (log + baris status=failed), bukan panik.
type AIVisionService struct {
	DB             *gorm.DB
	Client         *aivision.Client
	RecordingsDir  string
	Enabled        bool
	MinDurationSec int
}

func NewAIVisionService(db *gorm.DB, client *aivision.Client, recordingsDir string, enabled bool, minDurationSec int) *AIVisionService {
	return &AIVisionService{
		DB:             db,
		Client:         client,
		RecordingsDir:  recordingsDir,
		Enabled:        enabled,
		MinDurationSec: minDurationSec,
	}
}

// recordingFilenameRe — konvensi BlackVue sama persis dengan pkg/blackvue/vod.go,
// disalin di sini (bukan diekspor dari sana) karena beda konteks: vod.go mem-parsing
// listing HTTP dari Fleet Server, ini mem-parsing nama file lokal hasil VOD Sync.
var recordingFilenameRe = regexp.MustCompile(`^(\d{8})_(\d{6})_[NPEM]F\.mp4$`)

// jakarta — sama alasan dengan pkg/blackvue/vod.go: nama file rekaman pakai jam device
// lokal (Asia/Jakarta), sedangkan LoadingCycle.StartTS/EndTS dari cycle_detector.py
// (ai-service) UTC. WAJIB dikonversi sebelum dibandingkan, atau salah pilih segmen
// (offset 7 jam).
var jakarta = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}()

type recordingSegment struct {
	Path string
	TS   time.Time // waktu mulai segmen, jam device lokal (Asia/Jakarta)
}

// listSegments membaca RecordingsDir/<cameraCode>/*.mp4, urut berdasarkan waktu mulai.
func listSegments(recordingsDir, cameraCode string) ([]recordingSegment, error) {
	dir := filepath.Join(recordingsDir, cameraCode)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("gagal baca folder rekaman %s: %w", dir, err)
	}
	var segs []recordingSegment
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := recordingFilenameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		ts, err := time.ParseInLocation("20060102150405", m[1]+m[2], jakarta)
		if err != nil {
			continue
		}
		segs = append(segs, recordingSegment{Path: filepath.Join(dir, e.Name()), TS: ts})
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].TS.Before(segs[j].TS) })
	return segs, nil
}

// ExtractClip mencari segmen VOD (hasil VODSyncService) yang overlap window
// [start-pad, end+pad] untuk 1 kamera, lalu menggabungkannya jadi 1 file mp4 di temp
// dir (ffmpeg concat kalau >1 segmen, copy langsung kalau cuma 1). Caller wajib hapus
// file hasil setelah dipakai (bukan segmen asli -- itu tetap milik VODSyncService).
func ExtractClip(recordingsDir, cameraCode string, start, end time.Time) (string, error) {
	const pad = 75 * time.Second // segmen ~1 menit, kasih slack di kedua ujung
	windowStart := start.In(jakarta).Add(-pad)
	windowEnd := end.In(jakarta).Add(pad)

	segs, err := listSegments(recordingsDir, cameraCode)
	if err != nil {
		return "", err
	}
	var matched []recordingSegment
	for _, s := range segs {
		if !s.TS.Before(windowStart) && !s.TS.After(windowEnd) {
			matched = append(matched, s)
		}
	}
	if len(matched) == 0 {
		return "", fmt.Errorf("tidak ada segmen rekaman kamera %s pada window %s..%s", cameraCode, windowStart, windowEnd)
	}

	outDir := filepath.Join(os.TempDir(), "aivision-clips")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("gagal buat temp dir klip: %w", err)
	}
	outPath := filepath.Join(outDir, uuid.NewString()+".mp4")

	if len(matched) == 1 {
		if err := copyFile(matched[0].Path, outPath); err != nil {
			return "", fmt.Errorf("gagal salin segmen tunggal: %w", err)
		}
		return outPath, nil
	}
	if err := concatSegments(matched, outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// concatSegments menggabungkan >1 segmen berurutan lewat ffmpeg concat demuxer (-c copy,
// tanpa re-encode -- semua segmen sumbernya sama, dashcam BlackVue yang sama).
func concatSegments(segs []recordingSegment, outPath string) error {
	listPath := outPath + ".concat.txt"
	var lines string
	for _, s := range segs {
		lines += fmt.Sprintf("file '%s'\n", filepath.ToSlash(s.Path))
	}
	if err := os.WriteFile(listPath, []byte(lines), 0644); err != nil {
		return fmt.Errorf("gagal tulis daftar concat: %w", err)
	}
	defer os.Remove(listPath)

	cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg concat gagal: %w (%s)", err, string(out))
	}
	return nil
}

// TriggerAsync dipanggil CCTVService.RecordLoadingCycle setelah 1 loading_cycles
// tersimpan. Fire-and-forget: tidak menunda/menggagalkan pencatatan siklus itu sendiri.
func (s *AIVisionService) TriggerAsync(cycle domain.LoadingCycle) {
	if !s.Enabled || s.Client == nil {
		return
	}
	if cycle.DurationSec < s.MinDurationSec {
		return // siklus terlalu pendek, kemungkinan noise -- jangan buang antrean GPU
	}
	if cycle.CameraID == nil {
		return // tidak tahu kamera mana -- tidak ada video untuk diambil
	}

	var cam domain.Camera
	if err := s.DB.First(&cam, "id = ?", *cycle.CameraID).Error; err != nil {
		return
	}
	unitID := cam.Code
	if cycle.ExcavatorID != nil {
		var exc domain.Excavator
		if err := s.DB.First(&exc, "id = ?", *cycle.ExcavatorID).Error; err == nil && exc.Code != "" {
			unitID = exc.Code
		}
	}

	row := domain.AIVisionAnalysis{
		LoadingCycleID: &cycle.ID,
		TriggerSource:  "auto",
		Status:         "queued",
		UnitID:         unitID,
		SubmittedAt:    time.Now(),
	}
	if err := s.DB.Create(&row).Error; err != nil {
		if utils.Log != nil {
			utils.Log.Warn("ai vision: gagal simpan baris awal", zap.Error(err))
		}
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil && utils.Log != nil {
				utils.Log.Error("ai vision trigger panic", zap.Any("recover", r))
			}
		}()
		clipPath, err := ExtractClip(s.RecordingsDir, cam.Code, cycle.StartTS, cycle.EndTS)
		if err != nil {
			if utils.Log != nil {
				utils.Log.Warn("ai vision: gagal ambil klip", zap.String("camera", cam.Code), zap.Error(err))
			}
			s.markFailed(row.ID, err.Error())
			return
		}
		defer os.Remove(clipPath)
		s.submitAndPoll(row.ID, clipPath, unitID, cycle.StartTS.Format(time.RFC3339))
	}()
}

// TriggerManual dipanggil handler POST /ai-vision/manual. videoPath sudah siap pakai
// (file upload yang sudah disimpan, atau file test yang sudah ada di TestVideosDir) --
// TIDAK dihapus di sini (beda dari klip otomatis yang memang temp file), dan TIDAK
// difilter MinDurationSec (video manual sengaja dipilih user).
func (s *AIVisionService) TriggerManual(unitID, label, videoPath string) (*domain.AIVisionAnalysis, error) {
	if !s.Enabled || s.Client == nil {
		return nil, fmt.Errorf("AI Vision belum dikonfigurasi (AI_VISION_ENABLED=false)")
	}
	if _, err := os.Stat(videoPath); err != nil {
		return nil, fmt.Errorf("video tidak ditemukan: %w", err)
	}

	row := domain.AIVisionAnalysis{
		TriggerSource: "manual",
		Label:         label,
		Status:        "queued",
		UnitID:        unitID,
		SubmittedAt:   time.Now(),
	}
	if err := s.DB.Create(&row).Error; err != nil {
		return nil, fmt.Errorf("gagal simpan baris analisa: %w", err)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil && utils.Log != nil {
				utils.Log.Error("ai vision manual trigger panic", zap.Any("recover", r))
			}
		}()
		s.submitAndPoll(row.ID, videoPath, unitID, time.Now().Format(time.RFC3339))
	}()

	return &row, nil
}

// submitAndPoll upload video -> submit job -> poll berkala sampai selesai/gagal/timeout,
// mengupdate baris ai_vision_analyses di tiap langkah.
func (s *AIVisionService) submitAndPoll(rowID uuid.UUID, videoPath, unitID, recordedAt string) {
	uploaded, err := s.Client.UploadVideo(videoPath)
	if err != nil {
		s.markFailed(rowID, err.Error())
		return
	}
	job, err := s.Client.SubmitJob(uploaded.VideoID, uploaded.Filename, unitID, recordedAt)
	if err != nil {
		s.markFailed(rowID, err.Error())
		return
	}
	s.DB.Model(&domain.AIVisionAnalysis{}).Where("id = ?", rowID).Updates(map[string]interface{}{
		"external_job_id": job.JobID,
		"status":          "running",
	})

	deadline := time.Now().Add(20 * time.Minute) // ~1.3x durasi video + buffer antrean GPU
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		if time.Now().After(deadline) {
			s.markFailed(rowID, "timeout menunggu hasil AI Vision (>20 menit)")
			return
		}
		polled, err := s.Client.GetJob(job.JobID)
		if err != nil {
			continue // transient -- coba lagi di tick berikutnya, jangan langsung gagal
		}
		switch polled.Status {
		case "completed":
			artifactPath := ""
			if polled.ArtifactURLs != nil {
				if _, ok := polled.ArtifactURLs["annotated_video"]; ok {
					artifactPath = "annotated_video"
				}
			}
			now := time.Now()
			s.DB.Model(&domain.AIVisionAnalysis{}).Where("id = ?", rowID).Updates(map[string]interface{}{
				"status":               "completed",
				"dashboard_summary":    string(polled.Dashboard),
				"annotated_video_path": artifactPath,
				"finished_at":          &now,
			})
			return
		case "failed", "cancelled":
			msg := polled.Error
			if msg == "" {
				msg = polled.Message
			}
			s.markFailed(rowID, msg)
			return
		}
		// queued/running -- lanjut poll
	}
}

func (s *AIVisionService) markFailed(rowID uuid.UUID, message string) {
	now := time.Now()
	s.DB.Model(&domain.AIVisionAnalysis{}).Where("id = ?", rowID).Updates(map[string]interface{}{
		"status":        "failed",
		"error_message": message,
		"finished_at":   &now,
	})
	if utils.Log != nil {
		utils.Log.Warn("ai vision: job gagal", zap.String("row_id", rowID.String()), zap.String("error", message))
	}
}
