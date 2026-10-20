package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// CreateInvitation stores the invitation and its mail in the same transaction.
// The worker sends only committed entries: staff import may create invitations
// mid-transaction, and a rolled-back link must never reach an inbox.
func (s *SchoolInvitation) CreateInvitation(ctx context.Context, request domain.SchoolInvitationRequest) (result domain.SchoolInvitation, err error) {
	tenantID := request.TenantID
	if tenantID == 0 {
		tenantID = s.runtime.TenantID(ctx)
	}
	if tenantID <= 0 {
		return result, failed(opCreateInvitation, domain.ErrTenantRequired)
	}
	err = s.runtime.RunInTx(s.runtime.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
		result, err = s.createInvitation(txCtx, request)
		return err
	})
	return result, err
}

func (s *SchoolInvitation) ListPendingInvitations(ctx context.Context) (value0 []domain.SchoolInvitation, err error) {
	err = s.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitListPendingInvitations(txCtx)
		return err
	})
	return
}

func (s *SchoolInvitation) RevokeInvitation(ctx context.Context, invitationID, actorAccountID int64) (err error) {
	err = s.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = s.inUnitRevokeInvitation(txCtx, invitationID, actorAccountID)
		return err
	})
	return
}

func (s *SchoolInvitation) InvitationDeliverySent(ctx context.Context, id int64) (value0 bool, err error) {
	err = s.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitInvitationDeliverySent(txCtx, id)
		return err
	})
	return
}

// FindFormerEmployment answers for the school in context, before any
// invitation exists, which former employment accepting one for the address
// would continue. An address without an account answers like one without a
// former employment here. It reads on the caller's school unit (the request
// transaction) and starts none of its own.
func (s *SchoolInvitation) findFormerEmployment(ctx context.Context, email string) (domain.FormerEmployment, bool, error) {
	address, err := invitationAddress(email)
	if err != nil {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, err)
	}
	if s.runtime.TenantID(ctx) <= 0 {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, domain.ErrTenantRequired)
	}
	if !s.runtime.HasTransaction(ctx) || s.runtime.IsAdminTx(ctx) {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, errSchoolUnitRequired)
	}
	account, exists, _, err := s.logins.FindLoginAccountByEmail(ctx, address)
	if err != nil {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, err)
	}
	if !exists {
		return domain.FormerEmployment{}, false, nil
	}
	former, found, err := s.identity.FindFormerEmployment(ctx, account.ID)
	if err != nil {
		return domain.FormerEmployment{}, false, failed(opFindFormerEmployment, err)
	}
	return former, found, nil
}

// errSchoolUnitRequired refuses a former-employment question outside a school
// transaction: the person and membership reads rely on its RLS scope.
var errSchoolUnitRequired = errors.New("former employment lookup requires the school's transaction")

// invitationAddress is the normalized address an invitation names.
func invitationAddress(raw string) (string, error) {
	email := strings.TrimSpace(strings.ToLower(raw))
	switch {
	case email == "":
		return "", fmt.Errorf("email is required")
	case len(email) > maxInvitationEmailLength:
		return "", fmt.Errorf("email address too long")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return "", fmt.Errorf("invalid email address")
	}
	return parsed.Address, nil
}

func (s *SchoolInvitation) CreateSchoolInvitation(ctx context.Context, request identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error) {
	if s == nil {
		return identityaccess.SchoolInvitation{}, errSchoolInvitationUnavailable
	}
	invitation, err := s.CreateInvitation(bindRuntime(s.attach, ctx), domain.SchoolInvitationRequest{
		Email: request.Email, RoleID: request.RoleID, TenantID: request.TenantID, FirstName: request.FirstName,
		LastName: request.LastName, Position: request.Position, CaregiverEnabled: request.CaregiverEnabled,
		EmploymentAction: domain.EmploymentAction(request.EmploymentAction),
		PersonID:         request.PersonID, CreatedBy: request.CreatedBy, SchoolName: request.SchoolName,
		ActorPermissions: request.ActorPermissions, OperatorGrant: request.OperatorGrant,
	})
	return PublicSchoolInvitation(invitation), invitationError(err)
}

