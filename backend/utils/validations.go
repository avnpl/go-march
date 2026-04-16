package utils

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

func FormatValidationErrors(err error) string {
	var message string

	for _, e := range err.(validator.ValidationErrors) {
		switch e.Tag() {
		case "required":
			message = fmt.Sprintf("%s is required", e.StructField())
		default:
			message = "Invalid Request"
		}
	}
	return message
}
