package handler

import (
	"aimoc-backend/internal/service"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

// CCTVHandler (versi AI-only) — hanya endpoint yang dipakai AI service:
// daftar kamera untuk dipoll, dan penerimaan siklus loading terdeteksi AI.
type CCTVHandler struct {
	Svc *service.CCTVService
}

func NewCCTVHandler(s *service.CCTVService) *CCTVHandler {
	return &CCTVHandler{Svc: s}
}

// Cameras — dipakai AI CCTV service (camera_registry.py) polling berkala: daftar
// kamera yang bisa dibaca AI, supaya kamera baru yang ditambah lewat Master Kamera
// otomatis kepakai tanpa perlu edit .env CAMERAS + restart service manual.
func (h *CCTVHandler) Cameras(c *fiber.Ctx) error {
	rows, err := h.Svc.Cameras()
	if err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	return utils.OK(c, "OK", rows)
}

// LoadingCycleDetected menerima 1 siklus loading yang dideteksi AI service SECARA
// MANDIRI (cycle_detector.py). Inilah jalur utama pelaporan aktivitas AI di versi
// AI-only — disimpan ke tabel loading_cycles untuk dashboard & laporan.
func (h *CCTVHandler) LoadingCycleDetected(c *fiber.Ctx) error {
	var req service.LoadingCycleDetectedReq
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.BadRequest(c, "Validasi gagal", errs)
	}
	lc, err := h.Svc.RecordLoadingCycle(req)
	if err != nil {
		return utils.BadRequest(c, err.Error(), lc)
	}
	return utils.OK(c, "Siklus loading tercatat", lc)
}

// BucketDetected menerima 1 event bucket individual dengan timestamp presisi, dikirim
// AI service tiap kali bucket terdeteksi — dipakai backend mengelompokkan bucket jadi
// truk (lihat internal/service/truck_grouping.go), endpoint TERPISAH dari
// LoadingCycleDetected di atas.
func (h *CCTVHandler) BucketDetected(c *fiber.Ctx) error {
	var req service.BucketDetectedReq
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.BadRequest(c, "Validasi gagal", errs)
	}
	be, err := h.Svc.RecordBucketEvent(req)
	if err != nil {
		return utils.BadRequest(c, err.Error(), be)
	}
	return utils.OK(c, "Event bucket tercatat", be)
}

// CameraHeartbeat menerima laporan status TERKINI (digging/non_digging, live/recording)
// dari AI service, dikirim berkala tiap kamera. Dasar badge status real-time
// (Aktif/Idle/Offline) di dashboard — lihat CCTVService.RecordHeartbeat.
func (h *CCTVHandler) CameraHeartbeat(c *fiber.Ctx) error {
	var req service.CameraHeartbeatReq
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.BadRequest(c, "Validasi gagal", errs)
	}
	if err := h.Svc.RecordHeartbeat(req); err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	return utils.OK(c, "Heartbeat tercatat", nil)
}
