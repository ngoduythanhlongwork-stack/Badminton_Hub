package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"badmintonhub/internal/config"
	"badmintonhub/internal/httpapi"
	"badmintonhub/internal/modules/communication"
	"badmintonhub/internal/modules/identity"
	"badmintonhub/internal/modules/matches"
	"badmintonhub/internal/modules/moderation"
	"badmintonhub/internal/modules/notifications"
	"badmintonhub/internal/modules/payments"
	"badmintonhub/internal/modules/players"
	"badmintonhub/internal/modules/recommendations"
	"badmintonhub/internal/modules/venues"
	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

type r1Composition struct {
	routes            httpapi.R1Routes
	handlers          map[string]outbox.Handler
	email             *identity.CaptureEmailSender
	notificationEmail *notifications.CaptureEmailSender
	maintenance       func(context.Context) error
}

func composeR1(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) (r1Composition, error) {
	playerService, err := players.NewService(players.NewPostgresRepository(pool), clock.System{}, logger)
	if err != nil {
		return r1Composition{}, err
	}
	identityService, err := identity.NewService(identity.Config{Pool: pool, TokenSecret: []byte(cfg.IdentityTokenSecret), Eligibility: organizerEligibility{players: playerService}, Clock: clock.System{}})
	if err != nil {
		return r1Composition{}, err
	}
	venueService, err := venues.NewService(venues.NewPostgresRepository(pool), venueAuthorization{identity: identityService}, nil)
	if err != nil {
		return r1Composition{}, err
	}
	moderationService, err := moderation.NewService(pool, clock.System{})
	if err != nil {
		return r1Composition{}, err
	}
	matchService, err := matches.NewService(matches.NewPostgresRepository(pool), publishEligibility{identity: identityService, players: playerService}, joinEligibility{players: playerService}, venueCatalog{venues: venueService}, nil, moderationService)
	if err != nil {
		return r1Composition{}, err
	}
	paymentService, err := payments.NewService(pool, matchSettlement{matches: matchService}, clock.System{})
	if err != nil {
		return r1Composition{}, err
	}
	notificationEmail := &notifications.CaptureEmailSender{}
	notificationService, err := notifications.NewService(pool, accountDirectory{identity: identityService}, notificationEmail, clock.System{})
	if err != nil {
		return r1Composition{}, err
	}
	trustProcessor, err := players.NewTrustProcessor(pool, clock.System{})
	if err != nil {
		return r1Composition{}, err
	}
	communicationService, err := communication.NewService(pool, roomAccess{matches: matchService}, clock.System{})
	if err != nil {
		return r1Composition{}, err
	}
	recommendationService, err := recommendations.NewService(pool, recommendationProfiles{players: playerService}, recommendationCandidates{matches: matchService}, moderationService, clock.System{})
	if err != nil {
		return r1Composition{}, err
	}
	measurement := recommendations.NewMeasurement(pool, clock.System{})
	email := &identity.CaptureEmailSender{}
	emailHandler, err := identity.NewEmailOutboxHandler(email, []byte(cfg.IdentityTokenSecret))
	if err != nil {
		return r1Composition{}, err
	}
	return r1Composition{
		routes: httpapi.R1Routes{Identity: identityService, Players: playerService, Venues: venueService, Matches: matchService, Payments: paymentService, Notifications: notificationService, Communication: communicationService, Moderation: moderationService, Recommendations: recommendationService, Measurement: measurement},
		handlers: map[string]outbox.Handler{
			"identity.email":              emailHandler,
			"matches.payment-required":    paymentService.PaymentRequiredHandler(),
			"payments.transfer-reported":  combineHandlers(paymentService.TransferReportedHandler(), measurement.OutcomeHandler("TransferReported")),
			"payments.deposit-satisfied":  combineHandlers(paymentService.DepositSatisfiedHandler(), measurement.OutcomeHandler("ReceiptAcknowledged")),
			"matches.joined":              combineHandlers(notificationService.JoinedHandler(), measurement.OutcomeHandler("JoinConfirmed")),
			"payments.notification":       notificationService.PaymentEventHandler(),
			"notifications.deliver-email": notificationService.EmailDeliveryHandler(),
			"matches.cancelled":           combineHandlers(paymentService.CancellationHandler(), notificationService.CancellationHandler(), measurement.OutcomeHandler("ParticipationCancelled")),
			"matches.trust-signal":        combineHandlers(trustProcessor.Handler(), measurement.TrustSignalHandler()),
			"matches.completed":           measurement.OutcomeHandler("MatchCompleted"),
		},
		email:             email,
		notificationEmail: notificationEmail,
		maintenance: func(ctx context.Context) error {
			if _, err := matchService.CompleteDueMatches(ctx, 100); err != nil {
				return fmt.Errorf("complete due matches: %w", err)
			}
			if _, err := matchService.ExpireDueHolds(ctx, 100); err != nil {
				return fmt.Errorf("expire holds: %w", err)
			}
			if _, err := paymentService.MarkOverdue(ctx, 100); err != nil {
				return fmt.Errorf("mark refunds overdue: %w", err)
			}
			if _, err := notificationService.DispatchDue(ctx, 100); err != nil {
				return fmt.Errorf("dispatch notifications: %w", err)
			}
			return nil
		},
	}, nil
}

