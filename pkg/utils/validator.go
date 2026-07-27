package utils

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func ValidateStruct(s interface{}) []FieldError {
	if err := validate.Struct(s); err != nil {
		errs := []FieldError{}
		if ves, ok := err.(validator.ValidationErrors); ok {
			for _, ve := range ves {
				errs = append(errs, FieldError{
					Field:   strings.ToLower(ve.Field()),
					Message: messageForTag(ve),
				})
			}
		}
		return errs
	}
	return nil
}

func messageForTag(ve validator.FieldError) string {
	switch ve.Tag() {
	case "required":
		return "Wajib diisi"
	case "email":
		return "Format email tidak valid"
	case "min":
		return "Minimal " + ve.Param() + " karakter"
	case "max":
		return "Maksimal " + ve.Param() + " karakter"
	case "gt":
		return "Harus lebih dari " + ve.Param()
	case "gte":
		return "Harus lebih dari atau sama dengan " + ve.Param()
	case "oneof":
		return "Harus salah satu dari: " + ve.Param()
	default:
		return "Nilai tidak valid"
	}
}
