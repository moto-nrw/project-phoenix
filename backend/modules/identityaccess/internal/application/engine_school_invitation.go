package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func PublicSchoolInvitation(invitation domain.SchoolInvitation) identityaccess.SchoolInvitation {
	return identityaccess.SchoolInvitation{
		ID: invitation.ID, TenantID: invitation.TenantID, Email: invitation.Email, Token: invitation.Token,
		RoleID: invitation.RoleID, RoleName: invitation.RoleName, ExpiresAt: invitation.ExpiresAt, UsedAt: invitation.UsedAt,
		CreatedBy: invitation.CreatedBy, CreatorEmail: invitation.CreatorEmail, FirstName: invitation.FirstName, LastName: invitation.LastName,
		Position: invitation.Position, CaregiverEnabled: invitation.CaregiverEnabled, PersonID: invitation.PersonID,
		Delivery: identityaccess.TokenDelivery(invitation.Delivery), CreatedAt: invitation.CreatedAt,
	}
}

var errSchoolInvitationUnavailable = identityaccess.ErrSchoolInvitationUnavailable

func (e engine) CreateSchoolInvitation(ctx context.Context, request identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error) {
	if e.invitations == nil {
		return identityaccess.SchoolInvitation{}, errSchoolInvitationUnavailable
	}
	invitation, err := e.invitations.CreateInvitation(e.attach(ctx), domain.SchoolInvitationRequest{
		Email: request.Email, RoleID: request.RoleID, TenantID: request.TenantID, FirstName: request.FirstName,
		LastName: request.LastName, Position: request.Position, CaregiverEnabled: request.CaregiverEnabled,
		PersonID: request.PersonID, CreatedBy: request.CreatedBy, SchoolName: request.SchoolName,
		ActorPermissions: request.ActorPermissions, OperatorGrant: request.OperatorGrant,
	})
	return PublicSchoolInvitation(invitation), invitationError(err)
}

func (e engine) ValidateSchoolInvitation(ctx context.Context, token string) (identityaccess.InvitationPreview, error) {
	if e.invitations == nil {
		return identityaccess.InvitationPreview{}, errSchoolInvitationUnavailable
	}
	preview, err := e.invitations.ValidateInvitation(e.attach(ctx), token)
	return identityaccess.InvitationPreview{
		Portal: identityaccess.InvitationPortal(preview.Portal), RequiresAccountLogin: preview.RequiresAccountLogin,
		Email: preview.Email, RoleName: preview.RoleName, FirstName: preview.FirstName, LastName: preview.LastName,
		Position: preview.Position, CaregiverEnabled: preview.CaregiverEnabled, ExpiresAt: preview.ExpiresAt,
	}, invitationError(err)
}

func (e engine) FindFormerEmployment(ctx context.Context, email string) (identityaccess.FormerEmployment, bool, error) {
	if e.invitations == nil {
		return identityaccess.FormerEmployment{}, false, errSchoolInvitationUnavailable
	}
	former, found, err := e.invitations.FindFormerEmployment(e.attach(ctx), email)
	if err != nil || !found {
		return identityaccess.FormerEmployment{}, false, invitationError(err)
	}
	return identityaccess.FormerEmployment{
		StartedOn: former.StartedOn, EndedOn: former.EndedOn,
		Resumes: []identityaccess.FormerEmploymentPart{
			identityaccess.FormerEmploymentMasterData,
			identityaccess.FormerEmploymentQualifications,
			identityaccess.FormerEmploymentWorkTime,
			identityaccess.FormerEmploymentPastShiftsAndAbsences,
		},
		NotReturning: []identityaccess.FormerEmploymentPart{
			identityaccess.FormerEmploymentStaffDocuments,
			identityaccess.FormerEmploymentFutureShiftsAndAbsences,
			identityaccess.FormerEmploymentGroupsClassesSubstitutions,
		},
	}, true, nil
}

func (e engine) AcceptSchoolInvitation(ctx context.Context, token string, registration identityaccess.InvitationRegistration) (identityaccess.Account, error) {
	if e.invitations == nil {
		return identityaccess.Account{}, errSchoolInvitationUnavailable
	}
	account, err := e.invitations.AcceptInvitation(e.attach(ctx), token, domain.InvitationRegistration{
		OwnerAccessToken: registration.OwnerAccessToken, FirstName: registration.FirstName, LastName: registration.LastName,
		Password: registration.Password, ConfirmPassword: registration.ConfirmPassword,
	})
	return identityaccess.Account{ID: account.ID, Email: account.Email}, invitationError(err)
}

