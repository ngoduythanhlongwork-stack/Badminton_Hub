package notifications

import (
	"badmintonhub/internal/platform/migrations"
	"embed"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

func Migrations() []migrations.Migration {
	body, err := migrationSQL.ReadFile("migrations/0008_notifications.sql")
	if err != nil {
		panic(err)
	}
	return []migrations.Migration{migrations.New(8, "notifications", "transactional_delivery", string(body))}
}

func Catalog() []migrations.Migration { return Migrations() }
