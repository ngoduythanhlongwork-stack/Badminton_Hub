package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"badmintonhub/internal/platform/clock"
	"badmintonhub/internal/platform/id"
)

const (
	verificationTTL = 24 * time.Hour
	resetTTL        = 30 * time.Minute
	accessTTL       = 15 * time.Minute
	refreshTTL      = 30 * 24 * time.Hour
)

type Service struct {
	store       Store
	hasher      PasswordHasher
	deriver     ActionTokenDeriver
	dummyHash   string
	eligibility OrganizerEligibility
	clock       clock.Clock
}

func New(store Store, hasher PasswordHasher, deriver ActionTokenDeriver, eligibility OrganizerEligibility, c clock.Clock) (*Service, error) {
	if store == nil || hasher == nil || deriver == nil {
		return nil, errors.New("identity store, hasher, and action token deriver are required")
	}
	if c == nil {
		c = clock.System{}
	}
	dummyHash, err := hasher.Hash("identity-dummy-password-never-used")
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}
	return &Service{store: store, hasher: hasher, deriver: deriver, dummyHash: dummyHash, eligibility: eligibility, clock: c}, nil
}

func CanonicalEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len(trimmed) > 320 || strings.Count(trimmed, "@") != 1 {
		return "", ErrInvalidInput
	}
	parts := strings.SplitN(trimmed, "@", 2)
	if parts[0] == "" || parts[1] == "" || strings.ContainsAny(trimmed, "\r\n\t ") {
		return "", ErrInvalidInput
	}
	return strings.ToLower(trimmed), nil
}

func validPassword(value string) bool { n := utf8.RuneCountInString(value); return n >= 12 && n <= 128 }

func (s *Service) Register(ctx context.Context, request RegisterRequest) (Account, error) {
	canonical, err := CanonicalEmail(request.Email)
	if err != nil || !validPassword(request.Password) {
		return Account{}, ErrInvalidInput
	}
	encoded, err := s.hasher.Hash(request.Password)
	if err != nil {
		return Account{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.clock.Now().UTC()
	accountID, err := id.New()
	if err != nil {
		return Account{}, err
	}
	tokenID, err := id.New()
	if err != nil {
		return Account{}, err
	}
	_, hash, err := s.deriver.Derive(tokenID, accountID, "VERIFY_EMAIL")
	if err != nil {
		return Account{}, err
	}
	account := Account{ID: accountID, Email: strings.TrimSpace(request.Email), State: AccountUnverified}
	record := TokenRecord{ID: tokenID, AccountID: accountID, Purpose: "VERIFY_EMAIL", Hash: hash, ExpiresAt: now.Add(verificationTTL), CreatedAt: now}
	message := EmailObligation{Kind: "VERIFY_EMAIL", Recipient: account.Email, TokenID: tokenID, AccountID: accountID, Purpose: record.Purpose, ExpiresAt: record.ExpiresAt}
	if err := s.store.CreateAccount(ctx, accountWithCanonical(account, canonical), encoded, record, message); err != nil {
		return Account{}, err
	}
	return account, nil
}

// accountWithCanonical uses Email as the persisted canonical value; display email is not a public invariant.
func accountWithCanonical(account Account, canonical string) Account {
	account.Email = canonical
	return account
}

func (s *Service) VerifyEmail(ctx context.Context, token string) (Account, error) {
	hash, err := tokenHash(token)
	if err != nil {
		return Account{}, ErrInvalidToken
	}
	account, _, err := s.store.ConsumeVerification(ctx, hash, s.clock.Now().UTC())
	return account, err
}

func (s *Service) ResendVerification(ctx context.Context, email, sourceKey string) error {
	canonical, err := CanonicalEmail(email)
	if err != nil || sourceKey == "" {
		return ErrInvalidInput
	}
	account, _, err := s.store.AccountByCanonicalEmail(ctx, canonical)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return nil
		}
		return err
	}
	if account.State != AccountUnverified {
		return nil
	}
	now := s.clock.Now().UTC()
	if err := s.store.CheckAndRecordResend(ctx, account.ID, sourceKey, now); err != nil {
		return err
	}
	tokenID, err := id.New()
	if err != nil {
		return err
	}
	_, hash, err := s.deriver.Derive(tokenID, account.ID, "VERIFY_EMAIL")
	if err != nil {
		return err
	}
	record := TokenRecord{ID: tokenID, AccountID: account.ID, Purpose: "VERIFY_EMAIL", Hash: hash, ExpiresAt: now.Add(verificationTTL), CreatedAt: now}
	return s.store.ReplaceActionToken(ctx, record, EmailObligation{Kind: "VERIFY_EMAIL", Recipient: account.Email, TokenID: tokenID, AccountID: account.ID, Purpose: record.Purpose, ExpiresAt: record.ExpiresAt})
}

