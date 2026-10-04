//go:build integration

package connections

import (
	"context"
	"os"
	"testing"
	"time"

	"badmintonhub/internal/config"
)

func TestLiveConnections(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" || os.Getenv("REDIS_URL") == "" {
		t.Fatal("integration tests require explicit DATABASE_URL and REDIS_URL")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clients, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()
	var value int
	if err := clients.Postgres.QueryRow(ctx, "SELECT 1").Scan(&value); err != nil || value != 1 {
		t.Fatal("PostgreSQL SELECT 1 failed")
	}
	if err := clients.PingRedis(ctx); err != nil {
		t.Fatal("Redis PING failed")
	}
}
