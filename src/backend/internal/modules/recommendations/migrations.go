package recommendations

import (
	"badmintonhub/internal/platform/migrations"
	"embed"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

func Migrations() []migrations.Migration {
	b, e := migrationSQL.ReadFile("migrations/0013_recommendations_measurement.sql")
	if e != nil {
		panic(e)
	}
	return []migrations.Migration{migrations.New(13, "recommendations", "v1_and_measurement", string(b))}
}
func Catalog() []migrations.Migration { return Migrations() }
