package venues

import (
	"badmintonhub/internal/platform/migrations"
	"embed"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

func Migrations() []migrations.Migration {
	b, err := migrationSQL.ReadFile("migrations/0004_venues.sql")
	if err != nil {
		panic(err)
	}
	return []migrations.Migration{migrations.New(4, "venues", "venue_catalog", string(b))}
}
func Catalog() []migrations.Migration { return Migrations() }
