package handler

import (
	"aimoc-backend/internal/middleware"
	"aimoc-backend/internal/service"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type AuthHandler struct {
	Svc *service.AuthService
}

func NewAuthHandler(s *service.AuthService) *AuthHandler { return &AuthHandler{Svc: s} }

type loginReq struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginReq
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "Body request tidak valid", nil)
	}
	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.BadRequest(c, "Validasi gagal", errs)
	}
	res, err := h.Svc.Login(req.Email, req.Password, c.IP(), string(c.Request().Header.UserAgent()))
	if err != nil {
		return utils.Unauthorized(c, err.Error())
	}
	return utils.OK(c, "Login berhasil", res)
}

// (Register / self-signup customer dibuang pada pivot AI-only — tidak ada lagi
// customer/transaksi. Akun dibuat lewat seed & manajemen user Super Admin.)

type refreshReq struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req refreshReq
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "Body tidak valid", nil)
	}
	res, err := h.Svc.Refresh(req.RefreshToken, c.IP(), string(c.Request().Header.UserAgent()))
	if err != nil {
		return utils.Unauthorized(c, err.Error())
	}
	return utils.OK(c, "Refresh berhasil", res)
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var req refreshReq
	_ = c.BodyParser(&req)
	if req.RefreshToken != "" {
		_ = h.Svc.Logout(req.RefreshToken)
	}
	return utils.OK(c, "Logout berhasil", nil)
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	uid, ok := c.Locals(middleware.CtxUserID).(uuid.UUID)
	if !ok {
		return utils.Unauthorized(c, "Sesi tidak valid")
	}
	u, err := h.Svc.Me(uid)
	if err != nil {
		return utils.NotFound(c, "User tidak ditemukan")
	}
	return utils.OK(c, "OK", u)
}
