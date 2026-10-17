package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (e engine) RegisterSchoolAccount(ctx context.Context, request identityaccess.SchoolAccountRegistration) (identityaccess.ProvisionedAccount, error) {
	if e.provisioning == nil {
		return identityaccess.ProvisionedAccount{}, identityaccess.ErrAccountProvisioningUnavailable
	}
	provisioned, err := e.provisioning.RegisterSchoolAccount(e.attach(ctx), domain.SchoolAccountRegistration{
		TenantID: request.TenantID, Email: request.Email, Username: request.Username,
		Password: request.Password, RoleID: request.RoleID, Identity: provisioningIdentity(request.Identity),
	})
	return publicProvisionedAccount(provisioned), provisioningError(err)
}

func (e engine) LinkSchoolAccount(ctx context.Context, request identityaccess.SchoolAccountLink) (identityaccess.ProvisionedAccount, error) {
	if e.provisioning == nil {
		return identityaccess.ProvisionedAccount{}, identityaccess.ErrAccountProvisioningUnavailable
	}
	provisioned, err := e.provisioning.LinkSchoolAccount(e.attach(ctx), domain.SchoolAccountLink{
		TenantID: request.TenantID, Email: request.Email, RoleID: request.RoleID,
		Identity: provisioningIdentity(request.Identity),
	})
	return publicProvisionedAccount(provisioned), provisioningError(err)
}

func provisioningIdentity(identity *identityaccess.SchoolAccountIdentity) *domain.SchoolAccountIdentity {
	if identity == nil {
		return nil
	}
	return &domain.SchoolAccountIdentity{FirstName: identity.FirstName, LastName: identity.LastName, TagID: identity.TagID}
}

func publicProvisionedAccount(provisioned domain.ProvisionedAccount) identityaccess.ProvisionedAccount {
	result := identityaccess.ProvisionedAccount{
		Account: identityaccess.RegisteredAccount{
			ID: provisioned.Account.ID, Email: provisioned.Account.Email,
			Username: provisioned.Account.Username, Active: provisioned.Account.Active,
			CreatedAt: provisioned.Account.CreatedAt, UpdatedAt: provisioned.Account.UpdatedAt,
			LastLogin: provisioned.Account.LastLogin,
		},
	}
	if provisioned.Identity != nil {
		result.Identity = &identityaccess.SchoolIdentity{
			PersonID: provisioned.Identity.PersonID, StaffID: provisioned.Identity.StaffID,
			TeacherID: provisioned.Identity.TeacherID,
		}
	}
	return result
}

var provisioningSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrEmailAlreadyExists, identityaccess.ErrEmailAlreadyExists},
	{domain.ErrUsernameAlreadyExists, identityaccess.ErrUsernameAlreadyExists},
	{domain.ErrTenantRequiredForRoleAssignment, identityaccess.ErrTenantRequiredForRoleAssignment},
}

// provisioningError translates a flow error to the public contract: the
// operation envelope keeps its text and the registration sentinels gain
// their public twin. Everything else falls through to the role mapping,
// which already carries the role, account and school identity sentinels
// these flows share.
func provisioningError(err error) error {
	if err == nil {
		return nil
	}
	var operation *OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: provisioningError(operation.Err)}
	}
	for _, sentinel := range provisioningSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return roleError(err)
}
