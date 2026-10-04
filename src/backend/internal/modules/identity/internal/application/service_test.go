package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"badmintonhub/internal/platform/clock"
)

type fakeHasher struct{ verifyCalls int }

func (h *fakeHasher) Hash(value string) (string, error) { return "hash:" + value, nil }
func (h *fakeHasher) Verify(value, encoded string) (bool, error) {
	h.verifyCalls++
	return encoded == "hash:"+value, nil
}

type fakeDeriver struct{}

func (fakeDeriver) Derive(tokenID, accountID, purpose string) (string, []byte, error) {
	return "derived-token-value-long-enough-123456", []byte("hashed-token"), nil
}

type fakeStore struct {
	Store
	created         Account
	obligation      EmailObligation
	lookupErr       error
	account         Account
	passwordHash    string
	loginAllowed    bool
	loginSucceeded  bool
	rotatedExpiry   time.Time
	permission      bool
	permissionQuery PermissionQuery
	application     OrganizerApplication
}

func (f *fakeStore) CreateAccount(_ context.Context, a Account, _ string, _ TokenRecord, o EmailObligation) error {
	f.created = a
	f.obligation = o
	return nil
}
func (f *fakeStore) AccountByCanonicalEmail(context.Context, string) (Account, string, error) {
	return f.account, f.passwordHash, f.lookupErr
}
func (f *fakeStore) CompleteLoginAttempt(_ context.Context, _, _ string, _ time.Time, succeeded bool) (bool, error) {
	f.loginSucceeded = succeeded
	return f.loginAllowed, nil
}
func (f *fakeStore) RotateRefresh(context.Context, []byte, SessionRecord, time.Time) (Account, time.Time, error) {
	return f.account, f.rotatedExpiry, nil
}
func (f *fakeStore) AccountByID(context.Context, string) (Account, error) { return f.account, nil }
func (f *fakeStore) HasPermission(_ context.Context, q PermissionQuery, _ time.Time) (bool, error) {
	f.permissionQuery = q
	return f.permission, nil
}
func (f *fakeStore) OrganizerApplication(context.Context, string) (OrganizerApplication, error) {
	return f.application, nil
}

func newTestService(t *testing.T, store Store, hasher PasswordHasher) *Service {
	t.Helper()
	service, err := New(store, hasher, fakeDeriver{}, nil, clock.Fixed{Time: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestCanonicalEmailOnlyTrimsAndFoldsCase(t *testing.T) {
	got, err := CanonicalEmail("  Player.Name+tag@Example.COM  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "player.name+tag@example.com" {
		t.Fatalf("canonical=%q", got)
	}
}

func TestRegisterPersistsCanonicalEmailAndMetadataOnlyObligation(t *testing.T) {
	store := &fakeStore{}
	service := newTestService(t, store, &fakeHasher{})
	account, err := service.Register(context.Background(), RegisterRequest{Email: " User+tag@Example.COM ", Password: "long-enough-password"})
	if err != nil {
		t.Fatal(err)
	}
	if store.created.Email != "user+tag@example.com" || account.Email != "User+tag@Example.COM" {
		t.Fatalf("stored=%q returned=%q", store.created.Email, account.Email)
	}
	if store.obligation.TokenID == "" || store.obligation.AccountID == "" || store.obligation.Purpose != "VERIFY_EMAIL" {
		t.Fatalf("obligation=%+v", store.obligation)
	}
}

func TestLoginUnknownEmailRunsDummyPasswordVerification(t *testing.T) {
	hasher := &fakeHasher{}
	store := &fakeStore{lookupErr: ErrInvalidCredentials, loginAllowed: true}
	service := newTestService(t, store, hasher)
	_, err := service.Login(context.Background(), LoginRequest{Email: "missing@example.com", Password: "guess", SourceKey: "source"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("error=%v", err)
	}
	if hasher.verifyCalls != 1 {
		t.Fatalf("verify calls=%d", hasher.verifyCalls)
	}
}

func TestRefreshReportsOriginalFamilyExpiry(t *testing.T) {
	expires := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{account: Account{ID: "a", State: AccountActive}, rotatedExpiry: expires}
	service := newTestService(t, store, &fakeHasher{})
	token, _, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := service.Refresh(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if !tokens.RefreshExpiresAt.Equal(expires) {
		t.Fatalf("expiry=%v want %v", tokens.RefreshExpiresAt, expires)
	}
}

func TestOrganizerReviewerCannotReviewOwnApplication(t *testing.T) {
	store := &fakeStore{account: Account{ID: "admin", State: AccountActive}, permission: true, application: OrganizerApplication{ID: "app", AccountID: "admin", Status: "PENDING"}}
	service := newTestService(t, store, &fakeHasher{})
	err := service.ReviewOrganizer(context.Background(), ReviewOrganizerRequest{ApplicationID: "app", AdminID: "admin", Decision: "APPROVED", Rationale: "review"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("error=%v", err)
	}
}

func TestSuspendedAccountHasNoPermission(t *testing.T) {
	store := &fakeStore{account: Account{ID: "a", State: AccountSuspended}, permission: true}
	service := newTestService(t, store, &fakeHasher{})
	allowed, err := service.HasPermission(context.Background(), PermissionQuery{AccountID: "a", Permission: PermissionVenueManage, ScopeType: "GLOBAL"})
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("suspended account was allowed")
	}
}
