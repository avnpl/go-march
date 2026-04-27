package utils

import (
	"net/http"

	"github.com/rs/xid"
)

const requestIDHeaderKey = "X-Request-ID"

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		requestId := r.Header.Get(requestIDHeaderKey)
		if requestId == "" {
			requestId = xid.New().String()
		}

		ctx = SetRequestID(ctx, requestId)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
