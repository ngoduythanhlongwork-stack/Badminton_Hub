package matches

import (
	"badmintonhub/internal/platform/migrations"
	"embed"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

func Migrations() []migrations.Migration {
	b, e := migrationSQL.ReadFile("migrations/0005_matches.sql")
	if e != nil {
		panic(e)
	}
	return []migrations.Migration{migrations.New(5, "matches", "matches_and_participation", string(b))}
}
func Catalog() []migrations.Migration { return Migrations() }
