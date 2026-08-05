package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aimoc-backend/internal/domain"
	"aimoc-backend/internal/service"
	"aimoc-backend/pkg/aivision"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// AIVisionHandler — endpoint untuk hasil analisa pipeline AI Vision baru tim AI
// (excavator_vlm, lihat internal/service/ai_vision.go). Browser TIDAK PERNAH panggil
// service Python langsung -- semua lewat sini (API key tetap di backend).
type AIVisionHandler struct {
	DB            *gorm.DB
	Svc           *service.AIVisionService
	Client        *aivision.Client
	TestVideosDir string
	UploadDir     string
}

func NewAIVisionHandler(db *gorm.DB, svc *service.AIVisionService, client *aivision.Client, testVideosDir, uploadDir string) *AIVisionHandler {
	return &AIVisionHandler{DB: db, Svc: svc, Client: client, TestVideosDir: testVideosDir, UploadDir: uploadDir}
}

// aiVisionRowInScope — isolasi dashboard demo/testing (lihat isTestViewer) untuk
// endpoint AI Vision yang akses per-baris (Get/Video/GetByLoadingCycle). unit_id di
// ai_vision_analyses SELALU kode excavator (lihat AIVisionService.TriggerAsync &
// AnalisaVideoAI.tsx yang selalu kirim Excavator.Code, bukan Camera.Code) -- jadi
// tinggal join balik ke excavators.code buat tahu is_test-nya. Excavator yang sudah
// dihapus (unit_id tidak match manapun) dianggap TIDAK dalam scope siapapun, lebih
// aman ditolak daripada bocor.
func aiVisionRowInScope(db *gorm.DB, c *fiber.Ctx, unitID string) bool {
	var exc domain.Excavator
	if err := db.Select("is_test").First(&exc, "code = ?", unitID).Error; err != nil {
		return false
	}
	return exc.IsTest == isTestViewer(db, c)
}

// GetByLoadingCycle — GET /loading-cycles/:id/ai-vision. 404 kalau belum pernah dipicu
// (siklus terlalu pendek, AI_VISION_ENABLED=false, atau memang belum diproses) --
// bukan error, FE tampilkan "tidak ada analisa lanjutan".
func (h *AIVisionHandler) GetByLoadingCycle(c *fiber.Ctx) error {
	var row domain.AIVisionAnalysis
	err := h.DB.Where("loading_cycle_id = ?", c.Params("id")).
		Order("submitted_at DESC").First(&row).Error
	if err != nil || !aiVisionRowInScope(h.DB, c, row.UnitID) {
		return utils.NotFound(c, "Belum ada analisa AI Vision untuk siklus ini")
	}
	return utils.OK(c, "OK", row)
}

// Get — GET /ai-vision/:id, dipakai jalur manual (tidak selalu ada loading_cycle_id).
func (h *AIVisionHandler) Get(c *fiber.Ctx) error {
	var row domain.AIVisionAnalysis
	if err := h.DB.First(&row, "id = ?", c.Params("id")).Error; err != nil || !aiVisionRowInScope(h.DB, c, row.UnitID) {
		return utils.NotFound(c, "Analisa tidak ditemukan")
	}
	return utils.OK(c, "OK", row)
}

// List — GET /ai-vision, riwayat analisa (auto + manual), dipakai halaman Analisa Video
// AI dan bisa juga untuk audit umum. Difilter is_test (05 Agu 2026 -- sebelumnya BOCOR,
// endpoint ini kelewat waktu isolasi demo/produksi dibuat, admin sempat lihat data test
// EXC-PC200-1 di halaman Analisa Video AI).
func (h *AIVisionHandler) List(c *fiber.Ctx) error {
	p := utils.GetPagination(c)
	// ai_vision_analyses & excavators sama-sama punya kolom id/status -- Select eksplisit
	// wajib di query Find (bukan cuma Count) supaya tidak SELECT * ambigu ketupel-JOIN.
	scope := h.DB.Table("ai_vision_analyses").
		Joins("JOIN excavators ON excavators.code = ai_vision_analyses.unit_id").
		Where("excavators.is_test = ?", isTestViewer(h.DB, c))
	var total int64
	scope.Session(&gorm.Session{}).Count(&total)
	var rows []domain.AIVisionAnalysis
	scope.Session(&gorm.Session{}).Select("ai_vision_analyses.*").
		Order("submitted_at DESC").Limit(p.PerPage).Offset(p.Offset).Find(&rows)
	return utils.OKMeta(c, "OK", rows, utils.MakeMeta(p, total))
}

