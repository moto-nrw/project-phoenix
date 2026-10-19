package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

var errOperatorAuthenticationUnavailable = identityaccess.ErrOperatorAuthenticationUnavailable

func (e engine) LoginOperatorWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
	if e.operatorAuth == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	result, err := e.operatorAuth.LoginWithMFAGate(e.attach(ctx), email, password, ipAddress, userAgent, trustedDeviceCookie)
	return operatorLoginResult(result), operatorError(err)
}

func (e engine) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error) {
	if e.operatorAuth == nil {
		return "", "", errOperatorAuthenticationUnavailable
	}
	access, refresh, err := e.operatorAuth.IssueTokensForAuthenticatedOperator(e.attach(ctx), operatorID, ipAddress, userAgent)
	return access, refresh, operatorError(err)
}

func (e engine) RefreshOperatorToken(ctx context.Context, operatorID int64, refreshToken string) (string, string, error) {
	if e.operatorAuth == nil {
		return "", "", errOperatorAuthenticationUnavailable
	}
	access, refresh, err := e.operatorAuth.RefreshToken(e.attach(ctx), operatorID, refreshToken)
	return access, refresh, operatorError(err)
}

func (e engine) UpdateOperatorProfile(ctx context.Context, operatorID int64, displayName string) (identityaccess.Operator, error) {
	if e.operatorAuth == nil {
		return identityaccess.Operator{}, errOperatorAuthenticationUnavailable
	}
	operator, err := e.operatorAuth.UpdateProfile(e.attach(ctx), operatorID, displayName)
	return identityaccess.Operator(operator), operatorError(err)
}

func (e engine) ChangeOperatorPassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	if e.operatorAuth == nil {
		return errOperatorAuthenticationUnavailable
	}
	return operatorError(e.operatorAuth.ChangePassword(e.attach(ctx), operatorID, currentPassword, newPassword))
}

func (e engine) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]identityaccess.AccountTenantAccess, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	entries, err := e.accountAccess.ListAccountTenantAccess(e.attach(ctx), accountID)
	return accountTenantAccess(entries), operatorError(err)
}

func (e engine) ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]identityaccess.AccountTenantRole, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	roles, err := e.accountAccess.ListAssignableSchoolRoles(e.attach(ctx), schoolID)
	return accountTenantRoles(roles), operatorError(err)
}

func (e engine) GrantAccountTenantAccess(ctx context.Context, request identityaccess.GrantAccountTenantAccess) ([]identityaccess.AccountTenantAccess, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	entries, err := e.accountAccess.GrantAccountTenantAccess(e.attach(ctx), domain.GrantAccountTenantAccess(request))
	return accountTenantAccess(entries), operatorError(err)
}

func (e engine) UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP string) ([]identityaccess.AccountTenantAccess, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	entries, err := e.accountAccess.UpdateAccountTenantRole(e.attach(ctx), accountID, schoolID, roleID, operatorID, clientIP)
	return accountTenantAccess(entries), operatorError(err)
}

func (e engine) RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP string) ([]identityaccess.AccountTenantAccess, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	entries, err := e.accountAccess.RevokeAccountTenantAccess(e.attach(ctx), accountID, schoolID, operatorID, clientIP)
	return accountTenantAccess(entries), operatorError(err)
}

func operatorLoginResult(value *domain.OperatorLoginResult) *identityaccess.OperatorLoginResult {
	if value == nil {
		return nil
	}
	result := identityaccess.OperatorLoginResult{
		Status: identityaccess.LoginStatus(value.Status), AccessToken: value.AccessToken, RefreshToken: value.RefreshToken,
		ChallengeToken: value.ChallengeToken, MaskedEmail: value.MaskedEmail, MFAEnrollmentRequired: value.MFAEnrollmentRequired,
		TrustedDeviceEnabled: value.TrustedDeviceEnabled, TrustedDeviceDays: value.TrustedDeviceDays,
	}
	if value.Operator != nil {
		operator := identityaccess.Operator(*value.Operator)
		result.Operator = &operator
	}
	return &result
}

func accountTenantRoles(values []domain.AccountTenantRole) []identityaccess.AccountTenantRole {
	if values == nil {
		return nil
	}
	result := make([]identityaccess.AccountTenantRole, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.AccountTenantRole(value))
	}
	return result
}

func accountTenantAccess(values []domain.AccountTenantAccess) []identityaccess.AccountTenantAccess {
	if values == nil {
		return nil
	}
	result := make([]identityaccess.AccountTenantAccess, 0, len(values))
	for _, value := range values {
		roles := accountTenantRoles(value.Roles)
		if roles == nil {
			roles = []identityaccess.AccountTenantRole{}
		}
		result = append(result, identityaccess.AccountTenantAccess{
			TenantID: value.TenantID, SchoolName: value.SchoolName, SchoolSlug: value.SchoolSlug, SchoolActive: value.SchoolActive,
			OrganizationID: value.OrganizationID, OrganizationName: value.OrganizationName, Status: value.Status,
			ActivatedAt: value.ActivatedAt, DeactivatedAt: value.DeactivatedAt, HasPerson: value.HasPerson, HasStaff: value.HasStaff,
			Roles: roles,
		})
	}
	return result
}

var operatorSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrOperatorEmailInvalid, identityaccess.ErrOperatorEmailInvalid},
	{domain.ErrOperatorEmailTooLong, identityaccess.ErrOperatorEmailTooLong},
	{domain.ErrOperatorPasswordTooWeak, identityaccess.ErrOperatorPasswordTooWeak},
	{domain.ErrOperatorDisplayNameRequired, identityaccess.ErrOperatorDisplayNameRequired},
	{domain.ErrOperatorDisplayNameTooLong, identityaccess.ErrOperatorDisplayNameTooLong},
	{domain.ErrOperatorInvalidCredentials, identityaccess.ErrOperatorInvalidCredentials},
	{domain.ErrOperatorInactive, identityaccess.ErrOperatorInactive},
	{domain.ErrOperatorNotFound, identityaccess.ErrOperatorNotFound},
	{domain.ErrOperatorRefreshTokenInvalid, identityaccess.ErrOperatorRefreshTokenInvalid},
	{domain.ErrOperatorPasswordMismatch, identityaccess.ErrOperatorPasswordMismatch},
	{domain.ErrAccountTenantAccessNotFound, identityaccess.ErrAccountTenantAccessNotFound},
	{domain.ErrAccountTenantAccessExists, identityaccess.ErrAccountTenantAccessExists},
	{domain.ErrSchoolNotFound, identityaccess.ErrSchoolNotFound},
	{domain.ErrSchoolDeleted, identityaccess.ErrSchoolDeleted},
}

// operatorError translates an operator flow error to the public contract:
// an input rejection keeps its message under the public type, an operator
// sentinel gains its public counterpart, and everything else falls through
// to the account-authentication translation the access flows share.
func operatorError(err error) error {
	if err == nil {
		return nil
	}
	if invalid, ok := errors.AsType[*domain.InvalidInputError](err); ok {
		return &identityaccess.InvalidInputError{Err: operatorError(invalid.Err)}
	}
	for _, sentinel := range operatorSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return authenticationError(err)
}