func (s *SchoolInvitation) ValidateSchoolInvitation(ctx context.Context, token string) (identityaccess.InvitationPreview, error) {
	if s == nil {
		return identityaccess.InvitationPreview{}, errSchoolInvitationUnavailable
	}
	preview, err := s.ValidateInvitation(bindRuntime(s.attach, ctx), token)
	return identityaccess.InvitationPreview{
		Portal: identityaccess.InvitationPortal(preview.Portal), RequiresAccountLogin: preview.RequiresAccountLogin,
		Email: preview.Email, RoleName: preview.RoleName, FirstName: preview.FirstName, LastName: preview.LastName,
		Position: preview.Position, CaregiverEnabled: preview.CaregiverEnabled, ExpiresAt: preview.ExpiresAt,
	}, invitationError(err)
}

func (s *SchoolInvitation) FindFormerEmployment(ctx context.Context, email string) (identityaccess.FormerEmployment, bool, error) {
	if s == nil {
		return identityaccess.FormerEmployment{}, false, errSchoolInvitationUnavailable
	}
	former, found, err := s.findFormerEmployment(bindRuntime(s.attach, ctx), email)
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

func (s *SchoolInvitation) AcceptSchoolInvitation(ctx context.Context, token string, registration identityaccess.InvitationRegistration) (identityaccess.Account, error) {
	if s == nil {
		return identityaccess.Account{}, errSchoolInvitationUnavailable
	}
	account, err := s.AcceptInvitation(bindRuntime(s.attach, ctx), token, domain.InvitationRegistration{
		OwnerAccessToken: registration.OwnerAccessToken, FirstName: registration.FirstName, LastName: registration.LastName,
		Password: registration.Password, ConfirmPassword: registration.ConfirmPassword,
	})
	return identityaccess.Account{ID: account.ID, Email: account.Email}, invitationError(err)
}

func (s *SchoolInvitation) ResendSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	if s == nil {
		return errSchoolInvitationUnavailable
	}
	return invitationError(s.ResendInvitation(bindRuntime(s.attach, ctx), invitationID, actorAccountID))
}

func (s *SchoolInvitation) RevokeSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	if s == nil {
		return errSchoolInvitationUnavailable
	}
	return invitationError(s.RevokeInvitation(bindRuntime(s.attach, ctx), invitationID, actorAccountID))
}

func (s *SchoolInvitation) ListPendingSchoolInvitations(ctx context.Context) ([]identityaccess.SchoolInvitation, error) {
	if s == nil {
		return nil, errSchoolInvitationUnavailable
	}
	invitations, err := s.ListPendingInvitations(bindRuntime(s.attach, ctx))
	if err != nil {
		return nil, invitationError(err)
	}
	result := make([]identityaccess.SchoolInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, PublicSchoolInvitation(invitation))
	}
	return result, nil
}

func (s *SchoolInvitation) RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	if s == nil {
		return errSchoolInvitationUnavailable
	}
	return invitationError(s.RecordInvitationDelivery(bindRuntime(s.attach, ctx), id, domain.TokenDelivery(delivery)))
}

func (s *SchoolInvitation) SchoolInvitationDeliverySent(ctx context.Context, id int64) (bool, error) {
	if s == nil {
		return false, errSchoolInvitationUnavailable
	}
	sent, err := s.InvitationDeliverySent(bindRuntime(s.attach, ctx), id)
	return sent, invitationError(err)
}

func (s *SchoolInvitation) SchoolInvitationSubdomain(ctx context.Context, token string) string {
	if s == nil {
		return ""
	}
	return s.InvitationSubdomain(bindRuntime(s.attach, ctx), token)
}

func PublicSchoolInvitation(invitation domain.SchoolInvitation) identityaccess.SchoolInvitation {
	return identityaccess.SchoolInvitation{
		ID: invitation.ID, TenantID: invitation.TenantID, Email: invitation.Email, Token: invitation.Token,
		RoleID: invitation.RoleID, RoleName: invitation.RoleName, ExpiresAt: invitation.ExpiresAt, UsedAt: invitation.UsedAt,
		CreatedBy: invitation.CreatedBy, CreatorEmail: invitation.CreatorEmail, FirstName: invitation.FirstName, LastName: invitation.LastName,
		EmploymentAction: identityaccess.EmploymentAction(invitation.EmploymentAction),
		Position:         invitation.Position, CaregiverEnabled: invitation.CaregiverEnabled, PersonID: invitation.PersonID,
		Delivery: identityaccess.TokenDelivery(invitation.Delivery), CreatedAt: invitation.CreatedAt,
	}
}

var errSchoolInvitationUnavailable = identityaccess.ErrSchoolInvitationUnavailable

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
