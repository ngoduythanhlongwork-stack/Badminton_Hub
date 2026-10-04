package identity

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"badmintonhub/internal/modules/identity/internal/application"
	"badmintonhub/internal/modules/identity/internal/argon2id"
	identitypostgres "badmintonhub/internal/modules/identity/internal/postgres"
	"badmintonhub/internal/modules/identity/internal/tokenhmac"
	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

type (
	Service                = application.Service
	Account                = application.Account
	AccountState           = application.AccountState
	Tokens                 = application.Tokens
	EmailMessage           = application.EmailMessage
	EmailSender            = application.EmailSender
	PasswordHasher         = application.PasswordHasher
	Eligibility            = application.Eligibility
	OrganizerEligibility   = application.OrganizerEligibility
	Permission             = application.Permission
	PermissionQuery        = application.PermissionQuery
	PermissionGrant        = application.PermissionGrant
	OrganizerApplication   = application.OrganizerApplication
	RegisterRequest        = application.RegisterRequest
	LoginRequest           = application.LoginRequest
	OrganizerRequest       = application.OrganizerRequest
	ReviewOrganizerRequest = application.ReviewOrganizerRequest
	RevokeOrganizerRequest = application.RevokeOrganizerRequest
	BootstrapAdminRequest  = application.BootstrapAdminRequest
)

const (
	AccountUnverified          = application.AccountUnverified
	AccountActive              = application.AccountActive
	AccountRestricted          = application.AccountRestricted
	AccountSuspended           = application.AccountSuspended
	PermissionOrganizerPublish = application.PermissionOrganizerPublish
	PermissionOrganizerReview  = application.PermissionOrganizerReview
	PermissionAccountManage    = application.PermissionAccountManage
	PermissionAssignmentsGrant = application.PermissionAssignmentsGrant
	PermissionVenueManage      = application.PermissionVenueManage
)

var (
	ErrInvalidInput       = application.ErrInvalidInput
	ErrEmailExists        = application.ErrEmailExists
	ErrInvalidToken       = application.ErrInvalidToken
	ErrInvalidCredentials = application.ErrInvalidCredentials
	ErrRateLimited        = application.ErrRateLimited
	ErrAccountUnavailable = application.ErrAccountUnavailable
	ErrForbidden          = application.ErrForbidden
	ErrConflict           = application.ErrConflict
)

type Config struct {
	Pool        *pgxpool.Pool
	Hasher      PasswordHasher
	Email       EmailSender
	TokenSecret []byte
	Eligibility OrganizerEligibility
	Clock       clock.Clock
}

func NewService(config Config) (*Service, error) {
	store, err := identitypostgres.New(config.Pool)
	if err != nil {
		return nil, err
	}
	hasher := config.Hasher
	if hasher == nil {
		defaultHasher := argon2id.Default()
		hasher = defaultHasher
	}
	deriver, err := tokenhmac.New(config.TokenSecret)
	if err != nil {
		return nil, err
	}
	return application.New(store, hasher, deriver, config.Eligibility, config.Clock)
}

// NewEmailOutboxHandler delivers the durable identity.email obligations created
// in the same transaction as their action token. The worker supplies retries.
func NewEmailOutboxHandler(sender EmailSender, tokenSecret []byte) (outbox.Handler, error) {
	if sender == nil {
		return nil, errors.New("identity email sender is required")
	}
	deriver, err := tokenhmac.New(tokenSecret)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, message outbox.Message) error {
		var obligation application.EmailObligation
		if err := json.Unmarshal(message.Payload, &obligation); err != nil {
			return errors.New("decode identity email obligation")
		}
		token, _, err := deriver.Derive(obligation.TokenID, obligation.AccountID, obligation.Purpose)
		if err != nil {
			return errors.New("derive identity email token")
		}
		email := EmailMessage{Kind: obligation.Kind, Recipient: obligation.Recipient, Token: token, ExpiresAt: obligation.ExpiresAt, IdempotencyKey: message.ID}
		return sender.Send(ctx, email)
	}, nil
}

func CanonicalEmail(value string) (string, error) { return application.CanonicalEmail(value) }
func NewArgon2idHasher() PasswordHasher           { return argon2id.Default() }

// CaptureEmailSender is a concurrency-safe local development/test delivery adapter.
// It never logs message contents; callers explicitly inspect Messages when appropriate.
type CaptureEmailSender struct {
	mu        sync.Mutex
	messages  []EmailMessage
	delivered map[string]struct{}
}

func (c *CaptureEmailSender) Send(_ context.Context, message EmailMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if message.IdempotencyKey != "" {
		if c.delivered == nil {
			c.delivered = make(map[string]struct{})
		}
		if _, exists := c.delivered[message.IdempotencyKey]; exists {
			return nil
		}
		c.delivered[message.IdempotencyKey] = struct{}{}
	}
	c.messages = append(c.messages, message)
	return nil
}
func (c *CaptureEmailSender) Messages() []EmailMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]EmailMessage, len(c.messages))
	copy(result, c.messages)
	return result
}
func (c *CaptureEmailSender) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = nil
	c.delivered = nil
}
