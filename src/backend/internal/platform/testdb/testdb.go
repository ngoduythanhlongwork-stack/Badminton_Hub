// Package testdb provides database-per-test isolation for integration tests.
// The configured PostgreSQL role must have CREATEDB; this prevents tests from
// sharing schemas, rows, sequences, or transaction state.
package testdb

import (
	"context"
	"os"
	"testing"
	"time"

	"badmintonhub/internal/platform/id"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(t testing.TB) *pgxpool.Pool {
	t.Helper()
	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for isolated integration tests")
	}
	config, err := pgxpool.ParseConfig(baseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	generated, err := id.New()
	if err != nil {
		t.Fatal(err)
	}
	databaseName := "badminton_test_" + generated[:8]
	adminConfig := config.Copy()
	adminConfig.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(context.Background(), adminConfig)
	if err != nil {
		t.Fatalf("connect to test database admin: %v", err)
	}
	identifier := pgx.Identifier{databaseName}.Sanitize()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		admin.Close()
		t.Fatalf("create isolated test database (role needs CREATEDB): %v", err)
	}
	config.ConnConfig.Database = databaseName
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+identifier+" WITH (FORCE)")
		admin.Close()
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			t.Errorf("drop isolated test database %s: %v", databaseName, err)
		}
		admin.Close()
	})
	return pool
}
