package utils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

func FormatValidationErrors(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return "Invalid Request"
	}

	msgs := make([]string, 0, len(ve))
	for _, e := range ve {
		switch e.Tag() {
		case "required":
			msgs = append(msgs, fmt.Sprintf("%s is required", e.StructField()))
		case "gt":
			msgs = append(msgs, fmt.Sprintf("%s must be greater than %s", e.StructField(), e.Param()))
		case "min":
			msgs = append(msgs, fmt.Sprintf("%s must be at least %s", e.StructField(), e.Param()))
		case "numeric":
			msgs = append(msgs, fmt.Sprintf("%s must contain only numeric characters", e.StructField()))
		case "len":
			msgs = append(msgs, fmt.Sprintf("%s must have %s characters", e.StructField(), e.Param()))
		default:
			msgs = append(msgs, fmt.Sprintf("%s is invalid", e.StructField()))
		}
	}
	return strings.Join(msgs, "; ")
}
