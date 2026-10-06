// Package players owns player onboarding, profiles, preferences, and the
// player-facing skill and reliability projections.
package players

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const eligibilityTimeZone = "Asia/Ho_Chi_Minh"

var (
	ErrNotFound         = errors.New("player profile not found")
	ErrInvalidProfile   = errors.New("invalid player profile")
	ErrUnderage         = errors.New("player must be at least 18 years old")
	ErrDateOfBirthFixed = errors.New("date of birth cannot be changed after onboarding")
	ErrConflict         = errors.New("player profile was changed concurrently")
)

type OnboardingStatus string

const (
	OnboardingNotStarted OnboardingStatus = "NOT_STARTED"
	OnboardingInProgress OnboardingStatus = "IN_PROGRESS"
	OnboardingComplete   OnboardingStatus = "COMPLETE"
)

type Experience string

const (
	ExperienceUnder3Months Experience = "UNDER_3_MONTHS"
	Experience3To12Months  Experience = "3_TO_12_MONTHS"
	Experience1To3Years    Experience = "1_TO_3_YEARS"
	Experience3PlusYears   Experience = "3_PLUS_YEARS"
)

type SkillLevel string

const (
	SkillBeginner         SkillLevel = "BEGINNER"
	SkillBeginnerPlus     SkillLevel = "BEGINNER_PLUS"
	SkillIntermediate     SkillLevel = "INTERMEDIATE"
	SkillIntermediatePlus SkillLevel = "INTERMEDIATE_PLUS"
	SkillAdvanced         SkillLevel = "ADVANCED"
	SkillCompetitive      SkillLevel = "COMPETITIVE"
)

type GameFormat string

const (
	FormatSingles GameFormat = "SINGLES"
	FormatDoubles GameFormat = "DOUBLES"
	FormatMixed   GameFormat = "MIXED"
)

type PlayStyle string

const (
	StyleCasual      PlayStyle = "CASUAL"
	StyleSocial      PlayStyle = "SOCIAL"
	StyleTraining    PlayStyle = "TRAINING"
	StyleCompetitive PlayStyle = "COMPETITIVE"
)

type UsualPeriod string

const (
	PeriodMorning   UsualPeriod = "MORNING"
	PeriodAfternoon UsualPeriod = "AFTERNOON"
	PeriodEvening   UsualPeriod = "EVENING"
)

type Gender string

const (
	GenderFemale      Gender = "FEMALE"
	GenderMale        Gender = "MALE"
	GenderNonBinary   Gender = "NON_BINARY"
	GenderUndisclosed Gender = "UNDISCLOSED"
)

type SkillConfidence string

const (
	SkillConfidenceNew         SkillConfidence = "NEW"
	SkillConfidenceDeveloping  SkillConfidence = "DEVELOPING"
	SkillConfidenceEstablished SkillConfidence = "ESTABLISHED"
)

type ReliabilityLabel string

const (
	ReliabilityNew              ReliabilityLabel = "NEW"
	ReliabilityReliable         ReliabilityLabel = "RELIABLE"
	ReliabilityGood             ReliabilityLabel = "GOOD"
	ReliabilityNeedsImprovement ReliabilityLabel = "NEEDS_IMPROVEMENT"
)

// CalendarDate deliberately has no time zone or clock component.
type CalendarDate struct {
	Year  int
	Month time.Month
	Day   int
}

