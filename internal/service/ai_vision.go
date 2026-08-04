package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aimoc-backend/internal/domain"
	"aimoc-backend/pkg/aivision"
	"aimoc-backend/pkg/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// maxUploadWidth -- batas lebar video yang dikirim ke AI Vision (permintaan tim AI,
// Gracia, 02 Agu 2026: "resolusinya jangan terlalu gede ... minimal 640 sampe 960 biar
// ga terlalu berat"). Cuma jadi batas ATAS -- video yang sudah lebih kecil TIDAK
// di-upscale (upscale cuma nambah blur, bukan detail asli, tidak membantu deteksi).
const maxUploadWidth = 960

// AIVisionService — jembatan ke pipeline AI Vision baru tim AI (excavator_vlm, Gracia
// BCS): YOLO+BoT-SORT+LSTM asli yang mendeteksi truk secara visual, dijalankan sebagai
// job upload+async GPU queue di service Python eksternal (pkg/aivision). Beda dari
// loading_cycles kita sendiri yang cuma mengelompokkan bucket_events dari jeda waktu.
//
// Dua jalur trigger:
//   - TriggerAsync: otomatis, dipanggil CCTVService.RecordLoadingCycle tiap 1 siklus
//     loading selesai -- ambil klip dari RecordingsDir (VOD Sync, kamera blackvue://)
//     atau langsung dari TestVideosDir (kamera video:// simulasi), fire-and-forget.
//   - TriggerManual: dipanggil handler POST /ai-vision/manual, video sudah disiapkan
//     user (upload atau file test yang sudah ada).
//
// Pola sama VODSyncService/RecordLoadingCycle: goroutine terpisah, tidak blocking
// caller, gagal-diam-diam-tapi-tercatat (log + baris status=failed), bukan panik.
type AIVisionService struct {
	DB             *gorm.DB
	Client         *aivision.Client
	RecordingsDir  string
	TestVideosDir  string
	Enabled        bool
	MinDurationSec int
}

