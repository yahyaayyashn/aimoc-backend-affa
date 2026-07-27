package middleware

import (
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

func WebhookSecret(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if secret == "" {
			return c.Next()
		}
		got := c.Get("X-Webhook-Secret")
		if got != secret {
			return utils.Unauthorized(c, "Webhook secret tidak valid")
		}
		return c.Next()
	}
}
