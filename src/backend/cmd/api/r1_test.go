package main

import (
	"testing"

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
