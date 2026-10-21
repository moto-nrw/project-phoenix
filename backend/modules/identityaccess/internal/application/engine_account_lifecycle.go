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