func (e engine) ResendSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	if e.invitations == nil {
		return errSchoolInvitationUnavailable
	}
	return invitationError(e.invitations.ResendInvitation(e.attach(ctx), invitationID, actorAccountID))
}

func (e engine) RevokeSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	if e.invitations == nil {
		return errSchoolInvitationUnavailable
	}
	return invitationError(e.invitations.RevokeInvitation(e.attach(ctx), invitationID, actorAccountID))
}

func (e engine) ListPendingSchoolInvitations(ctx context.Context) ([]identityaccess.SchoolInvitation, error) {
	if e.invitations == nil {
		return nil, errSchoolInvitationUnavailable
	}
	invitations, err := e.invitations.ListPendingInvitations(e.attach(ctx))
	if err != nil {
		return nil, invitationError(err)
	}
	result := make([]identityaccess.SchoolInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, PublicSchoolInvitation(invitation))
	}
	return result, nil
}

func (e engine) RevokeTenantSchoolInvitations(ctx context.Context, tenantID int64) (int, error) {
	revoked, err := e.invitationMaintenance.RevokeTenantInvitations(e.attach(ctx), tenantID)
	return revoked, invitationError(err)
}

func (e engine) DeleteExpiredSchoolInvitations(ctx context.Context) (int, error) {
	deleted, err := e.invitationMaintenance.CleanupExpiredInvitations(e.attach(ctx))
	return deleted, invitationError(err)
}

func (e engine) RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	if e.invitations == nil {
		return errSchoolInvitationUnavailable
	}
	return invitationError(e.invitations.RecordInvitationDelivery(e.attach(ctx), id, domain.TokenDelivery(delivery)))
}

func (e engine) SchoolInvitationDeliverySent(ctx context.Context, id int64) (bool, error) {
	if e.invitations == nil {
		return false, errSchoolInvitationUnavailable
	}
	sent, err := e.invitations.InvitationDeliverySent(e.attach(ctx), id)
	return sent, invitationError(err)
}

func (e engine) SchoolInvitationSubdomain(ctx context.Context, token string) string {
	if e.invitations == nil {
		return ""
	}
	return e.invitations.InvitationSubdomain(e.attach(ctx), token)
}

var invitationSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrInvitationNotFound, identityaccess.ErrInvitationNotFound},
	{domain.ErrInvitationExpired, identityaccess.ErrInvitationExpired},
	{domain.ErrInvitationUsed, identityaccess.ErrInvitationUsed},
	{domain.ErrInvitationDeliveryFailed, identityaccess.ErrInvitationDeliveryFailed},
	{domain.ErrInvitationTenantDeleted, identityaccess.ErrInvitationTenantDeleted},
	{domain.ErrInvitationNameRequired, identityaccess.ErrInvitationNameRequired},
	{domain.ErrInvitationOwnerRequired, identityaccess.ErrInvitationOwnerRequired},
	{domain.ErrInvitationOwnerMismatch, identityaccess.ErrInvitationOwnerMismatch},
	{domain.ErrAccountAlreadyHasTenantAccess, identityaccess.ErrAccountAlreadyHasTenantAccess},
	{domain.ErrPasswordMismatch, identityaccess.ErrInvitationPasswordMismatch},
	{domain.ErrRoleGrantNotPermitted, identityaccess.ErrRoleGrantNotPermitted},
	{domain.ErrLehrkraftNoCaregiver, identityaccess.ErrLehrkraftNoCaregiver},
	{domain.ErrEmailAlreadyExists, identityaccess.ErrEmailAlreadyExists},
	{domain.ErrRoleNotFound, identityaccess.ErrRoleNotAssignable},
	{domain.ErrTransactionUnusable, identityaccess.ErrTransactionUnusable},
}

// invitationError translates a flow error to the public contract: the
// operation envelope keeps its text, an invitation sentinel gains its public
// twin, and everything else falls through to the lifecycle mapping, which
// carries the role-policy and account sentinels the flows share.
func invitationError(err error) error {
	if err == nil {
		return nil
	}
	var operation *OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: invitationError(operation.Err)}
	}
	for _, sentinel := range invitationSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return lifecycleError(err)
}
