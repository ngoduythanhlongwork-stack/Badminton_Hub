package identity

import (
	"embed"

	"badmintonhub/internal/platform/migrations"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrations returns Identity's globally allocated, forward-only migrations.
func Migrations() []migrations.Migration {
	sql, err := migrationFiles.ReadFile("migrations/0002_identity.sql")
	if err != nil {
		panic(err)
	}
	return []migrations.Migration{migrations.New(2, "identity", "identity_foundation", string(sql))}
}

// Catalog is an alias for composition roots that call module manifests catalogs.
func Catalog() []migrations.Migration { return Migrations() }
