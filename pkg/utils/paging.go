package utils

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Pagination struct {
	Page    int
	PerPage int
	Offset  int
}

func GetPagination(c *fiber.Ctx) Pagination {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	per, _ := strconv.Atoi(c.Query("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if per < 1 || per > 200 {
		per = 20
	}
	return Pagination{Page: page, PerPage: per, Offset: (page - 1) * per}
}

func MakeMeta(p Pagination, total int64) Meta {
	totalPages := int((total + int64(p.PerPage) - 1) / int64(p.PerPage))
	return Meta{Page: p.Page, PerPage: p.PerPage, Total: total, TotalPages: totalPages}
}
