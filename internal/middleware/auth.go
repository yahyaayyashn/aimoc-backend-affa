package middleware

import (
	"strings"

	jwtpkg "aimoc-backend/pkg/jwt"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

const (
	CtxUserID   = "user_id"
	CtxRoleCode = "role_code"
	CtxEmail    = "email"
)

func AuthRequired(svc *jwtpkg.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := ""
		header := c.Get("Authorization")
		if header != "" {
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return utils.Unauthorized(c, "Format token tidak valid")
			}
			token = parts[1]
		} else if q := c.Query("token"); q != "" {
			// Fallback query-param, khusus untuk elemen <img>/<video> (mis. live stream kamera)
			// yang tidak bisa menyertakan header Authorization.
			token = q
		} else {
			return utils.Unauthorized(c, "Token tidak ditemukan")
		}
		claims, err := svc.ParseAccess(token)
		if err != nil {
			return utils.Unauthorized(c, "Token kadaluarsa atau tidak valid")
		}
		c.Locals(CtxUserID, claims.UserID)
		c.Locals(CtxRoleCode, claims.RoleCode)
		c.Locals(CtxEmail, claims.Email)
		return c.Next()
	}
}

func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals(CtxRoleCode).(string)
		if !ok {
			return utils.Forbidden(c, "Akses ditolak")
		}
		for _, r := range roles {
			if r == role || role == "SUPER_ADMIN" {
				return c.Next()
			}
		}
		return utils.Forbidden(c, "Role tidak memiliki akses ke endpoint ini")
	}
}
