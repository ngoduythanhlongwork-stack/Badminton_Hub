package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"badmintonhub/internal/modules/players"
)

func TestSkillRankUsesApprovedOrdering(t *testing.T) {
	levels := []players.SkillLevel{players.SkillBeginner, players.SkillBeginnerPlus, players.SkillIntermediate, players.SkillIntermediatePlus, players.SkillAdvanced, players.SkillCompetitive}
	for index, level := range levels {
		if got := skillRank(level); got != index+1 {
			t.Fatalf("level=%s rank=%d", level, got)
		}
	}
	if got := skillRank(players.SkillLevel("UNKNOWN")); got != 0 {
		t.Fatalf("unknown rank=%d", got)
	}
}

func TestMaintenanceRunsImmediatelyAndStops(t *testing.T) {
	called := make(chan struct{}, 1)
	worker := startMaintenance(context.Background(), func(context.Context) error {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("maintenance did not run immediately")
	}
	if err := worker.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
}
