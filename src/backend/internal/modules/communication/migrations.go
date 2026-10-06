package communication

import (
	"badmintonhub/internal/platform/migrations"
	"embed"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

func Migrations() []migrations.Migration {
	body, err := migrationSQL.ReadFile("migrations/0011_match_room.sql")
	if err != nil {
		panic(err)
	}
	return []migrations.Migration{migrations.New(11, "communication", "match_room", string(body))}
}
func Catalog() []migrations.Migration { return Migrations() }
