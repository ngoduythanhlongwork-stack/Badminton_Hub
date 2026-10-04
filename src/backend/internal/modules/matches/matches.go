package matches

import (
	"badmintonhub/internal/platform/id"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusDraft Status = "DRAFT"
	StatusOpen  Status = "OPEN"
	StatusFull  Status = "FULL"
)

type JoinMode string

const (
	JoinInstant  JoinMode = "INSTANT"
	JoinApproval JoinMode = "APPROVAL_REQUIRED"
)

type ParticipationStatus string

const (
	ParticipationRequested ParticipationStatus = "REQUESTED"
	ParticipationJoined    ParticipationStatus = "JOINED"
	ParticipationRejected  ParticipationStatus = "REJECTED"
	ParticipationRemoved   ParticipationStatus = "REMOVED"
)

type VenueSnapshot struct{ ID, Name, Address, Area, TimeZone, Court string }
type Match struct {
	ID, HostID, Title, Description, Format, Style, Rules string
	Venue                                                VenueSnapshot
	StartAt, EndAt                                       time.Time
	MinLevel, MaxLevel, Capacity                         int
	FeeMinor                                             int64
	Currency                                             string
	JoinMode                                             JoinMode
	HostPlays, CourtAttested                             bool
	CourtAttestedAt                                      time.Time
	Status                                               Status
	Occupied                                             int
	CreatedAt, UpdatedAt                                 time.Time
}
type Draft struct {
	Title, Description, Format, Style, Rules, VenueID, Court string
	StartAt, EndAt                                           time.Time
	MinLevel, MaxLevel, Capacity                             int
	FeeMinor                                                 int64
	Currency                                                 string
	JoinMode                                                 JoinMode
	HostPlays, CourtAttested                                 bool
}
type Filters struct {
	From, To           *time.Time
	MinLevel, MaxLevel *int
	Area, Format       string
	MaxCostMinor       *int64
	Limit              int
	Cursor             string
}
type Page struct {
	Matches    []Match
	NextCursor string
}
type Participation struct {
	ID, MatchID, PlayerID string
	Status                ParticipationStatus
	DecisionReason        string
	CreatedAt, UpdatedAt  time.Time
}
type PublishEligibility interface {
	CanPublishMatch(context.Context, string) (bool, error)
}
type PlayerEligibility struct {
	Eligible bool
	Level    int
}
type JoinEligibility interface {
	PlayerEligibility(context.Context, string) (PlayerEligibility, error)
}
type VenueCatalog interface {
	PublishedVenue(context.Context, string, string) (VenueSnapshot, error)
}
type Repository interface {
	Create(context.Context, Match) (Match, error)
	UpdateDraft(context.Context, Match) (Match, error)
	OwnedDraft(context.Context, string, string) (Match, error)
	Publish(context.Context, string, string, time.Time) (Match, error)
	PublicList(context.Context, Filters) (Page, error)
	PublicDetail(context.Context, string) (Match, error)
	Participation(context.Context, string) (Participation, error)
	Join(context.Context, string, string, string, time.Time) (Participation, error)
	Decide(context.Context, string, string, string, string, ParticipationStatus, string, time.Time) (Participation, error)
}

var (
	ErrForbidden            = errors.New("match action forbidden")
	ErrInvalid              = errors.New("invalid match")
	ErrNotFound             = errors.New("match not found")
	ErrNotOpen              = errors.New("match is not open")
	ErrFull                 = errors.New("match is full")
	ErrScheduleConflict     = errors.New("player schedule conflict")
	ErrAlreadyParticipating = errors.New("player already participates")
	ErrPaidUnsupported      = errors.New("paid matches are not supported in R1")
	ErrIdempotencyConflict  = errors.New("idempotency key reused for another command")
)

type Service struct {
	repo               Repository
	publishEligibility PublishEligibility
	joinEligibility    JoinEligibility
	venues             VenueCatalog
	now                func() time.Time
}

