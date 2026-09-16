package authtest

import (
	"context"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	svcauth "github.com/moto-nrw/project-phoenix/services/auth"
)

// InvitationServiceMock is a func-field test double for auth.InvitationService.
type InvitationServiceMock struct {
	CreateInvitationFn                       func(ctx context.Context, req svcauth.InvitationRequest) (*authModels.InvitationToken, error)
	ValidateInvitationFn                     func(ctx context.Context, token string) (*svcauth.InvitationValidationResult, error)
	AcceptInvitationFn                       func(ctx context.Context, token string, userData svcauth.UserRegistrationData) (*authModels.Account, error)
	ResendInvitationFn                       func(ctx context.Context, invitationID int64, actorAccountID int64) error
	ListPendingInvitationsFn                 func(ctx context.Context) ([]*authModels.InvitationToken, error)
	RevokeInvitationFn                       func(ctx context.Context, invitationID int64, actorAccountID int64) error
	InvalidatePendingInvitationsByTenantIDFn func(ctx context.Context, tenantID int64) (int, error)
	CleanupExpiredInvitationsFn              func(ctx context.Context) (int, error)
	GetTenantSubdomainForTokenFn             func(ctx context.Context, token string) string
}

var _ svcauth.InvitationService = (*InvitationServiceMock)(nil)

func (m *InvitationServiceMock) CreateInvitation(ctx context.Context, req svcauth.InvitationRequest) (*authModels.InvitationToken, error) {
	if m.CreateInvitationFn != nil {
		return m.CreateInvitationFn(ctx, req)
	}
	return nil, nil
}

func (m *InvitationServiceMock) ValidateInvitation(ctx context.Context, token string) (*svcauth.InvitationValidationResult, error) {
	if m.ValidateInvitationFn != nil {
		return m.ValidateInvitationFn(ctx, token)
	}
	return nil, nil
}

func (m *InvitationServiceMock) AcceptInvitation(ctx context.Context, token string, userData svcauth.UserRegistrationData) (*authModels.Account, error) {
	if m.AcceptInvitationFn != nil {
		return m.AcceptInvitationFn(ctx, token, userData)
	}
	return nil, nil
}

func (m *InvitationServiceMock) ResendInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error {
	if m.ResendInvitationFn != nil {
		return m.ResendInvitationFn(ctx, invitationID, actorAccountID)
	}
	return nil
}

func (m *InvitationServiceMock) ListPendingInvitations(ctx context.Context) ([]*authModels.InvitationToken, error) {
	if m.ListPendingInvitationsFn != nil {
		return m.ListPendingInvitationsFn(ctx)
	}
	return nil, nil
}

func (m *InvitationServiceMock) RevokeInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error {
	if m.RevokeInvitationFn != nil {
		return m.RevokeInvitationFn(ctx, invitationID, actorAccountID)
	}
	return nil
}

func (m *InvitationServiceMock) InvalidatePendingInvitationsByTenantID(ctx context.Context, tenantID int64) (int, error) {
	if m.InvalidatePendingInvitationsByTenantIDFn != nil {
		return m.InvalidatePendingInvitationsByTenantIDFn(ctx, tenantID)
	}
	return 0, nil
}

func (m *InvitationServiceMock) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	if m.CleanupExpiredInvitationsFn != nil {
		return m.CleanupExpiredInvitationsFn(ctx)
	}
	return 0, nil
}

func (m *InvitationServiceMock) GetTenantSubdomainForToken(ctx context.Context, token string) string {
	if m.GetTenantSubdomainForTokenFn != nil {
		return m.GetTenantSubdomainForTokenFn(ctx, token)
	}
	return ""
}
