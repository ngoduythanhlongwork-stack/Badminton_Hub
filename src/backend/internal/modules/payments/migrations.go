package payments

import (
	"badmintonhub/internal/platform/migrations"
	"embed"
)

//go:embed migrations/*.sql
var migrationSQL embed.FS

func Migrations() []migrations.Migration {
	body, err := migrationSQL.ReadFile("migrations/0007_payments.sql")
	if err != nil {
		panic(err)
	}
	return []migrations.Migration{migrations.New(7, "payments", "direct_transfer_lifecycle", string(body))}
}

func Catalog() []migrations.Migration { return Migrations() }
