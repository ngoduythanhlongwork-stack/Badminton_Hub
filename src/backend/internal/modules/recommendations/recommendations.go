package recommendations

import (
	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/id"
	"badmintonhub/internal/platform/outbox"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"sort"
	"strings"
	"time"
)

const ScoringVersion = "D23-V1"

var (
	ErrInvalid = errors.New("invalid recommendation request")
)

type Profile struct {
	AccountID                string
	SkillLevel               int
	Formats, Styles, Periods []string
	Area, Reliability        string
}
type Candidate struct {
	ID, HostID, Area, TimeZone, Format, Style, JoinMode string
	StartAt, EndAt                                      time.Time
	MinLevel, MaxLevel                                  int
}
type ProfileProvider interface {
	RecommendationProfile(context.Context, string) (Profile, error)
	HostReliabilities(context.Context, []string) (map[string]string, error)
}
type CandidateProvider interface {
	RecommendationCandidates(context.Context, string, int) ([]Candidate, error)
}
type InteractionPolicy interface {
	Blocked(context.Context, string, string) (bool, error)
}
type Item struct {
	MatchID string    `json:"matchId"`
	Score   float64   `json:"score"`
	Reasons []string  `json:"reasons"`
	StartAt time.Time `json:"startAt"`
}
type Result struct {
	ScoringVersion string    `json:"scoringVersion"`
	Items          []Item    `json:"items"`
	CalculatedAt   time.Time `json:"calculatedAt"`
}
type Service struct {
	pool       *pgxpool.Pool
	profiles   ProfileProvider
	candidates CandidateProvider
	policy     InteractionPolicy
	clock      clock.Clock
}