func NewCalendarDate(year int, month time.Month, day int) (CalendarDate, error) {
	date := CalendarDate{Year: year, Month: month, Day: day}
	if !date.Valid() {
		return CalendarDate{}, fmt.Errorf("%w: invalid calendar date", ErrInvalidProfile)
	}
	return date, nil
}
func ParseCalendarDate(value string) (CalendarDate, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return CalendarDate{}, fmt.Errorf("%w: invalid date of birth", ErrInvalidProfile)
	}
	return CalendarDate{Year: parsed.Year(), Month: parsed.Month(), Day: parsed.Day()}, nil
}
func (d CalendarDate) Valid() bool {
	if d.Year < 1 || d.Month < time.January || d.Month > time.December || d.Day < 1 {
		return false
	}
	value := time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
	return value.Year() == d.Year && value.Month() == d.Month && value.Day() == d.Day
}
func (d CalendarDate) String() string {
	if !d.Valid() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// Draft is a full draft snapshot. Nil fields represent data not yet entered.
type Draft struct {
	DisplayName      *string
	AvatarURL        *string
	DateOfBirth      *CalendarDate
	Gender           *Gender
	Experience       *Experience
	SkillLevel       *SkillLevel
	PreferredFormats []GameFormat
	PlayStyles       []PlayStyle
	UsualPeriods     []UsualPeriod
	RegularArea      *string
}

type OwnerProfile struct {
	AccountID             string
	Status                OnboardingStatus
	DisplayName           *string
	AvatarURL             *string
	DateOfBirth           *CalendarDate
	Gender                *Gender
	Experience            *Experience
	SkillLevel            *SkillLevel
	PreferredFormats      []GameFormat
	PlayStyles            []PlayStyle
	UsualPeriods          []UsualPeriod
	RegularArea           *string
	SkillConfidence       SkillConfidence
	Reliability           ReliabilityLabel
	ReliabilityScore      *float64
	ReliabilitySampleSize int
	SkillFeedbackCount    int
	LevelReviewSuggested  bool
	MatchCount            int
	CompletedAt           *time.Time
	UpdatedAt             time.Time
	Version               int64
}

// PublicProfile is the D-09 minimal social-proof projection. Preferences,
// location, DOB, contact data, and internal confidence are deliberately private.
type PublicProfile struct {
	AccountID             string
	DisplayName           string
	SkillLevel            SkillLevel
	Reliability           ReliabilityLabel
	ReliabilityScore      *float64
	ReliabilitySampleSize int
	MatchCount            int
}

// Eligibility is the stable cross-module view used by Identity and Matches.
type Eligibility struct {
	AccountID          string
	OnboardingStatus   OnboardingStatus
	Adult              bool
	CanParticipate     bool
	SkillLevel         *SkillLevel
	SkillConfidence    SkillConfidence
	Reliability        ReliabilityLabel
	EligibilityChecked time.Time
}

type RecommendationProfile struct {
	AccountID       string
	SkillLevel      int
	Formats, Styles []string
	Periods         []string
	Area            string
	Reliability     ReliabilityLabel
}

type Repository interface {
	Get(context.Context, string) (OwnerProfile, error)
	Save(context.Context, OwnerProfile) (OwnerProfile, error)
}
type Clock interface{ Now() time.Time }

type Service struct {
	repository Repository
	clock      Clock
	logger     *slog.Logger
	location   *time.Location
}

func NewService(repository Repository, clock Clock, logger *slog.Logger) (*Service, error) {
	if repository == nil || clock == nil {
		return nil, errors.New("players service requires repository and clock")
	}
	location, err := time.LoadLocation(eligibilityTimeZone)
	if err != nil {
		return nil, fmt.Errorf("load eligibility time zone: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repository: repository, clock: clock, logger: logger, location: location}, nil
}

func (s *Service) GetOwnerProfile(ctx context.Context, accountID string) (OwnerProfile, error) {
	profile, err := s.repository.Get(ctx, accountID)
	if errors.Is(err, ErrNotFound) {
		return newProfile(accountID), nil
	}
	return profile, err
}
func (s *Service) SaveDraft(ctx context.Context, accountID string, draft Draft) (OwnerProfile, error) {
	profile, err := s.GetOwnerProfile(ctx, accountID)
	if err != nil {
		return OwnerProfile{}, err
	}
	if profile.Status == OnboardingComplete && !sameDate(profile.DateOfBirth, draft.DateOfBirth) {
		return OwnerProfile{}, ErrDateOfBirthFixed
	}
	applyDraft(&profile, draft)
	if err := validateEntered(profile); err != nil {
		return OwnerProfile{}, err
	}
	if profile.Status != OnboardingComplete {
		profile.Status = OnboardingInProgress
	}
	profile.UpdatedAt = s.clock.Now().UTC()
	return s.repository.Save(ctx, profile)
}
func (s *Service) CompleteOnboarding(ctx context.Context, accountID string) (OwnerProfile, error) {
	profile, err := s.GetOwnerProfile(ctx, accountID)
	if err != nil {
		return OwnerProfile{}, err
	}
	if profile.Status == OnboardingComplete {
		return profile, nil
	}
	if err := validateComplete(profile); err != nil {
		return OwnerProfile{}, err
	}
	checkedAt := s.clock.Now()
	if !IsAdult(*profile.DateOfBirth, checkedAt, s.location) {
		return OwnerProfile{}, ErrUnderage
	}
	completedAt := checkedAt.UTC()
	profile.Status = OnboardingComplete
	profile.CompletedAt = &completedAt
	profile.UpdatedAt = completedAt
	return s.repository.Save(ctx, profile)
}
func (s *Service) GetEligibility(ctx context.Context, accountID string) (Eligibility, error) {
	profile, err := s.GetOwnerProfile(ctx, accountID)
	if err != nil {
		return Eligibility{}, err
	}
	now := s.clock.Now()
	adult := profile.DateOfBirth != nil && IsAdult(*profile.DateOfBirth, now, s.location)
	return Eligibility{AccountID: accountID, OnboardingStatus: profile.Status, Adult: adult, CanParticipate: profile.Status == OnboardingComplete && adult, SkillLevel: profile.SkillLevel, SkillConfidence: profile.SkillConfidence, Reliability: profile.Reliability, EligibilityChecked: now.UTC()}, nil
}
func (s *Service) GetPublicProfile(ctx context.Context, accountID string) (PublicProfile, error) {
	profile, err := s.repository.Get(ctx, accountID)
	if err != nil {
		return PublicProfile{}, err
	}
	if profile.Status != OnboardingComplete {
		return PublicProfile{}, ErrNotFound
	}
	return PublicProfile{AccountID: accountID, DisplayName: *profile.DisplayName, SkillLevel: *profile.SkillLevel, Reliability: profile.Reliability, ReliabilityScore: profile.ReliabilityScore, ReliabilitySampleSize: profile.ReliabilitySampleSize, MatchCount: profile.MatchCount}, nil
}

func (s *Service) RecommendationProfile(ctx context.Context, accountID string) (RecommendationProfile, error) {
	p, err := s.GetOwnerProfile(ctx, accountID)
	if err != nil {
		return RecommendationProfile{}, err
	}
	if p.Status != OnboardingComplete || p.SkillLevel == nil || p.RegularArea == nil {
		return RecommendationProfile{}, ErrInvalidProfile
	}
	levels := []SkillLevel{SkillBeginner, SkillBeginnerPlus, SkillIntermediate, SkillIntermediatePlus, SkillAdvanced, SkillCompetitive}
	rank := 0
	for i, level := range levels {
		if level == *p.SkillLevel {
			rank = i + 1
			break
		}
	}
	return RecommendationProfile{AccountID: accountID, SkillLevel: rank, Formats: toStrings(p.PreferredFormats), Styles: toStrings(p.PlayStyles), Periods: toStrings(p.UsualPeriods), Area: *p.RegularArea, Reliability: p.Reliability}, nil
}

func (s *Service) HostReliabilities(ctx context.Context, accountIDs []string) (map[string]ReliabilityLabel, error) {
	type batch interface {
		Reliabilities(context.Context, []string) (map[string]ReliabilityLabel, error)
	}
	repository, ok := s.repository.(batch)
	if !ok {
		return nil, errors.New("players repository does not support recommendation projection")
	}
	return repository.Reliabilities(ctx, accountIDs)
}

func IsAdult(dateOfBirth CalendarDate, at time.Time, location *time.Location) bool {
	if !dateOfBirth.Valid() || location == nil {
		return false
	}
	local := at.In(location)
	// time.Date normalizes 29 February to 1 March in a non-leap year.
	eligibleOn := time.Date(dateOfBirth.Year+18, dateOfBirth.Month, dateOfBirth.Day, 0, 0, 0, 0, location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return !today.Before(eligibleOn)
}
func newProfile(accountID string) OwnerProfile {
	return OwnerProfile{AccountID: accountID, Status: OnboardingNotStarted, SkillConfidence: SkillConfidenceNew, Reliability: ReliabilityNew}
}
func applyDraft(profile *OwnerProfile, draft Draft) {
	profile.DisplayName, profile.AvatarURL, profile.DateOfBirth = cleanString(draft.DisplayName), cleanString(draft.AvatarURL), draft.DateOfBirth
	profile.Gender, profile.Experience, profile.SkillLevel = draft.Gender, draft.Experience, draft.SkillLevel
	profile.PreferredFormats, profile.PlayStyles, profile.UsualPeriods = clone(draft.PreferredFormats), clone(draft.PlayStyles), clone(draft.UsualPeriods)
	profile.RegularArea = cleanString(draft.RegularArea)
}
func validateEntered(profile OwnerProfile) error {
	if profile.DateOfBirth != nil && !profile.DateOfBirth.Valid() {
		return fmt.Errorf("%w: invalid date of birth", ErrInvalidProfile)
	}
	if profile.DisplayName != nil && len(*profile.DisplayName) > 100 || profile.RegularArea != nil && len(*profile.RegularArea) > 120 {
		return fmt.Errorf("%w: profile text is too long", ErrInvalidProfile)
	}
	if !validOptional(profile.Gender, GenderFemale, GenderMale, GenderNonBinary, GenderUndisclosed) || !validOptional(profile.Experience, ExperienceUnder3Months, Experience3To12Months, Experience1To3Years, Experience3PlusYears) || !validOptional(profile.SkillLevel, SkillBeginner, SkillBeginnerPlus, SkillIntermediate, SkillIntermediatePlus, SkillAdvanced, SkillCompetitive) || !validAll(profile.PreferredFormats, FormatSingles, FormatDoubles, FormatMixed) || !validAll(profile.PlayStyles, StyleCasual, StyleSocial, StyleTraining, StyleCompetitive) || !validAll(profile.UsualPeriods, PeriodMorning, PeriodAfternoon, PeriodEvening) {
		return fmt.Errorf("%w: unsupported or duplicate profile option", ErrInvalidProfile)
	}
	return nil
}
func validateComplete(profile OwnerProfile) error {
	if err := validateEntered(profile); err != nil {
		return err
	}
	if profile.DisplayName == nil || profile.DateOfBirth == nil || profile.Experience == nil || profile.SkillLevel == nil || profile.RegularArea == nil || len(profile.PreferredFormats) == 0 || len(profile.PlayStyles) == 0 || len(profile.UsualPeriods) == 0 {
		return fmt.Errorf("%w: required onboarding fields are missing", ErrInvalidProfile)
	}
	return nil
}
func validOptional[T comparable](value *T, allowed ...T) bool {
	if value == nil {
		return true
	}
	for _, item := range allowed {
		if *value == item {
			return true
		}
	}
	return false
}
func validAll[T comparable](values []T, allowed ...T) bool {
	seen := make(map[T]struct{}, len(values))
	for _, value := range values {
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
		if !validOptional(&value, allowed...) {
			return false
		}
	}
	return true
}
func cleanString(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	if cleaned == "" {
		return nil
	}
	return &cleaned
}
func sameDate(left, right *CalendarDate) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}
func clone[T any](values []T) []T { return append([]T(nil), values...) }
