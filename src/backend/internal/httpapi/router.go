package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Options struct {
	Readiness   Readiness
	Logger      *slog.Logger
	MaxBodySize int64
}

// NewHandler is the composition point for module HTTP routes.
// Health is liveness only: no database/cache readiness claim is made.
func NewHandler(dependencies ...Readiness) http.Handler {
	readiness := Readiness{}
	if len(dependencies) > 0 {
		readiness = dependencies[0]
	}
	return NewHandlerWithOptions(Options{Readiness: readiness})
}

func NewHandlerWithOptions(options Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", method(http.MethodGet, options.Readiness.ServeHTTP))
	mux.HandleFunc("/{$}", method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{
			"service":      "Badminton Hub API",
			"architecture": "modular-monolith",
			"status":       "foundation",
		})
	}))
	mux.HandleFunc("/health", method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusNotFound, "route_not_found", "Route not found.")
	})
	return middleware(options.Logger, options.MaxBodySize, mux)
}

func method(allowed string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != allowed {
			w.Header().Set("Allow", allowed)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
			return
		}
		handler(w, r)
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}
