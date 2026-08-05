package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"aimoc-backend/internal/domain"
	wshub "aimoc-backend/internal/websocket"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MasterHandler (versi AI-only) — master data yang relevan untuk AI (Kamera &
// Excavator) plus Material (dikembalikan 01 Agu 2026, lihat migration 000027).
// Master transaksi lain (Customer/Vendor/Truck/Driver) tetap dibuang pada
// pivot AI-only (19 Jul 2026).
type MasterHandler struct {
	DB *gorm.DB
}

func NewMasterHandler(db *gorm.DB) *MasterHandler { return &MasterHandler{DB: db} }

// notifyMaster mem-broadcast perubahan data master ke channel admin & dashboard
// supaya seluruh klien ter-update realtime.
func notifyMaster(eventType, action string, data interface{}) {
	if wshub.Default == nil {
		return
	}
	wshub.Default.Broadcast([]string{"admin", "dashboard"}, wshub.Event{
		Type:      eventType,
		Channel:   "admin",
		Data:      map[string]interface{}{"action": action, "data": data},
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ============== CAMERAS ==============

// ListCameras -- difilter is_test (05 Agu 2026, lihat isTestViewer & cameraInScope).
// Kamera tidak punya kolom is_test sendiri, ikut excavator yang memakainya lewat LEFT
// JOIN excavators.camera_id -- kamera yang belum terhubung excavator manapun dianggap
// scope PRODUKSI (COALESCE ke false), bukan otomatis lolos ke semua viewer.
func (h *MasterHandler) ListCameras(c *fiber.Ctx) error {
	var data []domain.Camera
	p := utils.GetPagination(c)
	tx := h.DB.Table("cameras").
		Joins("LEFT JOIN excavators ON excavators.camera_id = cameras.id").
		Where("COALESCE(excavators.is_test, false) = ?", isTestViewer(h.DB, c))
	var total int64
	tx.Session(&gorm.Session{}).Count(&total)
	tx.Session(&gorm.Session{}).Select("cameras.*").
		Limit(p.PerPage).Offset(p.Offset).Order("cameras.code ASC").Find(&data)
	return utils.OKMeta(c, "OK", data, utils.MakeMeta(p, total))
}

func (h *MasterHandler) CreateCamera(c *fiber.Ctx) error {
	var x domain.Camera
	if err := c.BodyParser(&x); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	if x.Status == "" {
		x.Status = "AKTIF"
	}
	if err := h.DB.Create(&x).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	notifyMaster("CAMERA_CHANGED", "create", x)
	return utils.Created(c, "Kamera berhasil dibuat", x)
}

func (h *MasterHandler) UpdateCamera(c *fiber.Ctx) error {
	var x domain.Camera
	if err := h.DB.First(&x, "id = ?", c.Params("id")).Error; err != nil || !cameraInScope(h.DB, c, x.ID) {
		return utils.NotFound(c, "Kamera tidak ditemukan")
	}
	if err := c.BodyParser(&x); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	h.DB.Save(&x)
	notifyMaster("CAMERA_CHANGED", "update", x)
	return utils.OK(c, "Kamera diperbarui", x)
}

func (h *MasterHandler) DeleteCamera(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil || !cameraInScope(h.DB, c, id) {
		return utils.NotFound(c, "Kamera tidak ditemukan")
	}
	h.DB.Delete(&domain.Camera{}, "id = ?", id)
	notifyMaster("CAMERA_CHANGED", "delete", fiber.Map{"id": id})
	return utils.OK(c, "Kamera dihapus", nil)
}

// ============== EXCAVATORS ==============

func (h *MasterHandler) ListExcavators(c *fiber.Ctx) error {
	var data []domain.Excavator
	p := utils.GetPagination(c)
	q := c.Query("q", "")
	// Camera ikut dimuat — dashboard butuh stream_url kamera tiap excavator untuk
	// thumbnail live view di card excavator.
	tx := h.DB.Model(&domain.Excavator{}).Preload("Camera").
		Where("is_test = ?", isTestViewer(h.DB, c))
	if q != "" {
		tx = tx.Where("name ILIKE ? OR code ILIKE ? OR brand ILIKE ?", "%"+q+"%", "%"+q+"%", "%"+q+"%")
	}
	if status := c.Query("status", ""); status != "" {
		tx = tx.Where("status = ?", status)
	}
	var total int64
	tx.Count(&total)
	tx.Limit(p.PerPage).Offset(p.Offset).Order("code ASC").Find(&data)
	return utils.OKMeta(c, "OK", data, utils.MakeMeta(p, total))
}

// checkCameraNotTaken menolak assign camera_id yang sudah dedicated ke excavator LAIN.
// Satu kamera/dashcam fisik cuma boleh dipantau ke satu excavator (lihat migration 000013).
func (h *MasterHandler) checkCameraNotTaken(cameraID *uuid.UUID, excludeID uuid.UUID) error {
	if cameraID == nil {
		return nil
	}
	var other domain.Excavator
	err := h.DB.Where("camera_id = ? AND id <> ?", *cameraID, excludeID).First(&other).Error
	if err == nil {
		return errors.New("Kamera ini sudah terhubung ke excavator '" + other.Code + "' -- satu kamera cuma boleh dedicated ke satu excavator")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

func (h *MasterHandler) CreateExcavator(c *fiber.Ctx) error {
	var x domain.Excavator
	// Normalisasi camera_id:"" -> null, alasan sama seperti UpdateExcavator.
	body := bytes.ReplaceAll(c.Body(), []byte(`"camera_id":""`), []byte(`"camera_id":null`))
	if err := json.Unmarshal(body, &x); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	if x.Code == "" {
		x.Code = "EXC-" + uuid.NewString()[:8]
	}
	if x.Status == "" {
		x.Status = "AKTIF"
	}
	// is_test SELALU ikut scope user yang login, BUKAN dari body client (05 Agu 2026) --
	// supaya excavator baru otomatis nongol di dashboard SI PEMBUAT, tidak diam-diam
	// bocor ke scope lain (lihat isTestViewer).
	x.IsTest = isTestViewer(h.DB, c)
	if err := h.checkCameraNotTaken(x.CameraID, uuid.Nil); err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	if err := h.DB.Create(&x).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	notifyMaster("EXCAVATOR_CHANGED", "create", x)
	return utils.Created(c, "Excavator berhasil dibuat", x)
}

func (h *MasterHandler) UpdateExcavator(c *fiber.Ctx) error {
	var x domain.Excavator
	if err := h.DB.First(&x, "id = ?", c.Params("id")).Error; err != nil || x.IsTest != isTestViewer(h.DB, c) {
		return utils.NotFound(c, "Excavator tidak ditemukan")
	}
	originalIsTest := x.IsTest
	// camera_id:"" tidak bisa di-parse ke *uuid.UUID — normalisasi ke null supaya
	// MENGOSONGKAN kamera tidak gagal "Body tidak valid".
	body := bytes.ReplaceAll(c.Body(), []byte(`"camera_id":""`), []byte(`"camera_id":null`))
	if err := json.Unmarshal(body, &x); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	// is_test bukan field yang bisa diubah lewat form Master Data (tidak ada di UI) --
	// pertahankan nilai asli, jangan biarkan body request menimpanya.
	x.IsTest = originalIsTest
	if err := h.checkCameraNotTaken(x.CameraID, x.ID); err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	// Kosongkan relasi + Omit supaya GORM tidak ikut meng-upsert baris kamera.
	x.Camera = nil
	if err := h.DB.Omit("Camera").Save(&x).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	notifyMaster("EXCAVATOR_CHANGED", "update", x)
	return utils.OK(c, "Excavator diperbarui", x)
}

func (h *MasterHandler) DeleteExcavator(c *fiber.Ctx) error {
	var x domain.Excavator
	if err := h.DB.First(&x, "id = ?", c.Params("id")).Error; err != nil || x.IsTest != isTestViewer(h.DB, c) {
		return utils.NotFound(c, "Excavator tidak ditemukan")
	}
	if err := h.DB.Delete(&domain.Excavator{}, "id = ?", c.Params("id")).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	notifyMaster("EXCAVATOR_CHANGED", "delete", fiber.Map{"id": c.Params("id")})
	return utils.OK(c, "Excavator dihapus", nil)
}

// ============== MATERIALS ==============
// Dikembalikan sebagai master data (permintaan user 01 Agu 2026) -- tabel
// sempat di-drop saat pivot AI-only (migration 000017), sekarang di-recreate
// via migration 000027.

func (h *MasterHandler) ListMaterials(c *fiber.Ctx) error {
	var data []domain.Material
	p := utils.GetPagination(c)
	tx := h.DB.Model(&domain.Material{})
	if q := c.Query("q", ""); q != "" {
		tx = tx.Where("name ILIKE ? OR code ILIKE ?", "%"+q+"%", "%"+q+"%")
	}
	var total int64
	tx.Count(&total)
	tx.Limit(p.PerPage).Offset(p.Offset).Order("code ASC").Find(&data)
	return utils.OKMeta(c, "OK", data, utils.MakeMeta(p, total))
}

func (h *MasterHandler) CreateMaterial(c *fiber.Ctx) error {
	var x domain.Material
	if err := c.BodyParser(&x); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	if x.Code == "" {
		x.Code = "MAT-" + uuid.NewString()[:8]
	}
	if x.Status == "" {
		x.Status = "AKTIF"
	}
	if err := h.DB.Create(&x).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	notifyMaster("MATERIAL_CHANGED", "create", x)
	return utils.Created(c, "Material berhasil dibuat", x)
}

func (h *MasterHandler) UpdateMaterial(c *fiber.Ctx) error {
	var x domain.Material
	if err := h.DB.First(&x, "id = ?", c.Params("id")).Error; err != nil {
		return utils.NotFound(c, "Material tidak ditemukan")
	}
	if err := c.BodyParser(&x); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	h.DB.Save(&x)
	notifyMaster("MATERIAL_CHANGED", "update", x)
	return utils.OK(c, "Material diperbarui", x)
}

func (h *MasterHandler) DeleteMaterial(c *fiber.Ctx) error {
	if err := h.DB.Delete(&domain.Material{}, "id = ?", c.Params("id")).Error; err != nil {
		return utils.BadRequest(c, err.Error(), nil)
	}
	notifyMaster("MATERIAL_CHANGED", "delete", fiber.Map{"id": c.Params("id")})
	return utils.OK(c, "Material dihapus", nil)
}
