package application

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidInput       = errors.New("identity: invalid input")
	ErrEmailExists        = errors.New("identity: email already registered")
	ErrInvalidToken       = errors.New("identity: token is invalid or expired")
	ErrInvalidCredentials = errors.New("identity: invalid credentials")
	ErrRateLimited        = errors.New("identity: rate limited")
	ErrAccountUnavailable = errors.New("identity: account unavailable")
	ErrForbidden          = errors.New("identity: forbidden")
	ErrConflict           = errors.New("identity: conflict")
)

type AccountState string

const (
	AccountUnverified AccountState = "UNVERIFIED"
	AccountActive     AccountState = "ACTIVE"
	AccountRestricted AccountState = "RESTRICTED"
	AccountSuspended  AccountState = "SUSPENDED"
)

type Account struct {
	ID              string
	Email           string
	State           AccountState
	EmailVerifiedAt *time.Time
}

type Tokens struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type EmailMessage struct {
	Kind           string
	Recipient      string
	Token          string
	ExpiresAt      time.Time
	IdempotencyKey string
}

type EmailObligation struct {
	Kind, Recipient, TokenID, AccountID, Purpose string
	ExpiresAt                                    time.Time
}

type ActionTokenDeriver interface {
	Derive(string, string, string) (string, []byte, error)
}

type EmailSender interface {
	Send(context.Context, EmailMessage) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) (bool, error)
}

type Eligibility struct {
	ProfileComplete bool
	AdultEligible   bool
}

type OrganizerEligibility interface {
	OrganizerEligibility(context.Context, string) (Eligibility, error)
}

type Permission string

const (
	PermissionOrganizerPublish Permission = "matches.publish.own"
	PermissionOrganizerReview  Permission = "identity.organizer.review"
	PermissionAccountManage    Permission = "identity.account.manage"
	PermissionAssignmentsGrant Permission = "identity.permission.grant"
	PermissionVenueManage      Permission = "venues.manage.global"
)

type PermissionQuery struct {
	AccountID  string
	Permission Permission
	ScopeType  string
	ScopeID    string
}

type OrganizerApplication struct {
	ID              string
	AccountID       string
	Reason          string
	Contact         string
	Status          string
	SubmittedAt     time.Time
	ReviewedBy      string
	ReviewedAt      *time.Time
	ReviewRationale string
}

type RegisterRequest struct{ Email, Password, SourceKey string }
type LoginRequest struct{ Email, Password, SourceKey string }
type OrganizerRequest struct{ AccountID, Reason, Contact string }
type ReviewOrganizerRequest struct{ ApplicationID, AdminID, Decision, Rationale string }
type RevokeOrganizerRequest struct{ AccountID, AdminID, Rationale string }
type BootstrapAdminRequest struct{ AccountID, Rationale string }

type Store interface {
	CreateAccount(context.Context, Account, string, TokenRecord, EmailObligation) error
	AccountByCanonicalEmail(context.Context, string) (Account, string, error)
	AccountByID(context.Context, string) (Account, error)
	ReplaceActionToken(context.Context, TokenRecord, EmailObligation) error
	ConsumeVerification(context.Context, []byte, time.Time) (Account, bool, error)
	ConsumeReset(context.Context, []byte, string, time.Time) (string, error)
	CheckAndRecordResend(context.Context, string, string, time.Time) error
	LoginFailureCount(context.Context, string, string, time.Time) (int, error)
	RecordLoginFailure(context.Context, string, string, time.Time) error
	ClearLoginFailures(context.Context, string, string) error
	CompleteLoginAttempt(context.Context, string, string, time.Time, bool) (bool, error)
	CreateSession(context.Context, SessionRecord) error
	RotateRefresh(context.Context, []byte, SessionRecord, time.Time) (Account, time.Time, error)
	RevokeFamilyByAccess(context.Context, []byte, time.Time, string) error
	AuthenticateAccess(context.Context, []byte, time.Time) (Account, error)
	RevokeAccountSessions(context.Context, string, time.Time, string) error
	HasPermission(context.Context, PermissionQuery, time.Time) (bool, error)
	GrantPermission(context.Context, PermissionGrant) error
	RevokePermission(context.Context, PermissionQuery, string, string, time.Time) error
	CreateOrganizerApplication(context.Context, OrganizerApplication) error
	OrganizerApplication(context.Context, string) (OrganizerApplication, error)
	ReviewOrganizerApplication(context.Context, OrganizerApplication, *PermissionGrant) error
	SetAccountState(context.Context, string, AccountState, string, string, time.Time) error
	BootstrapAdmin(context.Context, string, string, time.Time) error
}

type TokenRecord struct {
	ID, AccountID, Purpose string
	Hash                   []byte
	ExpiresAt, CreatedAt   time.Time
}

type SessionRecord struct {
	FamilyID, AccountID, AccessID, RefreshID     string
	AccessHash, RefreshHash                      []byte
	AccessExpiresAt, RefreshExpiresAt, CreatedAt time.Time
}

type PermissionGrant struct {
	ID, AccountID                            string
	Permission                               Permission
	ScopeType, ScopeID, GrantedBy, Rationale string
	GrantedAt                                time.Time
}
