package migrations

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockID int64 = 489934802927566743

type Runner struct {
	pool    *pgxpool.Pool
	catalog []Migration
}

// NewRunner composes platform migrations with module-owned groups supplied by
// the executable composition root.
func NewRunner(pool *pgxpool.Pool, moduleGroups ...[]Migration) (Runner, error) {
	groups := make([][]Migration, 0, len(moduleGroups)+1)
	groups = append(groups, PlatformCatalog())
	groups = append(groups, moduleGroups...)
	catalog, err := Compose(groups...)
	if err != nil {
		return Runner{}, err
	}
	return Runner{pool: pool, catalog: catalog}, nil
}

func (r Runner) Up(ctx context.Context) error {
	if r.pool == nil {
		return errors.New("migration pool is required")
	}
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer func() { _, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID) }()

	metaSchema := pgx.Identifier{"platform"}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+metaSchema); err != nil {
		return fmt.Errorf("create migration schema: %w", err)
	}
	if _, err := conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS "+metaSchema+`.schema_migrations (
        version bigint PRIMARY KEY,
        owner text NOT NULL,
        name text NOT NULL,
        checksum text NOT NULL,
        applied_at timestamptz NOT NULL DEFAULT now()
    )`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}

	for _, migration := range r.catalog {
		if err := apply(ctx, conn, metaSchema, migration); err != nil {
			return err
		}
	}
	return nil
}

func apply(ctx context.Context, conn *pgxpool.Conn, metaSchema string, migration Migration) error {
	checksum := hex.EncodeToString(migration.Hash[:])
	var existing string
	err := conn.QueryRow(ctx, "SELECT checksum FROM "+metaSchema+".schema_migrations WHERE version=$1", migration.Version).Scan(&existing)
	if err == nil {
		if existing != checksum {
			return fmt.Errorf("migration %d checksum changed", migration.Version)
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read migration %d: %w", migration.Version, err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.Version, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO "+metaSchema+".schema_migrations(version, owner, name, checksum) VALUES ($1,$2,$3,$4)", migration.Version, migration.Owner, migration.Name, checksum); err != nil {
		return fmt.Errorf("record migration %d: %w", migration.Version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %d: %w", migration.Version, err)
	}
	return nil
}
