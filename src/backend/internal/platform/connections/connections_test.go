package connections

import (
	"context"
	"strings"
	"testing"
	"time"

	"badmintonhub/internal/config"
)

func TestRejectsMalformedURLsWithoutLeakingSecrets(t *testing.T) {
	for _, cfg := range []config.Config{
		{DatabaseURL: "postgres://user:secret@host:invalid/db", RedisURL: "redis://localhost:6379"},
		{DatabaseURL: "postgres://localhost/db", RedisURL: "redis://user:secret@host:invalid"},
	} {
		cfg.DependencyTimeout = time.Second
		clients, err := Open(context.Background(), cfg)
		if clients != nil {
			clients.Close()
			t.Fatal("unexpected clients")
		}
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("expected sanitized configuration error")
		}
	}
}

func TestCancelledStartupReturnsNoClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	clients, err := Open(ctx, config.Config{
		DatabaseURL:       "postgres://user:secret@127.0.0.1:5432/test?sslmode=disable",
		RedisURL:          "redis://127.0.0.1:6379",
		DependencyTimeout: time.Second,
	})
	if clients != nil {
		clients.Close()
		t.Fatal("unexpected clients")
	}
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("expected sanitized startup error")
	}
}
