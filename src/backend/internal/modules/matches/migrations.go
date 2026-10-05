package matches

import (
	"badmintonhub/internal/platform/migrations"
	"embed"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

func Migrations() []migrations.Migration {
	files := []struct {
		version int64
		name    string
		file    string
	}{
		{5, "matches_and_participation", "migrations/0005_matches.sql"},
		{6, "paid_holds_and_cancellation", "migrations/0006_paid_holds_and_cancellation.sql"},
	}
	result := make([]migrations.Migration, 0, len(files))
	for _, item := range files {
		body, err := migrationSQL.ReadFile(item.file)
		if err != nil {
			panic(err)
		}
		result = append(result, migrations.New(item.version, "matches", item.name, string(body)))
	}
	return result
}
func Catalog() []migrations.Migration { return Migrations() }
