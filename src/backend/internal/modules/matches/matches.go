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
	StatusDraft      Status = "DRAFT"
	StatusOpen       Status = "OPEN"
	StatusFull       Status = "FULL"
	StatusInProgress Status = "IN_PROGRESS"
	StatusCompleted  Status = "COMPLETED"
	StatusCancelled  Status = "CANCELLED"
)

type JoinMode string

const (
	JoinInstant  JoinMode = "INSTANT"
	JoinApproval JoinMode = "APPROVAL_REQUIRED"
)

type ParticipationStatus string

const (
	ParticipationRequested       ParticipationStatus = "REQUESTED"
	ParticipationAwaitingPayment ParticipationStatus = "AWAITING_PAYMENT"
	ParticipationJoined          ParticipationStatus = "JOINED"
	ParticipationCheckedIn       ParticipationStatus = "CHECKED_IN"
	ParticipationCompleted       ParticipationStatus = "COMPLETED"
	ParticipationNoShow          ParticipationStatus = "NO_SHOW"
	ParticipationUnknown         ParticipationStatus = "UNKNOWN"
	ParticipationRejected        ParticipationStatus = "REJECTED"
	ParticipationRemoved         ParticipationStatus = "REMOVED"
	ParticipationCancelled       ParticipationStatus = "CANCELLED"
	ParticipationExpired         ParticipationStatus = "EXPIRED"
)

type HoldStatus string

const (
	HoldHeld     HoldStatus = "HELD"
	HoldConsumed HoldStatus = "CONSUMED"
	HoldExpired  HoldStatus = "EXPIRED"
	HoldReleased HoldStatus = "RELEASED"
)

type VenueSnapshot struct{ ID, Name, Address, Area, TimeZone, Court string }
type Match struct {
	ID, HostID, Title, Description, Format, Style, Rules string
	Venue                                                VenueSnapshot
	StartAt, EndAt                                       time.Time
	MinLevel, MaxLevel, Capacity                         int
	FeeMinor                                             int64
	DepositMinor                                         int64
	Currency                                             string
	PaymentRecipient, PaymentInstructions                string
	PaymentInstructionVersion                            int
	CancellationPolicyVersion                            string
	JoinMode                                             JoinMode
	HostPlays, CourtAttested                             bool
	CourtAttestedAt                                      time.Time
	Status                                               Status
	Occupied                                             int
	CreatedAt, UpdatedAt                                 time.Time
	CompletedAt                                          *time.Time
}
type Draft struct {
	Title, Description, Format, Style, Rules, VenueID, Court string
	StartAt, EndAt                                           time.Time
	MinLevel, MaxLevel, Capacity                             int
	FeeMinor                                                 int64
	DepositMinor                                             int64
	Currency                                                 string
	PaymentRecipient, PaymentInstructions                    string
	PaymentInstructionVersion                                int
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
	Hold                  *Hold
	AttendanceRevision    int
}

type Review struct {
	ID             string    `json:"id"`
	MatchID        string    `json:"matchId"`
	ReviewerID     string    `json:"reviewerId"`
	TargetPlayerID string    `json:"targetPlayerId"`
	MatchQuality   int       `json:"matchQuality"`
	HostRating     int       `json:"hostRating"`
	Revision       int       `json:"revision"`
	Tags           []string  `json:"tags"`
	SkillFeedback  string    `json:"skillFeedback"`
	SubmittedAt    time.Time `json:"submittedAt"`
}

type TrustSignalEvent struct {
	EventID, AccountID, SourceType, SourceID, MatchID, SkillDirection string
	SourceRevision                                                    int
	ReliabilityValue                                                  *float64
	OccurredAt                                                        time.Time
}

type Hold struct {
	ID, ParticipationID string
	Status              HoldStatus
	ExpiresAt           time.Time
	TransferReportedAt  *time.Time
}

