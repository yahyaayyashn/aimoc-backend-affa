package handler

import (
	"aimoc-backend/internal/domain"
	"aimoc-backend/internal/middleware"
	"aimoc-backend/internal/service"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CameraIncidentHandler — "03 Pemeriksaan Unit" (Dokumen_Penjelasan_Dashboard_AIMOC.pdf):
// pemantauan & tindak lanjut gangguan koneksi dashcam. DB dipegang di sini (bukan cuma
// Svc) khusus buat isolasi demo/produksi (cameraInScope) -- CameraIncidentService
// sendiri sengaja tidak tahu soal is_test/viewer, itu concern HTTP layer.
type CameraIncidentHandler struct {
	DB  *gorm.DB
	Svc *service.CameraIncidentService
}

func NewCameraIncidentHandler(db *gorm.DB, s *service.CameraIncidentService) *CameraIncidentHandler {
	return &CameraIncidentHandler{DB: db, Svc: s}
}

// ActiveIncident — gangguan yang sedang berlangsung untuk 1 kamera (dibuat otomatis
// bila belum ada, lihat CameraIncidentService.GetOrCreateActive). data: null kalau
// kamera sedang online.
func (h *CameraIncidentHandler) ActiveIncident(c *fiber.Ctx) error {
	camID, err := uuid.Parse(c.Params("id"))
	if err != nil || !cameraInScope(h.DB, c, camID) {
		return utils.BadRequest(c, "ID kamera tidak valid", nil)
	}
	inc, err := h.Svc.GetOrCreateActive(camID)
	if err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	return utils.OK(c, "OK", inc)
}

// IncidentHistory — riwayat gangguan 1 kamera, terbaru dulu.
func (h *CameraIncidentHandler) IncidentHistory(c *fiber.Ctx) error {
	camID, err := uuid.Parse(c.Params("id"))
	if err != nil || !cameraInScope(h.DB, c, camID) {
		return utils.BadRequest(c, "ID kamera tidak valid", nil)
	}
	rows, err := h.Svc.ListIncidents(camID)
	if err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	return utils.OK(c, "OK", rows)
}

// ListActiveIncidents — semua gangguan terbuka lintas kamera, dipakai widget "Unit
// Memerlukan Tindakan" di dashboard. Difilter is_test (05 Agu 2026) -- ListActive di
// service sengaja tidak tahu soal viewer, jadi disaring di sini per baris.
func (h *CameraIncidentHandler) ListActiveIncidents(c *fiber.Ctx) error {
	rows, err := h.Svc.ListActive()
	if err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	scoped := make([]domain.CameraIncident, 0, len(rows))
	for _, r := range rows {
		if cameraInScope(h.DB, c, r.CameraID) {
			scoped = append(scoped, r)
		}
	}
	return utils.OK(c, "OK", scoped)
}

type updateIncidentReq struct {
	Status string `json:"status" validate:"required,oneof=BELUM_DIPERIKSA SEDANG_DITANGANI SUDAH_PULIH"`
	Notes  string `json:"notes"`
}

// UpdateIncident — operator menyimpan catatan pemeriksaan + status penanganan.
func (h *CameraIncidentHandler) UpdateIncident(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "ID gangguan tidak valid", nil)
	}
	var req updateIncidentReq
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.BadRequest(c, "Validasi gagal", errs)
	}
	uid, _ := c.Locals(middleware.CtxUserID).(uuid.UUID)
	inc, err := h.Svc.UpdateIncident(id, req.Status, req.Notes, &uid)
	if err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	return utils.OK(c, "Hasil pemeriksaan tersimpan", inc)
}
