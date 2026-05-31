package web

import (
	"fmt"
	"net/http"
)

// HealthHandler returns a simple liveness handler.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "OK")
	}
}