// ExcavatorSummary — GET /excavators/:id/ai-vision-summary, agregat durasi Mining vs
// Loading dari analisa AI Vision yang statusnya completed untuk excavator ini
// (dicocokkan lewat unit_id = Excavator.Code). Dipakai menggantikan placeholder "belum
// tersedia" di DetailExcavator & card Klasifikasi Aktivitas di Dashboard Produktivitas
// -- catatan penting: cakupannya cuma siklus yang KEBETULAN sudah dianalisa AI Vision
// (otomatis per siklus >=20 detik, atau manual), BUKAN semua aktivitas excavator ini --
// FE wajib tampilkan ini sebagai cakupan parsial, bukan total aktivitas.
func (h *AIVisionHandler) ExcavatorSummary(c *fiber.Ctx) error {
	var exc domain.Excavator
	if err := h.DB.First(&exc, "id = ?", c.Params("id")).Error; err != nil {
		return utils.NotFound(c, "Excavator tidak ditemukan")
	}
	// Isolasi dashboard demo/testing (lihat isTestViewer) -- excavator di luar scope
	// user yang login dibalas 404 sama seperti tidak ada, jangan bocorkan bahwa ID-nya
	// sebenarnya valid.
	if exc.IsTest != isTestViewer(h.DB, c) {
		return utils.NotFound(c, "Excavator tidak ditemukan")
	}

	// trigger_source discoping SAMA PERSIS dengan getAIVisionKPI (dashboard_ai.go,
	// 05 Agu 2026) -- sebelumnya endpoint ini tidak filter trigger_source SAMA SEKALI,
	// jadi excavator produksi (is_test=false) yang bahkan TIDAK punya kamera bisa
	// nunjukkin data dari trigger manual test di halaman Analisa Video AI (dibuktikan
	// nyata: excavator baru tanpa kamera menampilkan "1 cycle analyzed").
	q := h.DB.Where("unit_id = ? AND status = 'completed'", exc.Code)
	if exc.IsTest {
		q = q.Where("trigger_source IN ('auto', 'manual')")
	} else {
		q = q.Where("trigger_source = 'auto'")
	}
	var rows []domain.AIVisionAnalysis
	q.Find(&rows)

	// AI Vision sebenarnya kirim 4 kategori (mining/loading/idle/unknown) di
	// activity.durations_seconds -- dulu cuma mining+loading yang diambil, idle/unknown
	// diam-diam dibuang. Sekarang keempatnya di-sum supaya FE bisa hitung persentase
	// dari total durasi video yang BENAR (bukan cuma dari mining+loading sebagai 100%).
	var miningSec, loadingSec, idleSec, unknownSec float64
	analyzed := 0
	for _, r := range rows {
		if r.DashboardSummary == nil {
			continue
		}
		var parsed struct {
			Activity struct {
				DurationsSeconds map[string]float64 `json:"durations_seconds"`
			} `json:"activity"`
		}
		if err := json.Unmarshal([]byte(*r.DashboardSummary), &parsed); err != nil {
			continue
		}
		miningSec += parsed.Activity.DurationsSeconds["mining"]
		loadingSec += parsed.Activity.DurationsSeconds["loading"]
		idleSec += parsed.Activity.DurationsSeconds["idle"]
		unknownSec += parsed.Activity.DurationsSeconds["unknown"]
		analyzed++
	}

	return utils.OK(c, "OK", fiber.Map{
		"analyzed_count":  analyzed,
		"mining_seconds":  miningSec,
		"loading_seconds": loadingSec,
		"idle_seconds":    idleSec,
		"unknown_seconds": unknownSec,
	})
}

