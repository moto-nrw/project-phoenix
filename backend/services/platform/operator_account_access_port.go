package platform

import (
	"context"
	"errors"
	"net"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// Operator-led management of which schools an existing account may access
// (issue #1021) is owned by Identity & Access (#3252). The operator routes
// still reach it through the retained provisioning contract, which delegates
// to the consumer-owned OperatorAccountAccess port below; the serving root
// binds the port to the public module and translates its outcomes into the
// retained error types.

// AccountTenantRole is one role an account holds at one school.
type AccountTenantRole struct {
	ID       int64   `json:"id,string"`
	Name     string  `json:"name"`
	IsSystem bool    `json:"is_system"`
	BaseRole *string `json:"base_role,omitempty"`
}

// AccountTenantAccessEntry is one school an account has (or had) access to,
// including the roles it holds there.
type AccountTenantAccessEntry struct {
	authModels.AccountTenantAccessInfo
	Roles []AccountTenantRole `json:"roles"`
}

// GrantAccountTenantAccessRequest carries the optional person data used when
// the account has no person record anywhere yet.
type GrantAccountTenantAccessRequest struct {
	RoleID    int64
	FirstName string
	LastName  string
	Position  string
}

// OperatorAccountAccess is the account school-access capability the
// retained provisioning contract delegates to. Errors arrive in the retained
// shapes (AccountNotFoundError, AccountTenantAccessNotFoundError,
// SchoolNotFoundError, SchoolAlreadyDeletedError, ConflictError,
// InvalidDataError).
type OperatorAccountAccess interface {
	ListAccountTenantAccess(ctx context.Context, accountID int64) ([]AccountTenantAccessEntry, error)
	ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]AccountTenantRole, error)
	GrantAccountTenantAccess(ctx context.Context, accountID, schoolID int64, req GrantAccountTenantAccessRequest, operatorID int64, clientIP net.IP) ([]AccountTenantAccessEntry, error)
	UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP net.IP) ([]AccountTenantAccessEntry, error)
	RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP net.IP) ([]AccountTenantAccessEntry, error)
}

// ErrOperatorAccountAccessUnavailable reports a provisioning service
// composed without the Identity & Access account access port.
var ErrOperatorAccountAccessUnavailable = errors.New("operator account access is not composed")

// ListAccountTenantAccess returns every school mapping of one account.
func (s *operatorProvisioningService) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]AccountTenantAccessEntry, error) {
	if s.AccountAccess == nil {
		return nil, ErrOperatorAccountAccessUnavailable
	}
	return s.AccountAccess.ListAccountTenantAccess(ctx, accountID)
}

// ListAssignableSchoolRoles returns the roles an operator may assign at one
// school, projected onto the retained role model the operator role picker
// renders.
func (s *operatorProvisioningService) ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]*authModels.Role, error) {
	if s.AccountAccess == nil {
		return nil, ErrOperatorAccountAccessUnavailable
	}
	options, err := s.AccountAccess.ListAssignableSchoolRoles(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	roles := make([]*authModels.Role, 0, len(options))
	for _, option := range options {
		role := &authModels.Role{Name: option.Name, IsSystem: option.IsSystem, BaseRole: option.BaseRole}
		role.ID = option.ID
		roles = append(roles, role)
	}
	return roles, nil
}

// GrantAccountTenantAccess gives an existing account access to an
// additional school and assigns the requested role there.
func (s *operatorProvisioningService) GrantAccountTenantAccess(
	ctx context.Context,
	accountID, schoolID int64,
	req GrantAccountTenantAccessRequest,
	operatorID int64,
	clientIP net.IP,
) ([]AccountTenantAccessEntry, error) {
	if s.AccountAccess == nil {
		return nil, ErrOperatorAccountAccessUnavailable
	}
	return s.AccountAccess.GrantAccountTenantAccess(ctx, accountID, schoolID, req, operatorID, clientIP)
}

// UpdateAccountTenantRole changes the role an account holds at one school.
func (s *operatorProvisioningService) UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP net.IP) ([]AccountTenantAccessEntry, error) {
	if s.AccountAccess == nil {
		return nil, ErrOperatorAccountAccessUnavailable
	}
	return s.AccountAccess.UpdateAccountTenantRole(ctx, accountID, schoolID, roleID, operatorID, clientIP)
}

// RevokeAccountTenantAccess removes an account's access to one school.
func (s *operatorProvisioningService) RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP net.IP) ([]AccountTenantAccessEntry, error) {
	if s.AccountAccess == nil {
		return nil, ErrOperatorAccountAccessUnavailable
	}
	return s.AccountAccess.RevokeAccountTenantAccess(ctx, accountID, schoolID, operatorID, clientIP)
}
