//go:build integration

package migrations

import (
	"context"
	"testing"

	"badmintonhub/internal/platform/testdb"
)

func TestRunnerAppliesAndIsIdempotent(t *testing.T) {
	pool := testdb.Open(t)
	runner, err := NewRunner(pool)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(context.Background()); err != nil {
		t.Fatalf("second migration run must be unchanged: %v", err)
	}
	var migrations, tables int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM platform.schema_migrations").Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM information_schema.tables
        WHERE table_schema='platform' AND table_name='outbox_messages'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if migrations != len(PlatformCatalog()) || tables != 1 {
		t.Fatalf("migrations=%d tables=%d", migrations, tables)
	}
}
