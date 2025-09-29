package utils

import (
	"github.com/pkg/errors"
)

var (
	ErrConflict       = errors.New("ErrConflict")
	ErrInternal       = errors.New("Internal Error")
	ErrInvalidRequest = errors.New("Invalid Request")
	ErrRecordNotFound = errors.New("Record Not Found")
)

type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
