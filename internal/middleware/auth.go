package middleware

import (
	"net/http"
	"strings"
)

// ServiceKey validates the X-Service-Key header.
func ServiceKey(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				http.Error(w, `{"error":{"code":"not_configured","message":"service key not configured"}}`, http.StatusInternalServerError)
				return
			}
			if strings.TrimSpace(r.Header.Get("X-Service-Key")) != apiKey {
				http.Error(w, `{"error":{"code":"unauthorized","message":"invalid service key"}}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
