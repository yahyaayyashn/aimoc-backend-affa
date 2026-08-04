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
