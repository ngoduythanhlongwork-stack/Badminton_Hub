package players

import (
	"embed"

	"badmintonhub/internal/platform/migrations"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

// Migrations returns an independent copy of the Players-owned catalog.
func Migrations() []migrations.Migration {
	files := []struct {
		version int64
		name    string
		path    string
	}{{3, "profiles", "migrations/0003_players_profiles.sql"}, {10, "trust_signals", "migrations/0010_trust_signals.sql"}}
	result := make([]migrations.Migration, 0, len(files))
	for _, file := range files {
		contents, err := migrationSQL.ReadFile(file.path)
		if err != nil {
			panic(err)
		}
		result = append(result, migrations.New(file.version, "players", file.name, string(contents)))
	}
	return result
}

func Catalog() []migrations.Migration { return Migrations() }
