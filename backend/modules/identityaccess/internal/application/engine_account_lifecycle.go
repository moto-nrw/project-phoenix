package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

var errAccountLifecycleUnavailable = identityaccess.ErrAccountLifecycleUnavailable

func roleFacts(role *identityaccess.RoleFacts) *domain.RoleFacts {
	if role == nil {
		return nil
	}
	facts := domain.RoleFacts(*role)
	return &facts
}

func (e engine) StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, previousToken, ipAddress, userAgent string) (*identityaccess.StaffPreviewSession, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	session, err := e.lifecycle.StartStaffPreview(e.attach(ctx), adminAccountID, tenantID, targetAccountID, previousToken, ipAddress, userAgent)
	if err != nil {
		return nil, lifecycleError(err)
	}
	result := identityaccess.StaffPreviewSession(*session)
	return &result, nil
}

func (e engine) EndStaffPreview(ctx context.Context, previewToken, ipAddress, userAgent string) (int64, error) {
	if e.lifecycle == nil {
		return 0, errAccountLifecycleUnavailable
	}
	accountID, err := e.lifecycle.EndStaffPreview(e.attach(ctx), previewToken, ipAddress, userAgent)
	return accountID, lifecycleError(err)
}

func (e engine) ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) (value0 []identityaccess.StaffPreviewCandidate, err error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) ([]identityaccess.StaffPreviewCandidate, error) {
		candidates, err := e.lifecycle.ListStaffPreviewCandidates(e.attach(ctx), tenantID, excludeAccountID)
		if err != nil {
			return nil, lifecycleError(err)
		}
		result := make([]identityaccess.StaffPreviewCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			result = append(result, identityaccess.StaffPreviewCandidate(candidate))
		}
		return result, nil
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) PreviewStaffOffboarding(ctx context.Context, accountID int64) (identityaccess.StaffOffboardingPreview, error) {
	if e.lifecycle == nil {
		return identityaccess.StaffOffboardingPreview{}, errAccountLifecycleUnavailable
	}
	preview, err := e.lifecycle.PreviewStaffOffboarding(e.attach(ctx), accountID)
	return identityaccess.StaffOffboardingPreview(preview), lifecycleError(err)
}

func (e engine) ExecuteStaffOffboarding(ctx context.Context, accountID int64, revision string) (identityaccess.StaffOffboardingResult, error) {
	if e.lifecycle == nil {
		return identityaccess.StaffOffboardingResult{}, errAccountLifecycleUnavailable
	}
	result, err := e.lifecycle.ExecuteStaffOffboarding(e.attach(ctx), accountID, revision)
	return identityaccess.StaffOffboardingResult(result), lifecycleError(err)
}

func (e engine) EnsureSchoolIdentity(ctx context.Context, input identityaccess.SchoolIdentityInput) (value0 *identityaccess.SchoolIdentity, err error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) (*identityaccess.SchoolIdentity, error) {
		identity, err := e.lifecycle.EnsureSchoolIdentity(e.attach(ctx), domain.SchoolIdentityInput{
			AccountID: input.AccountID, TenantID: input.TenantID, Role: roleFacts(input.Role),
			FirstName: input.FirstName, LastName: input.LastName, TagID: input.TagID, PersonID: input.PersonID,
			Position: input.Position, CaregiverUpgrade: input.CaregiverUpgrade, CreatePerson: input.CreatePerson,
		})
		if err != nil {
			return nil, lifecycleError(err)
		}
		if identity == nil {
			return nil, nil
		}
		result := identityaccess.SchoolIdentity(*identity)
		return &result, nil
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	if e.lifecycle == nil {
		return false, errAccountLifecycleUnavailable
	}
	hasProfile, err := e.lifecycle.profiles.HasLiveCaregiverProfile(e.attach(ctx), accountID)
	return hasProfile, lifecycleError(err)
}

