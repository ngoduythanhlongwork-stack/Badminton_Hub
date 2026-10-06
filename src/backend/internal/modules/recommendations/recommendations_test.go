package recommendations

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestScoreIsDeterministicAndUsesD23Weights(t *testing.T) {
	profile := Profile{SkillLevel: 3, Area: "Quan 1", Formats: []string{"DOUBLES"}, Styles: []string{"SOCIAL"}, Periods: []string{"EVENING"}}
	candidate := Candidate{ID: "m1", Area: "Quan 1", TimeZone: "Asia/Ho_Chi_Minh", Format: "DOUBLES", Style: "SOCIAL", StartAt: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), MinLevel: 2, MaxLevel: 4}
	first, reasons := score(profile, candidate, "RELIABLE")
	second, again := score(profile, candidate, "RELIABLE")
	if first != 92.5 || second != first {
		t.Fatalf("score=%v second=%v", first, second)
	}
	if !reflect.DeepEqual(reasons, again) {
		t.Fatalf("reasons changed: %v / %v", reasons, again)
	}
	want := []string{"SKILL_MATCH", "SAME_AREA", "USUAL_TIME", "FORMAT_MATCH", "STYLE_MATCH", "RELIABLE_HOST"}
	if !reflect.DeepEqual(reasons, want) {
		t.Fatalf("reasons=%v", reasons)
	}
}

func TestScoreRenormalizesMissingAreaAndUsesNeutralNewHost(t *testing.T) {
	profile := Profile{SkillLevel: 1, Formats: []string{"SINGLES"}, Periods: []string{"MORNING"}}
	candidate := Candidate{TimeZone: "Asia/Ho_Chi_Minh", Format: "DOUBLES", StartAt: time.Date(2026, 10, 10, 1, 0, 0, 0, time.UTC), MinLevel: 5, MaxLevel: 6}
	value, reasons := score(profile, candidate, "NEW")
	if value <= 0 {
		t.Fatalf("newcomer fallback must not be zero: %v", value)
	}
	for _, reason := range reasons {
		if reason == "SAME_AREA" {
			t.Fatal("missing area produced a distance reason")
		}
	}
}

func TestTieBreakIsStartThenStableID(t *testing.T) {
	start := time.Now().UTC()
	items := []Item{{MatchID: "b", Score: 80, StartAt: start}, {MatchID: "a", Score: 80, StartAt: start}}
	sortItems(items)
	if items[0].MatchID != "a" {
		t.Fatalf("order=%v", items)
	}
}

func sortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if !items[i].StartAt.Equal(items[j].StartAt) {
			return items[i].StartAt.Before(items[j].StartAt)
		}
		return items[i].MatchID < items[j].MatchID
	})
}