type PaymentRequiredEvent struct {
	EventID, ParticipationID, MatchID, PayerID, PayeeID            string
	FeeMinor, DepositMinor                                         int64
	Currency, PaymentRecipient, PaymentInstructions, PolicyVersion string
	InstructionVersion                                             int
	HoldExpiresAt, MatchStartAt, OccurredAt                        time.Time
}

type CancellationEvent struct {
	EventID, MatchID, ParticipationID, PlayerID, HostID string
	Cause, Reason, RefundOutcome, PolicyVersion         string
	OccurredAt                                          time.Time
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
type InteractionPolicy interface {
	Blocked(context.Context, string, string) (bool, error)
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
	ErrForbidden             = errors.New("match action forbidden")
	ErrInvalid               = errors.New("invalid match")
	ErrNotFound              = errors.New("match not found")
	ErrNotOpen               = errors.New("match is not open")
	ErrFull                  = errors.New("match is full")
	ErrScheduleConflict      = errors.New("player schedule conflict")
	ErrAlreadyParticipating  = errors.New("player already participates")
	ErrHoldExpired           = errors.New("payment hold expired")
	ErrCapacityBelowOccupied = errors.New("capacity is below occupied slots")
	ErrIdempotencyConflict   = errors.New("idempotency key reused for another command")
	ErrAttendanceWindow      = errors.New("attendance window is closed")
	ErrReviewWindow          = errors.New("review window is closed")
	ErrReviewNotAllowed      = errors.New("review is not allowed")
)

type R2Repository interface {
	ExtendHold(context.Context, string, string, time.Time) (Participation, error)
	ConfirmPayment(context.Context, string, string, time.Time) (Participation, error)
	ExpireDueHolds(context.Context, time.Time, int) (int, error)
	CancelParticipation(context.Context, string, string, string, string, time.Time) (Participation, error)
	CancelMatch(context.Context, string, string, string, string, time.Time) (Match, error)
	UpdatePublished(context.Context, string, string, string, string, int, time.Time) (Match, error)
}

type R3Repository interface {
	RecordAttendance(context.Context, string, string, string, ParticipationStatus, string, time.Time) (Participation, error)
	CompleteDueMatches(context.Context, time.Time, int) (int, error)
	SubmitReview(context.Context, Review, string, time.Time) (Review, error)
	CorrectAttendance(context.Context, string, string, string, string, ParticipationStatus, string, time.Time) (Participation, error)
	RoomAccess(context.Context, string, string, time.Time) (RoomAccess, error)
	RecommendationCandidates(context.Context, string, time.Time, int) ([]RecommendationCandidate, error)
}

type RoomAccess struct {
	CanRead, CanSend bool
	ReadThrough      *time.Time
}

type RecommendationCandidate struct {
	ID, HostID, Area, TimeZone, Format, Style string
	StartAt, EndAt                            time.Time
	MinLevel, MaxLevel                        int
	JoinMode                                  JoinMode
}

type Service struct {
	repo               Repository
	publishEligibility PublishEligibility
	joinEligibility    JoinEligibility
	venues             VenueCatalog
	interaction        InteractionPolicy
	now                func() time.Time
}

func NewService(r Repository, p PublishEligibility, j JoinEligibility, v VenueCatalog, now func() time.Time, policies ...InteractionPolicy) (Service, error) {
	if r == nil || p == nil || j == nil || v == nil {
		return Service{}, fmt.Errorf("%w: repository and contracts are required", ErrInvalid)
	}
	if now == nil {
		now = time.Now
	}
	var interaction InteractionPolicy
	if len(policies) > 0 {
		interaction = policies[0]
	}
	return Service{repo: r, publishEligibility: p, joinEligibility: j, venues: v, now: now, interaction: interaction}, nil
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
	if s.interaction != nil {
		blocked, policyErr := s.interaction.Blocked(ctx, player, m.HostID)
		if policyErr != nil {
			return Participation{}, policyErr
		}
		if blocked {
			return Participation{}, ErrForbidden
		}
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

func (s Service) ReportTransfer(ctx context.Context, player, participationID string) (Participation, error) {
	return s.ApplyTransferReport(ctx, player, participationID, s.now().UTC())
}

func (s Service) ApplyTransferReport(ctx context.Context, player, participationID string, reportedAt time.Time) (Participation, error) {
	repo, ok := s.repo.(R2Repository)
	if !ok {
		return Participation{}, ErrInvalid
	}
	return repo.ExtendHold(ctx, player, participationID, reportedAt.UTC())
}

func (s Service) ConfirmPayment(ctx context.Context, host, participationID string) (Participation, error) {
	return s.ConfirmPaymentAt(ctx, host, participationID, s.now().UTC())
}

func (s Service) ConfirmPaymentAt(ctx context.Context, host, participationID string, confirmedAt time.Time) (Participation, error) {
	repo, ok := s.repo.(R2Repository)
	if !ok {
		return Participation{}, ErrInvalid
	}
	return repo.ConfirmPayment(ctx, host, participationID, confirmedAt.UTC())
}

func (s Service) ExpireDueHolds(ctx context.Context, limit int) (int, error) {
	repo, ok := s.repo.(R2Repository)
	if !ok {
		return 0, ErrInvalid
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return repo.ExpireDueHolds(ctx, s.now().UTC(), limit)
}

func (s Service) CancelParticipation(ctx context.Context, player, participationID, key, reason string) (Participation, error) {
	if key == "" || strings.TrimSpace(reason) == "" {
		return Participation{}, ErrInvalid
	}
	repo, ok := s.repo.(R2Repository)
	if !ok {
		return Participation{}, ErrInvalid
	}
	return repo.CancelParticipation(ctx, player, participationID, key, strings.TrimSpace(reason), s.now().UTC())
}

func (s Service) CancelMatch(ctx context.Context, host, matchID, key, reason string) (Match, error) {
	if key == "" || strings.TrimSpace(reason) == "" {
		return Match{}, ErrInvalid
	}
	repo, ok := s.repo.(R2Repository)
	if !ok {
		return Match{}, ErrInvalid
	}
	return repo.CancelMatch(ctx, host, matchID, key, strings.TrimSpace(reason), s.now().UTC())
}

func (s Service) UpdatePublished(ctx context.Context, host, matchID, key, description string, capacity int) (Match, error) {
	description = strings.TrimSpace(description)
	if key == "" || description == "" || capacity <= 0 {
		return Match{}, ErrInvalid
	}
	repo, ok := s.repo.(R2Repository)
	if !ok {
		return Match{}, ErrInvalid
	}
	return repo.UpdatePublished(ctx, host, matchID, key, description, capacity, s.now().UTC())
}

func (s Service) RecordAttendance(ctx context.Context, host, matchID, participationID, key string, status ParticipationStatus, reason string) (Participation, error) {
	if key == "" || (status != ParticipationCheckedIn && status != ParticipationNoShow) {
		return Participation{}, ErrInvalid
	}
	repo, ok := s.repo.(R3Repository)
	if !ok {
		return Participation{}, ErrInvalid
	}
	part, err := s.repo.Participation(ctx, participationID)
	if err != nil {
		return Participation{}, err
	}
	if part.MatchID != matchID {
		return Participation{}, ErrNotFound
	}
	return repo.RecordAttendance(ctx, host, participationID, key, status, strings.TrimSpace(reason), s.now().UTC())
}

func (s Service) CompleteDueMatches(ctx context.Context, limit int) (int, error) {
	repo, ok := s.repo.(R3Repository)
	if !ok {
		return 0, ErrInvalid
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return repo.CompleteDueMatches(ctx, s.now().UTC(), limit)
}

func (s Service) SubmitReview(ctx context.Context, reviewer, matchID, targetPlayerID, key string, quality, hostRating int, tags []string, feedback string) (Review, error) {
	if key == "" || reviewer == targetPlayerID || quality < 1 || quality > 5 || hostRating < 1 || hostRating > 5 || !allowed(feedback, "AS_EXPECTED", "STRONGER_THAN_PROFILE", "LOWER_THAN_PROFILE") {
		return Review{}, ErrInvalid
	}
	for _, tag := range tags {
		if !allowed(tag, "FRIENDLY", "FAIR", "COMPETITIVE", "ON_TIME", "GOOD_SKILL_MATCH") {
			return Review{}, ErrInvalid
		}
	}
	repo, ok := s.repo.(R3Repository)
	if !ok {
		return Review{}, ErrInvalid
	}
	return repo.SubmitReview(ctx, Review{MatchID: matchID, ReviewerID: reviewer, TargetPlayerID: targetPlayerID, MatchQuality: quality, HostRating: hostRating, Tags: tags, SkillFeedback: feedback}, key, s.now().UTC())
}

func (s Service) CorrectAttendance(ctx context.Context, admin, participationID, caseID, key string, status ParticipationStatus, reason string) (Participation, error) {
	if admin == "" || caseID == "" || key == "" || strings.TrimSpace(reason) == "" || (status != ParticipationCheckedIn && status != ParticipationNoShow && status != ParticipationUnknown) {
		return Participation{}, ErrInvalid
	}
	repo, ok := s.repo.(R3Repository)
	if !ok {
		return Participation{}, ErrInvalid
	}
	return repo.CorrectAttendance(ctx, admin, participationID, caseID, key, status, strings.TrimSpace(reason), s.now().UTC())
}

func (s Service) RoomAccess(ctx context.Context, accountID, matchID string) (RoomAccess, error) {
	repo, ok := s.repo.(R3Repository)
	if !ok {
		return RoomAccess{}, ErrInvalid
	}
	return repo.RoomAccess(ctx, accountID, matchID, s.now().UTC())
}

func (s Service) RecommendationCandidates(ctx context.Context, accountID string, limit int) ([]RecommendationCandidate, error) {
	repo, ok := s.repo.(R3Repository)
	if !ok {
		return nil, ErrInvalid
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	return repo.RecommendationCandidates(ctx, accountID, s.now().UTC(), limit)
}

func (s Service) fromDraft(ctx context.Context, host string, d Draft) (Match, error) {
	if s.venues == nil {
		return Match{}, ErrInvalid
	}
	v, e := s.venues.PublishedVenue(ctx, d.VenueID, d.Court)
	if e != nil {
		return Match{}, e
	}
	m := Match{HostID: host, Title: strings.TrimSpace(d.Title), Description: strings.TrimSpace(d.Description), Format: d.Format, Style: d.Style, Rules: d.Rules, Venue: v, StartAt: d.StartAt.UTC(), EndAt: d.EndAt.UTC(), MinLevel: d.MinLevel, MaxLevel: d.MaxLevel, Capacity: d.Capacity, FeeMinor: d.FeeMinor, DepositMinor: d.DepositMinor, Currency: d.Currency, PaymentRecipient: strings.TrimSpace(d.PaymentRecipient), PaymentInstructions: strings.TrimSpace(d.PaymentInstructions), PaymentInstructionVersion: d.PaymentInstructionVersion, CancellationPolicyVersion: "MVP-2026-10-D15", JoinMode: d.JoinMode, HostPlays: d.HostPlays, CourtAttested: d.CourtAttested}
	if m.PaymentInstructionVersion <= 0 {
		m.PaymentInstructionVersion = 1
	}
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
	if m.FeeMinor < 0 || m.DepositMinor < 0 || m.DepositMinor > m.FeeMinor || m.Currency != "VND" {
		return fmt.Errorf("%w: VND fee/deposit is invalid", ErrInvalid)
	}
	if m.DepositMinor > 0 && (m.PaymentRecipient == "" || m.PaymentInstructions == "") {
		return fmt.Errorf("%w: payment instruction is required", ErrInvalid)
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