func parentAccount(value domain.ParentAccount) identityaccess.ParentAccount {
	return identityaccess.ParentAccount{
		ID: value.ID, TenantID: value.TenantID, Email: value.Email, Username: value.Username, Active: value.Active,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func (e engine) CreateParentAccount(ctx context.Context, email, username, password string) (value0 identityaccess.ParentAccount, err error) {
	if e.lifecycle == nil {
		return identityaccess.ParentAccount{}, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.ParentAccount, error) {
		account, err := e.lifecycle.CreateParentAccount(e.attach(ctx), email, username, password)
		return parentAccount(account), lifecycleError(err)
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) GetParentAccountByID(ctx context.Context, id int64) (value0 identityaccess.ParentAccount, err error) {
	if e.lifecycle == nil {
		return identityaccess.ParentAccount{}, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.ParentAccount, error) {
		account, err := e.lifecycle.GetParentAccountByID(e.attach(ctx), id)
		return parentAccount(account), lifecycleError(err)
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) GetParentAccountByEmail(ctx context.Context, email string) (value0 identityaccess.ParentAccount, err error) {
	if e.lifecycle == nil {
		return identityaccess.ParentAccount{}, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.ParentAccount, error) {
		account, err := e.lifecycle.GetParentAccountByEmail(e.attach(ctx), email)
		return parentAccount(account), lifecycleError(err)
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) UpdateParentAccount(ctx context.Context, account identityaccess.ParentAccount) (err error) {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) error {
		return lifecycleError(e.lifecycle.UpdateParentAccount(e.attach(ctx), domain.ParentAccount{
			ID: account.ID, TenantID: account.TenantID, Email: account.Email, Username: account.Username, Active: account.Active,
		}))
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

func (e engine) ActivateParentAccount(ctx context.Context, id int64) (err error) {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) error {
		return lifecycleError(e.lifecycle.ActivateParentAccount(e.attach(ctx), id))
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

func (e engine) DeactivateParentAccount(ctx context.Context, id int64) (err error) {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) error {
		return lifecycleError(e.lifecycle.DeactivateParentAccount(e.attach(ctx), id))
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

func (e engine) ListParentAccounts(ctx context.Context, filter identityaccess.ParentAccountFilter) (value0 []identityaccess.ParentAccount, err error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) ([]identityaccess.ParentAccount, error) {
		accounts, err := e.lifecycle.ListParentAccounts(e.attach(ctx), domain.ParentAccountFilter(filter))
		if err != nil {
			return nil, lifecycleError(err)
		}
		result := make([]identityaccess.ParentAccount, 0, len(accounts))
		for _, account := range accounts {
			result = append(result, parentAccount(account))
		}
		return result, nil
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) InviteToStudent(ctx context.Context, req identityaccess.InviteToStudentRequest) (value0 *identityaccess.InviteToStudentResult, err error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) (*identityaccess.InviteToStudentResult, error) {
		result, err := e.lifecycle.InviteToStudent(e.attach(ctx), domain.InviteToStudentRequest(req))
		if err != nil {
			return nil, lifecycleError(err)
		}
		public := identityaccess.InviteToStudentResult{
			Outcome: identityaccess.InviteToStudentOutcome(result.Outcome), GuardianProfileID: result.GuardianProfileID,
			InvitationID: result.InvitationID, ExistingRole: result.ExistingRole,
		}
		return &public, nil
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) ApproveInvitation(ctx context.Context, invitationID, approverAccountID int64) (err error) {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) error {
		return lifecycleError(e.lifecycle.ApproveInvitation(e.attach(ctx), invitationID, approverAccountID))
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

func (e engine) RejectInvitation(ctx context.Context, invitationID, approverAccountID int64) (err error) {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) error {
		return lifecycleError(e.lifecycle.RejectInvitation(e.attach(ctx), invitationID, approverAccountID))
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

func (e engine) CreateGuardianInvitation(ctx context.Context, request identityaccess.GuardianInvitationRequest) (value0 identityaccess.GuardianInvitation, err error) {
	if e.lifecycle == nil {
		return identityaccess.GuardianInvitation{}, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.GuardianInvitation, error) {
		invitation, err := e.lifecycle.CreateGuardianInvitation(e.attach(ctx), domain.GuardianInvitationRequest(request))
		if err != nil {
			return identityaccess.GuardianInvitation{}, invitationError(err)
		}
		return identityaccess.GuardianInvitation(invitation), nil
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) ValidateGuardianInvitation(ctx context.Context, token string) (identityaccess.GuardianInvitationPreview, error) {
	if e.lifecycle == nil {
		return identityaccess.GuardianInvitationPreview{}, errAccountLifecycleUnavailable
	}
	preview, err := e.lifecycle.ValidateGuardianInvitation(e.attach(ctx), token)
	if err != nil {
		return identityaccess.GuardianInvitationPreview{}, invitationError(err)
	}
	return identityaccess.GuardianInvitationPreview(preview), nil
}

func (e engine) AcceptGuardianInvitation(ctx context.Context, token string, registration identityaccess.GuardianRegistration) (identityaccess.Account, error) {
	if e.lifecycle == nil {
		return identityaccess.Account{}, errAccountLifecycleUnavailable
	}
	account, err := e.lifecycle.AcceptGuardianInvitation(e.attach(ctx), token, domain.GuardianRegistration(registration))
	if err != nil {
		return identityaccess.Account{}, invitationError(err)
	}
	return identityaccess.Account{ID: account.ID, Email: account.Email}, nil
}

func (e engine) ResendGuardianInvitation(ctx context.Context, invitationID, actorAccountID int64) (err error) {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) error {
		return invitationError(e.lifecycle.ResendGuardianInvitation(e.attach(ctx), invitationID, actorAccountID))
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

func (e engine) ListGuardianInvitations(ctx context.Context, guardianProfileID int64) ([]identityaccess.GuardianInvitation, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	return NewGuardianInvitationReads(e.lifecycle.invitations, e.lifecycle.sessions.tx.RunRead).ListGuardianInvitations(e.attach(ctx), guardianProfileID)
}

func (e engine) ListOpenGuardianInvitations(ctx context.Context, guardianProfileIDs []int64) ([]identityaccess.GuardianInvitation, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	return NewGuardianInvitationReads(e.lifecycle.invitations, e.lifecycle.sessions.tx.RunRead).ListOpenGuardianInvitations(e.attach(ctx), guardianProfileIDs)
}

func (e engine) ListRedeemableGuardianInvitations(ctx context.Context) ([]identityaccess.GuardianInvitation, error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	return NewGuardianInvitationReads(e.lifecycle.invitations, e.lifecycle.sessions.tx.RunRead).ListRedeemableGuardianInvitations(e.attach(ctx))
}

func publicGuardianInvitations(invitations []domain.GuardianInvitation) []identityaccess.GuardianInvitation {
	if invitations == nil {
		return nil
	}
	result := make([]identityaccess.GuardianInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, identityaccess.GuardianInvitation(invitation))
	}
	return result
}

func (e engine) GuardianInvitationSchoolSlug(ctx context.Context, token string) string {
	if e.lifecycle == nil {
		return ""
	}
	return e.lifecycle.GuardianInvitationSchoolSlug(e.attach(ctx), token)
}

func (e engine) PendingInvitationStudentID(ctx context.Context, invitationID int64) (value0 int64, err error) {
	if e.lifecycle == nil {
		return 0, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) (int64, error) {
		studentID, err := e.lifecycle.PendingInvitationStudentID(e.attach(ctx), invitationID)
		return studentID, lifecycleError(err)
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) ListPendingApprovalsDetailed(ctx context.Context) (value0 []identityaccess.PendingApprovalView, err error) {
	if e.lifecycle == nil {
		return nil, errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) ([]identityaccess.PendingApprovalView, error) {
		views, err := e.lifecycle.ListPendingApprovalsDetailed(e.attach(ctx))
		if err != nil {
			return nil, lifecycleError(err)
		}
		result := make([]identityaccess.PendingApprovalView, 0, len(views))
		for _, view := range views {
			result = append(result, identityaccess.PendingApprovalView(view))
		}
		return result, nil
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) RevokeAccess(ctx context.Context, req identityaccess.RevokeAccessRequest) (err error) {
	if e.lifecycle == nil {
		return errAccountLifecycleUnavailable
	}
	operation := func(ctx context.Context) error {
		return lifecycleError(e.lifecycle.RevokeAccess(e.attach(ctx), domain.RevokeAccessRequest(req)))
	}
	err = e.lifecycle.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

var lifecycleSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrPreviewSelf, identityaccess.ErrPreviewSelf},
	{domain.ErrPreviewTargetNotStaff, identityaccess.ErrPreviewTargetNotStaff},
	{domain.ErrPreviewTokenInvalid, identityaccess.ErrPreviewTokenInvalid},
	{domain.ErrStaffOffboardingConflict, identityaccess.ErrStaffOffboardingConflict},
	{domain.ErrSchoolIdentityNamesRequired, identityaccess.ErrSchoolIdentityNamesRequired},
	{domain.ErrSchoolIdentityPersonIsStudent, identityaccess.ErrSchoolIdentityPersonIsStudent},
	{domain.ErrSchoolIdentityTagUnknown, identityaccess.ErrSchoolIdentityTagUnknown},
	{domain.ErrSchoolIdentityTagConflict, identityaccess.ErrSchoolIdentityTagConflict},
	{domain.ErrSchoolIdentityTagTaken, identityaccess.ErrSchoolIdentityTagTaken},
	{domain.ErrParentAccountNotFound, identityaccess.ErrParentAccountNotFound},
	{domain.ErrEmailAlreadyExists, identityaccess.ErrEmailAlreadyExists},
	{domain.ErrUsernameAlreadyExists, identityaccess.ErrUsernameAlreadyExists},
	{domain.ErrCannotRemovePrimaryGuardian, identityaccess.ErrCannotRemovePrimaryGuardian},
	{domain.ErrCannotRemoveStaffManagedGuardian, identityaccess.ErrCannotRemoveStaffManagedGuardian},
	{domain.ErrCannotRemoveOwnAccess, identityaccess.ErrCannotRemoveOwnAccess},
	{domain.ErrCannotRemovePayerGuardian, identityaccess.ErrCannotRemovePayerGuardian},
	{domain.ErrInviteSocialWorkerManaged, identityaccess.ErrInviteSocialWorkerManaged},
	{domain.ErrGuardianInvitationNotFound, identityaccess.ErrGuardianInvitationNotFound},
	{domain.ErrGuardianInvitationExpired, identityaccess.ErrGuardianInvitationExpired},
}