func NewService(r Repository, p PublishEligibility, j JoinEligibility, v VenueCatalog, now func() time.Time) (Service, error) {
	if r == nil || p == nil || j == nil || v == nil {
		return Service{}, fmt.Errorf("%w: repository and contracts are required", ErrInvalid)
	}
	if now == nil {
		now = time.Now
	}
	return Service{r, p, j, v, now}, nil
}
func (s Service) Create(ctx context.Context, host string, d Draft) (Match, error) {
	m, e := s.fromDraft(ctx, host, d)
	if e != nil {
		return Match{}, e
	}
	m.ID, _ = id.New()
	m.Status = StatusDraft
	m.CreatedAt = s.now().UTC()
	m.UpdatedAt = m.CreatedAt
	return s.repo.Create(ctx, m)
}
func (s Service) UpdateDraft(ctx context.Context, host, mid string, d Draft) (Match, error) {
	m, e := s.fromDraft(ctx, host, d)
	if e != nil {
		return Match{}, e
	}
	m.ID = mid
	m.UpdatedAt = s.now().UTC()
	return s.repo.UpdateDraft(ctx, m)
}
func (s Service) Publish(ctx context.Context, host, mid string) (Match, error) {
	if s.publishEligibility == nil {
		return Match{}, ErrForbidden
	}
	ok, e := s.publishEligibility.CanPublishMatch(ctx, host)
	if e != nil {
		return Match{}, e
	}
	if !ok {
		return Match{}, ErrForbidden
	}
	now := s.now().UTC()
	draft, e := s.repo.OwnedDraft(ctx, host, mid)
	if e != nil {
		return Match{}, e
	}
	venue, e := s.venues.PublishedVenue(ctx, draft.Venue.ID, draft.Venue.Court)
	if e != nil {
		return Match{}, e
	}
	draft.Venue = venue
	if e = validate(draft, now); e != nil {
		return Match{}, e
	}
	draft.UpdatedAt = now
	if _, e = s.repo.UpdateDraft(ctx, draft); e != nil {
		return Match{}, e
	}
	return s.repo.Publish(ctx, host, mid, now)
}
func (s Service) List(ctx context.Context, f Filters) (Page, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	return s.repo.PublicList(ctx, f)
}
func (s Service) Detail(ctx context.Context, mid string) (Match, error) {
	return s.repo.PublicDetail(ctx, mid)
}
func (s Service) Join(ctx context.Context, player, mid, key string) (Participation, error) {
	if key == "" {
		return Participation{}, fmt.Errorf("%w: idempotency key required", ErrInvalid)
	}
	pe, e := s.joinEligibility.PlayerEligibility(ctx, player)
	if e != nil {
		return Participation{}, e
	}
	if !pe.Eligible {
		return Participation{}, ErrForbidden
	}
	m, e := s.repo.PublicDetail(ctx, mid)
	if e != nil {
		return Participation{}, e
	}
	if (pe.Level < m.MinLevel || pe.Level > m.MaxLevel) && m.JoinMode == JoinInstant {
		return Participation{}, ErrForbidden
	}
	return s.repo.Join(ctx, player, mid, key, s.now().UTC())
}
func (s Service) Approve(ctx context.Context, h, m, p, k string) (Participation, error) {
	if k == "" {
		return Participation{}, ErrInvalid
	}
	part, e := s.repo.Participation(ctx, p)
	if e != nil {
		return Participation{}, e
	}
	if part.MatchID != m {
		return Participation{}, ErrNotFound
	}
	pe, e := s.joinEligibility.PlayerEligibility(ctx, part.PlayerID)
	if e != nil {
		return Participation{}, e
	}
	if !pe.Eligible {
		return Participation{}, ErrForbidden
	}
	return s.repo.Decide(ctx, h, m, p, k, ParticipationJoined, "", s.now().UTC())
}
func (s Service) Reject(ctx context.Context, h, m, p, k, reason string) (Participation, error) {
	if k == "" || strings.TrimSpace(reason) == "" {
		return Participation{}, ErrInvalid
	}
	return s.repo.Decide(ctx, h, m, p, k, ParticipationRejected, reason, s.now().UTC())
}
func (s Service) Remove(ctx context.Context, h, m, p, k, reason string) (Participation, error) {
	if k == "" || strings.TrimSpace(reason) == "" {
		return Participation{}, ErrInvalid
	}
	return s.repo.Decide(ctx, h, m, p, k, ParticipationRemoved, reason, s.now().UTC())
}
func (s Service) fromDraft(ctx context.Context, host string, d Draft) (Match, error) {
	if s.venues == nil {
		return Match{}, ErrInvalid
	}
	v, e := s.venues.PublishedVenue(ctx, d.VenueID, d.Court)
	if e != nil {
		return Match{}, e
	}
	m := Match{HostID: host, Title: strings.TrimSpace(d.Title), Description: strings.TrimSpace(d.Description), Format: d.Format, Style: d.Style, Rules: d.Rules, Venue: v, StartAt: d.StartAt.UTC(), EndAt: d.EndAt.UTC(), MinLevel: d.MinLevel, MaxLevel: d.MaxLevel, Capacity: d.Capacity, FeeMinor: d.FeeMinor, Currency: d.Currency, JoinMode: d.JoinMode, HostPlays: d.HostPlays, CourtAttested: d.CourtAttested}
	if d.CourtAttested {
		m.CourtAttestedAt = s.now().UTC()
	}
	if e = validate(m, s.now().UTC()); e != nil {
		return Match{}, e
	}
	return m, nil
}
func validate(m Match, now time.Time) error {
	if m.Title == "" || m.Description == "" || m.Format == "" || m.Style == "" || m.Rules == "" || m.Venue.ID == "" || m.Venue.TimeZone == "" {
		return fmt.Errorf("%w: required data missing", ErrInvalid)
	}
	if _, e := time.LoadLocation(m.Venue.TimeZone); e != nil {
		return fmt.Errorf("%w: invalid timezone", ErrInvalid)
	}
	if !m.StartAt.After(now) || !m.EndAt.After(m.StartAt) || m.Capacity <= 0 || m.MinLevel < 1 || m.MaxLevel > 6 || m.MinLevel > m.MaxLevel {
		return ErrInvalid
	}
	if !allowed(m.Format, "SINGLES", "DOUBLES", "MIXED") || !allowed(m.Style, "CASUAL", "SOCIAL", "TRAINING", "COMPETITIVE") {
		return ErrInvalid
	}
	if m.FeeMinor != 0 {
		return ErrPaidUnsupported
	}
	if len(m.Currency) != 3 {
		return fmt.Errorf("%w: ISO currency is required", ErrInvalid)
	}
	if m.JoinMode != JoinInstant && m.JoinMode != JoinApproval {
		return ErrInvalid
	}
	if !m.CourtAttested {
		return fmt.Errorf("%w: court attestation required", ErrInvalid)
	}
	return nil
}
func allowed(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
