package handler

import (
	"aimoc-backend/internal/domain"
	"aimoc-backend/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// isTestViewer — cek flag is_test_viewer user yang sedang login (lihat migration
// 000031). Dipakai scoping semua endpoint yang expose data excavator, supaya
// dashboard demo/testing (is_test_viewer=true, cuma lihat excavator is_test=true)
// dan dashboard produksi (is_test_viewer=false, cuma lihat is_test=false) tidak
// pernah tercampur, siapa pun yang login. Fungsi package-level (bukan method) karena
// dipakai lintas handler struct berbeda (MasterHandler/MiscHandler/AIVisionHandler)
// yang masing-masing punya field DB sendiri.
func isTestViewer(db *gorm.DB, c *fiber.Ctx) bool {
	uid, ok := c.Locals(middleware.CtxUserID).(uuid.UUID)
	if !ok {
		return false
	}
	var u domain.User
	if err := db.Select("is_test_viewer").First(&u, "id = ?", uid).Error; err != nil {
		return false
	}
	return u.IsTestViewer
}

// cameraInScope — isolasi demo/produksi (lihat isTestViewer) untuk endpoint yang akses
// KAMERA per-ID (live view, recordings, incident, dst -- camera_stream.go &
// camera_incident.go). Kamera sendiri tidak punya kolom is_test -- ikut excavator yang
// memakainya lewat relasi excavators.camera_id -> cameras.id (satu kamera cuma boleh
// dedicated ke satu excavator, lihat checkCameraNotTaken di master.go). Kamera yang
// belum terhubung excavator manapun dianggap scope PRODUKSI (default aman, BUKAN
// otomatis lolos ke semua viewer).
func cameraInScope(db *gorm.DB, c *fiber.Ctx, cameraID uuid.UUID) bool {
	var exc domain.Excavator
	err := db.Select("is_test").Where("camera_id = ?", cameraID).First(&exc).Error
	isTest := err == nil && exc.IsTest
	return isTest == isTestViewer(db, c)
}
