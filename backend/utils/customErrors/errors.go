package customErrors

import (
	"errors"
	"net/http"
)

type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type errorHttpMapping struct {
	err    error
	status int
}

var (
	InvalidRequest    = errors.New("Invalid Request")
	RecordNotFound    = errors.New("Record Not Found")
	OutOfStock        = errors.New("Out of Stock")
	IncorrectAmount   = errors.New("Incorrect Amount")
	Conflict          = errors.New("Conflict Error")
	FailedTransaction = errors.New("Transaction Failed")
)

var errRegistry = []errorHttpMapping{
	{OutOfStock, http.StatusBadRequest},
	{InvalidRequest, http.StatusBadRequest},
	{RecordNotFound, http.StatusBadRequest},
	{IncorrectAmount, http.StatusBadRequest},
	{FailedTransaction, http.StatusBadRequest},
}

func HTTPFor(err error) (status int, ok bool) {
	for _, row := range errRegistry {
		if errors.Is(err, row.err) {
			return row.status, true
		}
	}
	return 0, false
}