func NewService(pool *pgxpool.Pool, p ProfileProvider, c CandidateProvider, policy InteractionPolicy, clk clock.Clock) (*Service, error) {
	if pool == nil || p == nil || c == nil {
		return nil, ErrInvalid
	}
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{pool: pool, profiles: p, candidates: c, policy: policy, clock: clk}, nil
}
func (s *Service) Recommend(ctx context.Context, player string) (Result, error) {
	profile, err := s.profiles.RecommendationProfile(ctx, player)
	if err != nil {
		return Result{}, err
	}
	candidates, err := s.candidates.RecommendationCandidates(ctx, player, 1000)
	if err != nil {
		return Result{}, err
	}
	hostIDs := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if !seen[candidate.HostID] {
			seen[candidate.HostID] = true
			hostIDs = append(hostIDs, candidate.HostID)
		}
	}
	reliabilities, err := s.profiles.HostReliabilities(ctx, hostIDs)
	if err != nil {
		return Result{}, err
	}
	items := make([]Item, 0, len(candidates))
	for _, candidate := range candidates {
		if s.policy != nil {
			blocked, e := s.policy.Blocked(ctx, player, candidate.HostID)
			if e != nil {
				return Result{}, e
			}
			if blocked {
				continue
			}
		}
		reliability := reliabilities[candidate.HostID]
		if reliability == "" {
			reliability = "NEW"
		}
		score, reasons := score(profile, candidate, reliability)
		items = append(items, Item{MatchID: candidate.ID, Score: math.Round(score*100) / 100, Reasons: reasons, StartAt: candidate.StartAt})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if !items[i].StartAt.Equal(items[j].StartAt) {
			return items[i].StartAt.Before(items[j].StartAt)
		}
		return items[i].MatchID < items[j].MatchID
	})
	if len(items) > 20 {
		items = items[:20]
	}
	result := Result{ScoringVersion: ScoringVersion, Items: items, CalculatedAt: s.clock.Now().UTC()}
	if err = s.recordExposure(ctx, player, len(candidates), result); err != nil {
		return Result{}, err
	}
	return result, nil
}
func score(p Profile, c Candidate, hostReliability string) (float64, []string) {
	type component struct {
		weight, value float64
		reason        string
	}
	components := []component{}
	skill := 0.0
	if p.SkillLevel >= c.MinLevel && p.SkillLevel <= c.MaxLevel {
		skill = 100
	} else if c.JoinMode == "APPROVAL_REQUIRED" && (p.SkillLevel == c.MinLevel-1 || p.SkillLevel == c.MaxLevel+1) {
		skill = 50
	}
	components = append(components, component{35, skill, reasonIf(skill == 100, "SKILL_MATCH")})
	if p.Area != "" && c.Area != "" {
		same := strings.EqualFold(p.Area, c.Area)
		v := 20.0
		if same {
			v = 70
		}
		components = append(components, component{25, v, reasonIf(same, "SAME_AREA")})
	}
	period := periodAt(c.StartAt, c.TimeZone)
	timeMatch := contains(p.Periods, period)
	tv := 40.0
	if timeMatch {
		tv = 100
	}
	components = append(components, component{20, tv, reasonIf(timeMatch, "USUAL_TIME")})
	prefTotal := 0.0
	prefParts := 0.0
	reasons := []string{}
	if len(p.Formats) > 0 {
		prefParts++
		if contains(p.Formats, c.Format) {
			prefTotal += 100
			reasons = append(reasons, "FORMAT_MATCH")
		}
	}
	if len(p.Styles) > 0 {
		prefParts++
		if contains(p.Styles, c.Style) {
			prefTotal += 100
			reasons = append(reasons, "STYLE_MATCH")
		}
	}
	if prefParts > 0 {
		components = append(components, component{15, prefTotal / prefParts, strings.Join(reasons, "+")})
	}
	trust := map[string]float64{"RELIABLE": 100, "GOOD": 80, "NEEDS_IMPROVEMENT": 40, "NEW": 70}[hostReliability]
	components = append(components, component{5, trust, reasonIf(hostReliability == "RELIABLE", "RELIABLE_HOST")})
	weighted, total := 0.0, 0.0
	outReasons := []string{}
	for _, x := range components {
		weighted += x.weight * x.value
		total += x.weight
		if x.reason != "" {
			outReasons = append(outReasons, strings.Split(x.reason, "+")...)
		}
	}
	return weighted / total, outReasons
}
func (s *Service) recordExposure(ctx context.Context, player string, candidateCount int, result Result) error {
	idValue, _ := id.New()
	ids := make([]string, len(result.Items))
	reasonMap := map[string][]string{}
	for i, item := range result.Items {
		ids[i] = item.MatchID
		reasonMap[item.MatchID] = item.Reasons
	}
	body, _ := json.Marshal(reasonMap)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `INSERT INTO recommendations.exposures(id,player_id,scoring_version,candidate_count,result_match_ids,reason_codes,exposed_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, idValue, player, ScoringVersion, candidateCount, ids, body, result.CalculatedAt); err != nil {
		return err
	}
	analyticsID, _ := id.New()
	payload, _ := json.Marshal(map[string]any{"scoringVersion": ScoringVersion, "candidateCount": candidateCount, "resultMatchIds": ids, "reasons": reasonMap})
	if _, err = tx.Exec(ctx, `INSERT INTO recommendations.analytics_events(id,event_key,event_type,actor_id,payload,occurred_at,recorded_at) VALUES($1,$2,'RecommendationExposed',$3,$4,$5,$5)`, analyticsID, "recommendation:"+idValue, player, payload, result.CalculatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func periodAt(at time.Time, zone string) string {
	location, e := time.LoadLocation(zone)
	if e == nil {
		at = at.In(location)
	}
	if at.Hour() < 12 {
		return "MORNING"
	}
	if at.Hour() < 18 {
		return "AFTERNOON"
	}
	return "EVENING"
}
func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
func reasonIf(ok bool, value string) string {
	if ok {
		return value
	}
	return ""
}

type Measurement struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewMeasurement(pool *pgxpool.Pool, c clock.Clock) *Measurement {
	if c == nil {
		c = clock.System{}
	}
	return &Measurement{pool: pool, clock: c}
}
func (m *Measurement) TrustSignalHandler() outbox.Handler {
	return func(ctx context.Context, message outbox.Message) error {
		var e struct {
			AccountID, SourceType, SourceID, MatchID string
			SourceRevision                           int
			ReliabilityValue                         *float64
			OccurredAt                               time.Time
		}
		if err := json.Unmarshal(message.Payload, &e); err != nil {
			return err
		}
		if e.SourceType != "ATTENDANCE" {
			idValue, _ := id.New()
			eventType := "ReviewSubmitted"
			if e.SourceType == "LATE_CANCEL" {
				eventType = "ParticipationCancelled"
			}
			_, err := m.pool.Exec(ctx, `INSERT INTO recommendations.analytics_events(id,event_key,event_type,source_id,source_revision,actor_id,match_id,payload,occurred_at,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,'{}',$8,$9) ON CONFLICT(event_key) DO NOTHING`, idValue, message.IdempotencyKey, eventType, e.SourceID, e.SourceRevision, e.AccountID, e.MatchID, e.OccurredAt, m.clock.Now().UTC())
			return err
		}
		outcome := "UNKNOWN"
		if e.ReliabilityValue != nil {
			if *e.ReliabilityValue == 0 {
				outcome = "NO_SHOW"
			} else {
				outcome = "PRESENT"
			}
		}
		idValue, _ := id.New()
		payload, _ := json.Marshal(map[string]any{"outcome": outcome})
		tx, err := m.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(context.Background())
		eventType := "AttendanceConfirmed"
		if e.SourceRevision > 1 {
			eventType = "AttendanceCorrected"
		}
		if _, err = tx.Exec(ctx, `INSERT INTO recommendations.analytics_events(id,event_key,event_type,source_id,source_revision,actor_id,match_id,payload,occurred_at,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(event_key) DO NOTHING`, idValue, message.IdempotencyKey, eventType, e.SourceID, e.SourceRevision, e.AccountID, e.MatchID, payload, e.OccurredAt, m.clock.Now().UTC()); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO recommendations.attendance_outcomes(participation_id,match_id,revision,outcome,occurred_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(participation_id) DO UPDATE SET revision=EXCLUDED.revision,outcome=EXCLUDED.outcome,occurred_at=EXCLUDED.occurred_at WHERE recommendations.attendance_outcomes.revision<EXCLUDED.revision`, e.SourceID, e.MatchID, e.SourceRevision, outcome, e.OccurredAt); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}

func (m *Measurement) OutcomeHandler(eventType string) outbox.Handler {
	return func(ctx context.Context, message outbox.Message) error {
		idValue, _ := id.New()
		_, err := m.pool.Exec(ctx, `INSERT INTO recommendations.analytics_events(id,event_key,event_type,payload,occurred_at,recorded_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(event_key) DO NOTHING`, idValue, eventType+":"+message.IdempotencyKey, eventType, message.Payload, message.OccurredAt, m.clock.Now().UTC())
		return err
	}
}

type AttendanceMetrics struct {
	Expected   int `json:"expected"`
	Present    int `json:"present"`
	NoShow     int `json:"noShow"`
	Unresolved int `json:"unresolved"`
}

func (m *Measurement) Attendance(ctx context.Context) (AttendanceMetrics, error) {
	var v AttendanceMetrics
	err := m.pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE outcome='PRESENT'),count(*) FILTER(WHERE outcome='NO_SHOW'),count(*) FILTER(WHERE outcome='UNKNOWN') FROM recommendations.attendance_outcomes`).Scan(&v.Expected, &v.Present, &v.NoShow, &v.Unresolved)
	return v, err
}
