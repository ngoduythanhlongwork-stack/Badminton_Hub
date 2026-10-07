package moderation

import (
	"badmintonhub/internal/platform/migrations"
	"embed"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

func Migrations() []migrations.Migration {
	files := []struct {
		version    int64
		name, path string
	}{{12, "cases_blocks", "migrations/0012_moderation.sql"}, {15, "case_appeals", "migrations/0015_case_appeals.sql"}}
	result := make([]migrations.Migration, 0, len(files))
	for _, file := range files {
		b, e := migrationSQL.ReadFile(file.path)
		if e != nil {
			panic(e)
		}
		result = append(result, migrations.New(file.version, "moderation", file.name, string(b)))
	}
	return result
}
func Catalog() []migrations.Migration { return Migrations() }