func NewAIVisionService(db *gorm.DB, client *aivision.Client, recordingsDir, testVideosDir string, enabled bool, minDurationSec int) *AIVisionService {
	return &AIVisionService{
		DB:             db,
		Client:         client,
		RecordingsDir:  recordingsDir,
		TestVideosDir:  testVideosDir,
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
// [start-pad, end+pad] untuk 1 kamera, lalu menggabungkan + membatasi resolusinya lewat
// prepareUploadClip. Caller wajib hapus file hasil setelah dipakai (bukan segmen asli --
// itu tetap milik VODSyncService).
func ExtractClip(recordingsDir, cameraCode string, start, end time.Time) (string, error) {
	// Padding asimetris (04 Agu 2026) -- awalnya simetris 75 detik, tapi cycle_detector.py
	// menutup siklus dgn end_ts = waktu GALI TERAKHIR terdeteksi (bukan waktu siklus
	// benar-benar ditutup, yang baru terjadi idle_timeout_sec/120s kemudian). Jadi jeda
	// nyata setelah bucket terakhir cuma dapat padTail detik -- kalau truk butuh lebih
	// lama dari itu utk pergi dari zona, klip kepotong & AI Vision balikin load_condition
	// "pending" (video_end) padahal truknya sebenarnya beres, cuma tidak tertangkap
	// kameranya sampai selesai. padHead tetap kecil (start_ts sudah akurat, awal gali).
	const padHead = 30 * time.Second
	const padTail = 180 * time.Second
	windowStart := start.In(jakarta).Add(-padHead)
	windowEnd := end.In(jakarta).Add(padTail)

	segs, err := listSegments(recordingsDir, cameraCode)
	if err != nil {
		return "", err
	}
	var paths []string
	for _, s := range segs {
		if !s.TS.Before(windowStart) && !s.TS.After(windowEnd) {
			paths = append(paths, s.Path)
		}
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("tidak ada segmen rekaman kamera %s pada window %s..%s", cameraCode, windowStart, windowEnd)
	}

	outPath, err := newClipOutputPath()
	if err != nil {
		return "", err
	}
	if err := prepareUploadClip(paths, outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

// prepareTestVideoClip -- untuk kamera video:// (simulasi/testing, mis. CAM-EXC-02
// dipakai sebagai pengganti dashcam sementara). Tidak ada konsep "rekaman per-window-
// waktu" seperti VOD dashcam asli -- satu-satunya sumber adalah file video test itu
// sendiri, diputar ulang terus oleh ai-service. Dikirim apa adanya lewat
// prepareUploadClip yang sama (batas resolusi tetap konsisten), bukan potongan sesuai
// window siklus seperti ExtractClip.
func prepareTestVideoClip(testVideosDir, streamURL string) (string, error) {
	rel := strings.TrimPrefix(streamURL, "video://")
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("path video kosong")
	}
	base := filepath.Clean(testVideosDir)
	full := filepath.Clean(filepath.Join(base, rel))
	if full != base && !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", fmt.Errorf("path video di luar direktori yang diizinkan")
	}
	if _, err := os.Stat(full); err != nil {
		return "", fmt.Errorf("file video test tidak ditemukan: %s", rel)
	}
	outPath, err := newClipOutputPath()
	if err != nil {
		return "", err
	}
	if err := prepareUploadClip([]string{full}, outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

// newClipOutputPath -- path temp baru untuk 1 klip hasil olahan (dipakai jalur otomatis
// & manual, keduanya lewat prepareUploadClip).
func newClipOutputPath() (string, error) {
	outDir := filepath.Join(os.TempDir(), "aivision-clips")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("gagal buat temp dir klip: %w", err)
	}
	return filepath.Join(outDir, uuid.NewString()+".mp4"), nil
}

// prepareUploadClip menggabungkan 1+ video (ffmpeg concat demuxer, jalan juga untuk 1
// file) SEKALIGUS membatasi lebar maksimum ke maxUploadWidth dalam 1 pass -- dipakai
// jalur otomatis (klip VOD) maupun manual (upload/pilihan user), supaya kontrol resolusi
// cuma di 1 tempat, bukan logic dobel. Selalu re-encode (bukan `-c copy`) karena filter
// scale butuh itu -- video yang sudah kecil pun ikut lewat sini (murah dibanding proses
// GPU di baliknya, ~detik bukan menit), demi 1 code path yang sederhana.
func prepareUploadClip(paths []string, outPath string) error {
	listPath := outPath + ".concat.txt"
	var lines string
	for _, p := range paths {
		lines += fmt.Sprintf("file '%s'\n", filepath.ToSlash(p))
	}
	if err := os.WriteFile(listPath, []byte(lines), 0644); err != nil {
		return fmt.Errorf("gagal tulis daftar concat: %w", err)
	}
	defer os.Remove(listPath)

	cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", listPath,
		"-vf", fmt.Sprintf("scale='min(%d,iw)':-2", maxUploadWidth),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "24", "-an",
		outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg gagal siapkan klip: %w (%s)", err, string(out))
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

	if strings.HasPrefix(cam.StreamURL, "video://") {
		// Kamera simulasi (video test diputar ulang terus) menghasilkan loading_cycles
		// baru berulang-ulang dari konten yang SAMA -- tanpa throttle ini, tiap
		// perulangan akan resubmit video test yang identik ke antrean GPU. Lewati kalau
		// masih ada job jalan, atau sudah ada yang selesai dalam 30 menit terakhir.
		var recentCount int64
		s.DB.Model(&domain.AIVisionAnalysis{}).
			Where("unit_id = ? AND trigger_source = 'auto' AND (status IN ('queued','running') OR (status = 'completed' AND submitted_at > ?))",
				unitID, time.Now().Add(-30*time.Minute)).
			Count(&recentCount)
		if recentCount > 0 {
			return
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
		var clipPath string
		var err error
		switch {
		case strings.HasPrefix(cam.StreamURL, "blackvue://"):
			clipPath, err = ExtractClip(s.RecordingsDir, cam.Code, cycle.StartTS, cycle.EndTS)
		case strings.HasPrefix(cam.StreamURL, "video://"):
			clipPath, err = prepareTestVideoClip(s.TestVideosDir, cam.StreamURL)
		default:
			err = fmt.Errorf("skema stream_url kamera tidak didukung untuk AI Vision: %s", cam.StreamURL)
		}
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
// videoPath ASLI tidak pernah dihapus/diubah di sini (beda dari klip otomatis yang
// memang temp file). Tetap lewat prepareUploadClip supaya batas resolusi (maxUploadWidth)
// konsisten dengan jalur otomatis -- hasilnya file temp BARU yang dihapus setelah upload,
// bukan videoPath aslinya. TIDAK difilter MinDurationSec (video manual sengaja dipilih
// user).
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
		clipPath, err := newClipOutputPath()
		if err != nil {
			s.markFailed(row.ID, err.Error())
			return
		}
		if err := prepareUploadClip([]string{videoPath}, clipPath); err != nil {
			s.markFailed(row.ID, err.Error())
			return
		}
		defer os.Remove(clipPath)
		s.submitAndPoll(row.ID, clipPath, unitID, time.Now().Format(time.RFC3339))
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
