//go:build integration

package main

import (
	"context"
	"testing"

	"badmintonhub/internal/modules/communication"
	"badmintonhub/internal/modules/identity"
	"badmintonhub/internal/modules/matches"
	"badmintonhub/internal/modules/moderation"
	"badmintonhub/internal/modules/notifications"
	"badmintonhub/internal/modules/payments"
	"badmintonhub/internal/modules/players"
	"badmintonhub/internal/modules/recommendations"
	"badmintonhub/internal/modules/venues"
	"badmintonhub/internal/platform/migrations"
	"badmintonhub/internal/platform/testdb"
)

func TestFullManifestAppliesTwice(t *testing.T) {
	pool := testdb.Open(t)
	runner, err := migrations.NewRunner(pool, identity.Migrations(), players.Migrations(), venues.Migrations(), matches.Migrations(), payments.Migrations(), notifications.Migrations(), communication.Migrations(), moderation.Migrations(), recommendations.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	if err = runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = runner.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
}
