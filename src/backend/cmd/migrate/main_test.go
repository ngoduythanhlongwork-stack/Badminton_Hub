package main

import (
	"testing"

	"badmintonhub/internal/modules/identity"
	"badmintonhub/internal/modules/matches"
	"badmintonhub/internal/modules/notifications"
	"badmintonhub/internal/modules/payments"
	"badmintonhub/internal/modules/players"
	"badmintonhub/internal/modules/venues"
	"badmintonhub/internal/platform/migrations"
)

func TestR1MigrationManifestIsOrderedAndComplete(t *testing.T) {
	manifest, err := migrations.Compose(migrations.PlatformCatalog(), identity.Migrations(), players.Migrations(), venues.Migrations(), matches.Migrations(), payments.Migrations(), notifications.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest) != 8 {
		t.Fatalf("migration count=%d", len(manifest))
	}
	for index, migration := range manifest {
		if migration.Version != int64(index+1) {
			t.Fatalf("migration[%d]=%d", index, migration.Version)
		}
	}
}