type roomAccess struct{ matches matches.Service }

func (a roomAccess) RoomAccess(ctx context.Context, accountID, matchID string) (communication.Access, error) {
	v, err := a.matches.RoomAccess(ctx, accountID, matchID)
	return communication.Access{CanRead: v.CanRead, CanSend: v.CanSend, ReadThrough: v.ReadThrough}, err
}

type recommendationProfiles struct{ players *players.Service }

func (a recommendationProfiles) RecommendationProfile(ctx context.Context, accountID string) (recommendations.Profile, error) {
	p, e := a.players.RecommendationProfile(ctx, accountID)
	return recommendations.Profile{AccountID: p.AccountID, SkillLevel: p.SkillLevel, Formats: p.Formats, Styles: p.Styles, Periods: p.Periods, Area: p.Area, Reliability: string(p.Reliability)}, e
}
func (a recommendationProfiles) HostReliabilities(ctx context.Context, accountIDs []string) (map[string]string, error) {
	values, e := a.players.HostReliabilities(ctx, accountIDs)
	if e != nil {
		return nil, e
	}
	result := make(map[string]string, len(values))
	for id, label := range values {
		result[id] = string(label)
	}
	return result, nil
}

type recommendationCandidates struct{ matches matches.Service }

func (a recommendationCandidates) RecommendationCandidates(ctx context.Context, accountID string, limit int) ([]recommendations.Candidate, error) {
	values, e := a.matches.RecommendationCandidates(ctx, accountID, limit)
	if e != nil {
		return nil, e
	}
	result := make([]recommendations.Candidate, len(values))
	for i, v := range values {
		result[i] = recommendations.Candidate{ID: v.ID, HostID: v.HostID, Area: v.Area, TimeZone: v.TimeZone, Format: v.Format, Style: v.Style, JoinMode: string(v.JoinMode), StartAt: v.StartAt, EndAt: v.EndAt, MinLevel: v.MinLevel, MaxLevel: v.MaxLevel}
	}
	return result, nil
}

func combineHandlers(handlers ...outbox.Handler) outbox.Handler {
	return func(ctx context.Context, message outbox.Message) error {
		for _, handler := range handlers {
			if err := handler(ctx, message); err != nil {
				return err
			}
		}
		return nil
	}
}

type matchSettlement struct{ matches matches.Service }

func (adapter matchSettlement) ApplyTransferReport(ctx context.Context, playerID, participationID string, reportedAt time.Time) error {
	_, err := adapter.matches.ApplyTransferReport(ctx, playerID, participationID, reportedAt)
	if errors.Is(err, matches.ErrHoldExpired) {
		return nil
	}
	return err
}

