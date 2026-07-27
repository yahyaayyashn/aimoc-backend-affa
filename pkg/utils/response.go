package utils

import "github.com/gofiber/fiber/v2"

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type Meta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func OK(c *fiber.Ctx, msg string, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Response{Success: true, Message: msg, Data: data})
}

func OKMeta(c *fiber.Ctx, msg string, data interface{}, meta interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Response{Success: true, Message: msg, Data: data, Meta: meta})
}

func Created(c *fiber.Ctx, msg string, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Response{Success: true, Message: msg, Data: data})
}

func BadRequest(c *fiber.Ctx, msg string, err interface{}) error {
	return c.Status(fiber.StatusBadRequest).JSON(Response{Success: false, Message: msg, Error: err})
}

func Unauthorized(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(Response{Success: false, Message: msg})
}

func Forbidden(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusForbidden).JSON(Response{Success: false, Message: msg})
}

func NotFound(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusNotFound).JSON(Response{Success: false, Message: msg})
}

func ServerError(c *fiber.Ctx, msg string, err interface{}) error {
	return c.Status(fiber.StatusInternalServerError).JSON(Response{Success: false, Message: msg, Error: err})
}
