package application

import (
	"context"
	"strings"

	"badmintonhub/internal/platform/id"
)

func (s *Service) HasPermission(ctx context.Context, query PermissionQuery) (bool, error) {
	if query.AccountID == "" || query.Permission == "" || query.ScopeType == "" {
		return false, ErrInvalidInput
	}
	account, err := s.store.AccountByID(ctx, query.AccountID)
	if err != nil {
		return false, err
	}
	if account.State == AccountSuspended || account.State == AccountUnverified {
		return false, nil
	}
	return s.store.HasPermission(ctx, query, s.clock.Now().UTC())
}

func (s *Service) GrantPermission(ctx context.Context, grant PermissionGrant) error {
	if grant.AccountID == "" || grant.GrantedBy == "" || grant.Permission == "" || grant.ScopeType == "" || strings.TrimSpace(grant.Rationale) == "" || grant.AccountID == grant.GrantedBy {
		return ErrInvalidInput
	}
	allowed, err := s.HasPermission(ctx, PermissionQuery{AccountID: grant.GrantedBy, Permission: PermissionAssignmentsGrant, ScopeType: "GLOBAL"})
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	grant.ID, err = id.New()
	if err != nil {
		return err
	}
	grant.GrantedAt = s.clock.Now().UTC()
	grant.Rationale = strings.TrimSpace(grant.Rationale)
	return s.store.GrantPermission(ctx, grant)
}

func (s *Service) ApplyOrganizer(ctx context.Context, request OrganizerRequest) (OrganizerApplication, error) {
	if request.AccountID == "" || strings.TrimSpace(request.Reason) == "" || strings.TrimSpace(request.Contact) == "" || s.eligibility == nil {
		return OrganizerApplication{}, ErrInvalidInput
	}
	account, err := s.store.AccountByID(ctx, request.AccountID)
	if err != nil {
		return OrganizerApplication{}, err
	}
	if account.State != AccountActive {
		return OrganizerApplication{}, ErrForbidden
	}
	eligible, err := s.eligibility.OrganizerEligibility(ctx, request.AccountID)
	if err != nil {
		return OrganizerApplication{}, err
	}
	if !eligible.ProfileComplete || !eligible.AdultEligible {
		return OrganizerApplication{}, ErrForbidden
	}
	applicationID, err := id.New()
	if err != nil {
		return OrganizerApplication{}, err
	}
	application := OrganizerApplication{ID: applicationID, AccountID: request.AccountID, Reason: strings.TrimSpace(request.Reason), Contact: strings.TrimSpace(request.Contact), Status: "PENDING", SubmittedAt: s.clock.Now().UTC()}
	if err := s.store.CreateOrganizerApplication(ctx, application); err != nil {
		return OrganizerApplication{}, err
	}
	return application, nil
}

func (s *Service) ReviewOrganizer(ctx context.Context, request ReviewOrganizerRequest) error {
	if request.ApplicationID == "" || request.AdminID == "" || strings.TrimSpace(request.Rationale) == "" || (request.Decision != "APPROVED" && request.Decision != "REJECTED") {
		return ErrInvalidInput
	}
	allowed, err := s.HasPermission(ctx, PermissionQuery{AccountID: request.AdminID, Permission: PermissionOrganizerReview, ScopeType: "GLOBAL"})
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	application, err := s.store.OrganizerApplication(ctx, request.ApplicationID)
	if err != nil {
		return err
	}
	if application.AccountID == request.AdminID {
		return ErrConflict
	}
	if application.Status != "PENDING" {
		return ErrConflict
	}
	now := s.clock.Now().UTC()
	application.Status = request.Decision
	application.ReviewedBy = request.AdminID
	application.ReviewedAt = &now
	application.ReviewRationale = strings.TrimSpace(request.Rationale)
	var grant *PermissionGrant
	if request.Decision == "APPROVED" {
		permissionID, err := id.New()
		if err != nil {
			return err
		}
		grant = &PermissionGrant{ID: permissionID, AccountID: application.AccountID, Permission: PermissionOrganizerPublish, ScopeType: "OWN", GrantedBy: request.AdminID, Rationale: application.ReviewRationale, GrantedAt: now}
	}
	return s.store.ReviewOrganizerApplication(ctx, application, grant)
}

func (s *Service) RevokeOrganizer(ctx context.Context, request RevokeOrganizerRequest) error {
	if request.AccountID == "" || request.AdminID == "" || request.AccountID == request.AdminID || strings.TrimSpace(request.Rationale) == "" {
		return ErrInvalidInput
	}
	allowed, err := s.HasPermission(ctx, PermissionQuery{AccountID: request.AdminID, Permission: PermissionOrganizerReview, ScopeType: "GLOBAL"})
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return s.store.RevokePermission(ctx, PermissionQuery{AccountID: request.AccountID, Permission: PermissionOrganizerPublish, ScopeType: "OWN"}, request.AdminID, strings.TrimSpace(request.Rationale), s.clock.Now().UTC())
}

func (s *Service) SetAccountState(ctx context.Context, accountID, adminID string, state AccountState, rationale string) error {
	if accountID == "" || adminID == "" || accountID == adminID || strings.TrimSpace(rationale) == "" || (state != AccountActive && state != AccountRestricted && state != AccountSuspended) {
		return ErrInvalidInput
	}
	allowed, err := s.HasPermission(ctx, PermissionQuery{AccountID: adminID, Permission: PermissionAccountManage, ScopeType: "GLOBAL"})
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	if err := s.store.SetAccountState(ctx, accountID, state, adminID, strings.TrimSpace(rationale), s.clock.Now().UTC()); err != nil {
		return err
	}
	return nil
}

// BootstrapAdmin is intended only for an operator-controlled bootstrap command.
// The store permits it exactly once, while no permission assignment exists.
func (s *Service) BootstrapAdmin(ctx context.Context, request BootstrapAdminRequest) error {
	if request.AccountID == "" || strings.TrimSpace(request.Rationale) == "" {
		return ErrInvalidInput
	}
	account, err := s.store.AccountByID(ctx, request.AccountID)
	if err != nil {
		return err
	}
	if account.State != AccountActive {
		return ErrForbidden
	}
	return s.store.BootstrapAdmin(ctx, request.AccountID, strings.TrimSpace(request.Rationale), s.clock.Now().UTC())
}