func (adapter matchSettlement) ConfirmPayment(ctx context.Context, hostID, participationID string, confirmedAt time.Time) (bool, error) {
	_, err := adapter.matches.ConfirmPaymentAt(ctx, hostID, participationID, confirmedAt)
	if errors.Is(err, matches.ErrHoldExpired) || errors.Is(err, matches.ErrFull) || errors.Is(err, matches.ErrScheduleConflict) || errors.Is(err, matches.ErrNotOpen) {
		return false, nil
	}
	return err == nil, err
}

type accountDirectory struct{ identity *identity.Service }

func (adapter accountDirectory) EmailForAccount(ctx context.Context, accountID string) (string, error) {
	account, err := adapter.identity.Account(ctx, accountID)
	if err != nil {
		return "", err
	}
	return account.Email, nil
}

type organizerEligibility struct{ players *players.Service }

func (adapter organizerEligibility) OrganizerEligibility(ctx context.Context, accountID string) (identity.Eligibility, error) {
	result, err := adapter.players.GetEligibility(ctx, accountID)
	if err != nil {
		return identity.Eligibility{}, err
	}
	return identity.Eligibility{ProfileComplete: result.OnboardingStatus == players.OnboardingComplete, AdultEligible: result.Adult}, nil
}

type venueAuthorization struct{ identity *identity.Service }

func (adapter venueAuthorization) VenueManagementScope(ctx context.Context, accountID string) (venues.ManagementScope, error) {
	allowed, err := adapter.identity.HasPermission(ctx, identity.PermissionQuery{AccountID: accountID, Permission: identity.PermissionVenueManage, ScopeType: "GLOBAL"})
	if err != nil {
		return "", err
	}
	if allowed {
		return venues.ScopeAllVenues, nil
	}
	return venues.ScopeAssignedVenues, nil
}

type publishEligibility struct {
	identity *identity.Service
	players  *players.Service
}

func (adapter publishEligibility) CanPublishMatch(ctx context.Context, accountID string) (bool, error) {
	profile, err := adapter.players.GetEligibility(ctx, accountID)
	if err != nil {
		return false, err
	}
	if !profile.CanParticipate {
		return false, nil
	}
	return adapter.identity.HasPermission(ctx, identity.PermissionQuery{AccountID: accountID, Permission: identity.PermissionOrganizerPublish, ScopeType: "OWN"})
}

type joinEligibility struct{ players *players.Service }

func (adapter joinEligibility) PlayerEligibility(ctx context.Context, accountID string) (matches.PlayerEligibility, error) {
	profile, err := adapter.players.GetEligibility(ctx, accountID)
	if err != nil {
		return matches.PlayerEligibility{}, err
	}
	level := 0
	if profile.SkillLevel != nil {
		level = skillRank(*profile.SkillLevel)
	}
	return matches.PlayerEligibility{Eligible: profile.CanParticipate && level > 0, Level: level}, nil
}

func skillRank(level players.SkillLevel) int {
	levels := []players.SkillLevel{players.SkillBeginner, players.SkillBeginnerPlus, players.SkillIntermediate, players.SkillIntermediatePlus, players.SkillAdvanced, players.SkillCompetitive}
	index := slices.Index(levels, level)
	if index < 0 {
		return 0
	}
	return index + 1
}

type venueCatalog struct{ venues venues.Service }

func (adapter venueCatalog) PublishedVenue(ctx context.Context, venueID, court string) (matches.VenueSnapshot, error) {
	venue, err := adapter.venues.Detail(ctx, venueID)
	if err != nil {
		return matches.VenueSnapshot{}, err
	}
	if court != "" && !slices.Contains(venue.Courts, court) {
		return matches.VenueSnapshot{}, fmt.Errorf("%w: court does not belong to published venue", matches.ErrInvalid)
	}
	return matches.VenueSnapshot{ID: venue.ID, Name: venue.Name, Address: venue.Address, Area: venue.Area, TimeZone: venue.TimeZone, Court: court}, nil
}
