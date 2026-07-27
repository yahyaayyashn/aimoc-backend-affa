package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UploadHandler struct {
	Dir string
}

func NewUploadHandler(dir string) *UploadHandler { return &UploadHandler{Dir: dir} }

var allowedImageExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
}

// sanitizeFolder hanya mengizinkan huruf kecil, angka, dash, underscore (mencegah path traversal).
func sanitizeFolder(f string) string {
	f = strings.ToLower(strings.TrimSpace(f))
	out := make([]rune, 0, len(f))
	for _, r := range f {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			out = append(out, r)
		}
	}
	s := string(out)
	if s == "" {
		s = "misc"
	}
	return s
}

// UploadImage menerima multipart field "file" dan menyimpannya ke {UploadDir}/images/{folder}/{uuid}.{ext}.
// Mengembalikan path relatif (mis. /uploads/images/customers/xxx.png) yang disimpan di DB
// dan diakses baik oleh Web (proxy) maupun Mobile (origin + path).
func (h *UploadHandler) UploadImage(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return utils.BadRequest(c, "File tidak ditemukan (gunakan field 'file')", nil)
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedImageExt[ext] {
		return utils.BadRequest(c, "Format gambar tidak didukung. Gunakan jpg, png, webp, atau gif", nil)
	}
	if fileHeader.Size > 5*1024*1024 {
		return utils.BadRequest(c, "Ukuran gambar maksimal 5MB", nil)
	}

	folder := sanitizeFolder(c.FormValue("folder", "misc"))
	destDir := filepath.Join(h.Dir, "images", folder)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return utils.ServerError(c, "Gagal membuat folder penyimpanan", nil)
	}

	name := uuid.NewString() + ext
	destPath := filepath.Join(destDir, name)
	if err := c.SaveFile(fileHeader, destPath); err != nil {
		return utils.ServerError(c, "Gagal menyimpan gambar", nil)
	}

	rel := fmt.Sprintf("/uploads/images/%s/%s", folder, name)
	return utils.OK(c, "Gambar berhasil diupload", fiber.Map{
		"path": rel,
		"url":  rel,
	})
}