// Video — GET /ai-vision/:id/video, proxy stream artifact video ber-anotasi dari
// service Python (pola sama CameraStreamHandler.Live's MJPEG proxy, cuma sumbernya file
// mp4 utuh, bukan multipart chunked).
func (h *AIVisionHandler) Video(c *fiber.Ctx) error {
	var row domain.AIVisionAnalysis
	if err := h.DB.First(&row, "id = ?", c.Params("id")).Error; err != nil || !aiVisionRowInScope(h.DB, c, row.UnitID) {
		return utils.NotFound(c, "Analisa tidak ditemukan")
	}
	if row.Status != "completed" || row.ExternalJobID == "" || row.AnnotatedVideoPath == "" {
		return utils.NotFound(c, "Video ber-anotasi belum tersedia")
	}
	resp, err := h.Client.OpenArtifact(row.ExternalJobID, row.AnnotatedVideoPath)
	if err != nil {
		return utils.ServerError(c, err.Error(), nil)
	}
	// TIDAK defer Close() di sini -- SetBodyStream lazy, body baru dibaca fasthttp
	// SETELAH handler ini return. fasthttp menutup resp.Body (io.Closer) sendiri
	// setelah selesai stream/klien putus (pola sama CameraStreamHandler.Live).
	c.Set("Content-Type", "video/mp4")
	c.Context().Response.SetBodyStream(resp.Body, -1)
	return nil
}

// TestVideos — GET /ai-vision/test-videos, daftar file video yang sudah ada di
// TestVideosDir (dipakai halaman manual supaya user bisa pilih video yang telah
// disiapkan, bukan cuma upload baru). Flat listing, tidak rekursif.
func (h *AIVisionHandler) TestVideos(c *fiber.Ctx) error {
	entries, err := os.ReadDir(h.TestVideosDir)
	if err != nil {
		return utils.OK(c, "OK", []string{})
	}
	exts := map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if exts[strings.ToLower(filepath.Ext(e.Name()))] {
			names = append(names, e.Name())
		}
	}
	return utils.OK(c, "OK", names)
}

// Manual — POST /ai-vision/manual (multipart/form-data). Dua sumber video, salah satu
// wajib: `file` (upload baru) ATAU `video_path` (nama file di TestVideosDir, dari daftar
// TestVideos di atas). `unit_id` wajib, `label` opsional.
func (h *AIVisionHandler) Manual(c *fiber.Ctx) error {
	if h.Svc == nil || !h.Svc.Enabled {
		return utils.BadRequest(c, "AI Vision belum dikonfigurasi (AI_VISION_ENABLED=false)", nil)
	}
	unitID := strings.TrimSpace(c.FormValue("unit_id"))
	if unitID == "" {
		return utils.BadRequest(c, "unit_id wajib diisi", nil)
	}
	label := strings.TrimSpace(c.FormValue("label"))

	var videoPath string
	if fileHeader, err := c.FormFile("file"); err == nil {
		if err := os.MkdirAll(filepath.Join(h.UploadDir, "ai-vision-manual"), 0755); err != nil {
			return utils.ServerError(c, "gagal siapkan folder upload", nil)
		}
		dest := filepath.Join(h.UploadDir, "ai-vision-manual", time.Now().Format("20060102150405")+"_"+filepath.Base(fileHeader.Filename))
		if err := c.SaveFile(fileHeader, dest); err != nil {
			return utils.ServerError(c, "gagal simpan video upload", nil)
		}
		videoPath = dest
	} else if vp := strings.TrimSpace(c.FormValue("video_path")); vp != "" {
		resolved, err := resolveVideoFilePath(h.TestVideosDir, "video://"+vp)
		if err != nil {
			return utils.BadRequest(c, err.Error(), nil)
		}
		videoPath = resolved
	} else {
		return utils.BadRequest(c, "sertakan file upload ATAU video_path", nil)
	}

	row, err := h.Svc.TriggerManual(unitID, label, videoPath)
	if err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"success": true, "message": "Analisa dimulai", "data": row})
}
