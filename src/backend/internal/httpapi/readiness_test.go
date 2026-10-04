package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadiness(t *testing.T) {
	up := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("secret connection string") }
	for _, tc := range []struct {
		name      string
		pg, redis Ping
		code      int
		status    string
	}{
		{"healthy", up, up, 200, "ok"},
		{"cache failure", up, down, 200, "degraded"},
		{"database failure", down, up, 503, "unavailable"},
		{"both failures", down, down, 503, "unavailable"},
		{"unconfigured", nil, nil, 503, "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewHandler(Readiness{Postgres: tc.pg, Redis: tc.redis, Timeout: time.Second}).ServeHTTP(w, httptest.NewRequest("GET", "/ready", nil))
			var body struct{ Status string }
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.code || body.Status != tc.status {
				t.Fatalf("unexpected readiness: %d %s", w.Code, w.Body)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("readiness must not be cached")
			}
		})
	}
}

func TestReadinessDeadline(t *testing.T) {
	slow := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	w := httptest.NewRecorder()
	start := time.Now()
	Readiness{Postgres: slow, Redis: slow, Timeout: 20 * time.Millisecond}.ServeHTTP(w, httptest.NewRequest("GET", "/ready", nil))
	if w.Code != 503 || time.Since(start) > time.Second {
		t.Fatal("deadline not enforced")
	}
}

func TestLivenessIndependentOfDependencies(t *testing.T) {
	fail := func(context.Context) error { t.Fatal("liveness must not ping dependencies"); return nil }
	w := httptest.NewRecorder()
	NewHandler(Readiness{Postgres: fail, Redis: fail}).ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
