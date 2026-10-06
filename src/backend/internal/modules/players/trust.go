package players

import (
	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/id"
	"badmintonhub/internal/platform/outbox"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TrustSignal struct {
	EventID, AccountID, SourceType, SourceID, MatchID, SkillDirection string
	SourceRevision                                                    int
	ReliabilityValue                                                  *float64
	OccurredAt                                                        time.Time
}

type TrustProcessor struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

func NewTrustProcessor(pool *pgxpool.Pool, c clock.Clock) (*TrustProcessor, error) {
	if pool == nil {
		return nil, fmt.Errorf("trust processor pool is required")
	}
	if c == nil {
		c = clock.System{}
	}
	return &TrustProcessor{pool: pool, clock: c}, nil
}

func (p *TrustProcessor) Handler() outbox.Handler {
	return func(ctx context.Context, message outbox.Message) error {
		var signal TrustSignal
		if err := json.Unmarshal(message.Payload, &signal); err != nil {
			return fmt.Errorf("decode trust signal: %w", err)
		}
		return p.Apply(ctx, signal)
	}
}

func (p *TrustProcessor) Apply(ctx context.Context, signal TrustSignal) error {
	if signal.AccountID == "" || signal.SourceID == "" || signal.MatchID == "" || signal.SourceRevision <= 0 {
		return fmt.Errorf("invalid trust signal")
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `UPDATE players.trust_signals SET effective=false WHERE source_type=$1 AND source_id=$2 AND effective AND source_revision < $3`, signal.SourceType, signal.SourceID, signal.SourceRevision); err != nil {
		return err
	}
	signalID, _ := id.New()
	if _, err = tx.Exec(ctx, `INSERT INTO players.trust_signals(id,account_id,source_type,source_id,source_revision,match_id,reliability_value,skill_direction,effective,occurred_at,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,true,$9,$10) ON CONFLICT(source_type,source_id,source_revision) DO NOTHING`, signalID, signal.AccountID, signal.SourceType, signal.SourceID, signal.SourceRevision, signal.MatchID, signal.ReliabilityValue, nullString(signal.SkillDirection), signal.OccurredAt, p.clock.Now().UTC()); err != nil {
		return err
	}
	return recomputeTrust(ctx, tx, signal.AccountID, p.clock.Now().UTC())
}

func recomputeTrust(ctx context.Context, tx pgx.Tx, accountID string, now time.Time) error {
	rows, err := tx.Query(ctx, `SELECT reliability_value::float8 FROM players.trust_signals WHERE account_id=$1 AND effective AND reliability_value IS NOT NULL ORDER BY occurred_at DESC,source_id DESC LIMIT 20`, accountID)
	if err != nil {
		return err
	}
	values := []float64{}
	for rows.Next() {
		var v float64
		if err = rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		values = append(values, v)
	}
	rows.Close()
	label := ReliabilityNew
	var score *float64
	if len(values) > 0 {
		total := 0.0
		for _, v := range values {
			total += v
		}
		s := total / float64(len(values)) * 100
		score = &s
		if len(values) >= 3 {
			switch {
			case s >= 90:
				label = ReliabilityReliable
			case s >= 75:
				label = ReliabilityGood
			default:
				label = ReliabilityNeedsImprovement
			}
		}
	}
	var feedbackCount, matchCount, stronger, lower int
	err = tx.QueryRow(ctx, `SELECT count(*),count(DISTINCT match_id),count(*) FILTER(WHERE skill_direction='STRONGER_THAN_PROFILE'),count(*) FILTER(WHERE skill_direction='LOWER_THAN_PROFILE') FROM players.trust_signals WHERE account_id=$1 AND effective AND source_type='SKILL_FEEDBACK'`, accountID).Scan(&feedbackCount, &matchCount, &stronger, &lower)
	if err != nil {
		return err
	}
	confidence := SkillConfidenceNew
	if feedbackCount >= 10 {
		confidence = SkillConfidenceEstablished
	} else if feedbackCount >= 3 {
		confidence = SkillConfidenceDeveloping
	}
	directional := stronger
	if lower > directional {
		directional = lower
	}
	suggested := feedbackCount >= 5 && matchCount >= 3 && float64(directional)/float64(feedbackCount) >= 0.70
	_, err = tx.Exec(ctx, `UPDATE players.profiles SET reliability_label=$2,reliability_score=$3,reliability_sample_size=$4,skill_confidence=$5,skill_feedback_count=$6,level_review_suggested=$7,match_count=(SELECT count(DISTINCT match_id) FROM players.trust_signals WHERE account_id=$1 AND effective AND reliability_value=1),updated_at=$8,version=version+1 WHERE account_id=$1`, accountID, label, score, len(values), confidence, feedbackCount, suggested, now)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
