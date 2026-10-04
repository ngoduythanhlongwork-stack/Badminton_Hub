package players

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PostgresRepository struct{ db DBTX }

func NewPostgresRepository(db DBTX) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) Get(ctx context.Context, accountID string) (OwnerProfile, error) {
	const query = `SELECT account_id::text, onboarding_status, display_name, avatar_url,
date_of_birth::text, gender, experience, skill_level, preferred_formats, play_styles,
usual_periods, regular_area, skill_confidence, reliability_label, match_count,
completed_at, updated_at, version FROM players.profiles WHERE account_id = $1`
	var p OwnerProfile
	var dob *string
	var gender, experience, skill *string
	var formats, styles, periods []string
	err := r.db.QueryRow(ctx, query, accountID).Scan(&p.AccountID, &p.Status, &p.DisplayName, &p.AvatarURL,
		&dob, &gender, &experience, &skill, &formats, &styles, &periods, &p.RegularArea,
		&p.SkillConfidence, &p.Reliability, &p.MatchCount, &p.CompletedAt, &p.UpdatedAt, &p.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnerProfile{}, ErrNotFound
	}
	if err != nil {
		return OwnerProfile{}, fmt.Errorf("get player profile: %w", err)
	}
	if dob != nil {
		parsed, parseErr := ParseCalendarDate(*dob)
		if parseErr != nil {
			return OwnerProfile{}, fmt.Errorf("decode player date: %w", parseErr)
		}
		p.DateOfBirth = &parsed
	}
	if gender != nil {
		value := Gender(*gender)
		p.Gender = &value
	}
	if experience != nil {
		value := Experience(*experience)
		p.Experience = &value
	}
	if skill != nil {
		value := SkillLevel(*skill)
		p.SkillLevel = &value
	}
	p.PreferredFormats = fromStrings[GameFormat](formats)
	p.PlayStyles = fromStrings[PlayStyle](styles)
	p.UsualPeriods = fromStrings[UsualPeriod](periods)
	return p, nil
}

func (r *PostgresRepository) Save(ctx context.Context, p OwnerProfile) (OwnerProfile, error) {
	const insert = `INSERT INTO players.profiles
(account_id, onboarding_status, display_name, avatar_url, date_of_birth, gender, experience, skill_level,
 preferred_formats, play_styles, usual_periods, regular_area, skill_confidence, reliability_label,
 match_count, completed_at, updated_at, version)
VALUES ($1,$2,$3,$4,$5::date,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,1)
ON CONFLICT (account_id) DO NOTHING RETURNING version`
	const update = `UPDATE players.profiles SET onboarding_status=$2, display_name=$3, avatar_url=$4,
date_of_birth=$5::date, gender=$6, experience=$7, skill_level=$8, preferred_formats=$9,
play_styles=$10, usual_periods=$11, regular_area=$12, skill_confidence=$13,
reliability_label=$14, match_count=$15, completed_at=$16, updated_at=$17, version=version+1
WHERE account_id=$1 AND version=$18 RETURNING version`
	args := profileArgs(p)
	var version int64
	var err error
	if p.Version == 0 {
		err = r.db.QueryRow(ctx, insert, args[:17]...).Scan(&version)
	} else {
		err = r.db.QueryRow(ctx, update, append(args[:17], p.Version)...).Scan(&version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnerProfile{}, ErrConflict
	}
	if err != nil {
		return OwnerProfile{}, fmt.Errorf("save player profile: %w", err)
	}
	p.Version = version
	return p, nil
}

func profileArgs(p OwnerProfile) []any {
	var dob any
	if p.DateOfBirth != nil {
		dob = p.DateOfBirth.String()
	}
	return []any{p.AccountID, p.Status, p.DisplayName, p.AvatarURL, dob, p.Gender, p.Experience,
		p.SkillLevel, toStrings(p.PreferredFormats), toStrings(p.PlayStyles), toStrings(p.UsualPeriods),
		p.RegularArea, p.SkillConfidence, p.Reliability, p.MatchCount, p.CompletedAt, p.UpdatedAt}
}

func toStrings[T ~string](values []T) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = string(value)
	}
	return result
}
func fromStrings[T ~string](values []string) []T {
	result := make([]T, len(values))
	for i, value := range values {
		result[i] = T(value)
	}
	return result
}

var _ Repository = (*PostgresRepository)(nil)