func (s *Service) Login(ctx context.Context, request LoginRequest) (Tokens, error) {
	canonical, err := CanonicalEmail(request.Email)
	if err != nil || request.SourceKey == "" {
		return Tokens{}, ErrInvalidCredentials
	}
	now := s.clock.Now().UTC()
	account, encoded, lookupErr := s.store.AccountByCanonicalEmail(ctx, canonical)
	valid := false
	if lookupErr == nil {
		valid, err = s.hasher.Verify(request.Password, encoded)
		if err != nil {
			return Tokens{}, err
		}
	} else {
		_, err = s.hasher.Verify(request.Password, s.dummyHash)
		if err != nil {
			return Tokens{}, err
		}
	}
	if lookupErr != nil || !valid {
		allowed, err := s.store.CompleteLoginAttempt(ctx, canonical, request.SourceKey, now, false)
		if err != nil {
			return Tokens{}, err
		}
		if !allowed {
			return Tokens{}, ErrRateLimited
		}
		return Tokens{}, ErrInvalidCredentials
	}
	allowed, err := s.store.CompleteLoginAttempt(ctx, canonical, request.SourceKey, now, true)
	if err != nil {
		return Tokens{}, err
	}
	if !allowed {
		return Tokens{}, ErrRateLimited
	}
	if account.State != AccountActive && account.State != AccountRestricted {
		return Tokens{}, ErrAccountUnavailable
	}
	tokens, record, err := session(account.ID, now)
	if err != nil {
		return Tokens{}, err
	}
	if err := s.store.CreateSession(ctx, record); err != nil {
		return Tokens{}, err
	}
	return tokens, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	hash, err := tokenHash(refreshToken)
	if err != nil {
		return Tokens{}, ErrInvalidToken
	}
	now := s.clock.Now().UTC()
	tokens, record, err := session("", now)
	if err != nil {
		return Tokens{}, err
	}
	account, familyExpiry, err := s.store.RotateRefresh(ctx, hash, record, now)
	if err != nil {
		return Tokens{}, err
	}
	if account.State != AccountActive && account.State != AccountRestricted {
		return Tokens{}, ErrAccountUnavailable
	}
	tokens.RefreshExpiresAt = familyExpiry
	return tokens, nil
}

func (s *Service) Logout(ctx context.Context, accessToken string) error {
	hash, err := tokenHash(accessToken)
	if err != nil {
		return ErrInvalidToken
	}
	return s.store.RevokeFamilyByAccess(ctx, hash, s.clock.Now().UTC(), "LOGOUT")
}

func (s *Service) Authenticate(ctx context.Context, accessToken string) (Account, error) {
	hash, err := tokenHash(accessToken)
	if err != nil {
		return Account{}, ErrInvalidToken
	}
	return s.store.AuthenticateAccess(ctx, hash, s.clock.Now().UTC())
}

// ForgotPassword deliberately returns nil for unknown emails.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	canonical, err := CanonicalEmail(email)
	if err != nil {
		return nil
	}
	account, _, err := s.store.AccountByCanonicalEmail(ctx, canonical)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return nil
		}
		return err
	}
	now := s.clock.Now().UTC()
	tokenID, err := id.New()
	if err != nil {
		return err
	}
	_, hash, err := s.deriver.Derive(tokenID, account.ID, "RESET_PASSWORD")
	if err != nil {
		return err
	}
	record := TokenRecord{ID: tokenID, AccountID: account.ID, Purpose: "RESET_PASSWORD", Hash: hash, ExpiresAt: now.Add(resetTTL), CreatedAt: now}
	return s.store.ReplaceActionToken(ctx, record, EmailObligation{Kind: "RESET_PASSWORD", Recipient: account.Email, TokenID: tokenID, AccountID: account.ID, Purpose: record.Purpose, ExpiresAt: record.ExpiresAt})
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if !validPassword(password) {
		return ErrInvalidInput
	}
	hash, err := tokenHash(token)
	if err != nil {
		return ErrInvalidToken
	}
	encoded, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	_, err = s.store.ConsumeReset(ctx, hash, encoded, s.clock.Now().UTC())
	return err
}

func session(accountID string, now time.Time) (Tokens, SessionRecord, error) {
	access, accessHash, err := newToken()
	if err != nil {
		return Tokens{}, SessionRecord{}, err
	}
	refresh, refreshHash, err := newToken()
	if err != nil {
		return Tokens{}, SessionRecord{}, err
	}
	familyID, err := id.New()
	if err != nil {
		return Tokens{}, SessionRecord{}, err
	}
	accessID, err := id.New()
	if err != nil {
		return Tokens{}, SessionRecord{}, err
	}
	refreshID, err := id.New()
	if err != nil {
		return Tokens{}, SessionRecord{}, err
	}
	tokens := Tokens{access, now.Add(accessTTL), refresh, now.Add(refreshTTL)}
	record := SessionRecord{familyID, accountID, accessID, refreshID, accessHash, refreshHash, tokens.AccessExpiresAt, tokens.RefreshExpiresAt, now}
	return tokens, record, nil
}

func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(value))
	return value, sum[:], nil
}

func tokenHash(value string) ([]byte, error) {
	if len(value) < 32 || len(value) > 256 {
		return nil, ErrInvalidToken
	}
	if _, err := base64.RawURLEncoding.DecodeString(value); err != nil {
		return nil, ErrInvalidToken
	}
	sum := sha256.Sum256([]byte(value))
	return sum[:], nil
}
