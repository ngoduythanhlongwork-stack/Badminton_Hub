package httpapi

import (
	"context"
	"net/http"
	"time"
)

type Ping func(context.Context) error

// Readiness keeps cache loss non-fatal; PostgreSQL is authoritative.
type Readiness struct {
	Postgres Ping
	Redis    Ping
	Timeout  time.Duration
}

func (d Readiness) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	checks := map[string]Ping{"postgres": d.Postgres, "redis": d.Redis}
	states := map[string]string{"postgres": "down", "redis": "down"}
	pending := 0
	for name, ping := range checks {
		if ping == nil {
			continue
		}
		pending++
		go func(name string, ping Ping) { results <- result{name, ping(ctx)} }(name, ping)
	}
	for pending > 0 {
		select {
		case result := <-results:
			if result.err == nil {
				states[result.name] = "up"
			}
			pending--
		case <-ctx.Done():
			pending = 0
		}
	}
	code, status := http.StatusOK, "ok"
	if states["redis"] != "up" {
		status = "degraded"
	}
	if states["postgres"] != "up" {
		code, status = http.StatusServiceUnavailable, "unavailable"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	writeJSON(w, struct {
		Status       string            `json:"status"`
		Dependencies map[string]string `json:"dependencies"`
	}{status, states})
}
