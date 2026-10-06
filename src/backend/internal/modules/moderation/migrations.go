package moderation

import (
	"badmintonhub/internal/platform/migrations"
	"embed"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

func Migrations() []migrations.Migration {
	b, e := migrationSQL.ReadFile("migrations/0012_moderation.sql")
	if e != nil {
		panic(e)
	}
	return []migrations.Migration{migrations.New(12, "moderation", "cases_blocks", string(b))}
}
func Catalog() []migrations.Migration { return Migrations() }
