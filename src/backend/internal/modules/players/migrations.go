package players

import (
	"embed"

	"badmintonhub/internal/platform/migrations"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

// Migrations returns an independent copy of the Players-owned catalog.
func Migrations() []migrations.Migration {
	contents, err := migrationSQL.ReadFile("migrations/0003_players_profiles.sql")
	if err != nil {
		panic(err)
	}
	return []migrations.Migration{migrations.New(3, "players", "profiles", string(contents))}
}

func Catalog() []migrations.Migration { return Migrations() }
