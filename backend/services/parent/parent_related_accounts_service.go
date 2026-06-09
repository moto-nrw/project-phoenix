package parent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Sentinel errors mapped to stable HTTP codes by the parent handlers.
var (
	// ErrInviteDisabled means guardians.parent_invite_mode is "disabled" for
	// the child's tenant — parents may not invite at this school.
	ErrInviteDisabled = errors.New("parent: inviting further guardians is disabled for this school")
	// ErrRemoveDisabled means guardians.parent_can_remove is off for the
	// child's tenant — parents may not remove other accounts.
	ErrRemoveDisabled = errors.New("parent: removing guardians is disabled for this school")
	// ErrEmailRequired means the invite request carried no email.
	ErrEmailRequired = errors.New("parent: email is required")
)

// RelatedAccountStatus describes a linked guardian's portal-access state.
type RelatedAccountStatus string

const (
	// RelatedAccountActive: the guardian has a portal account (login).
	RelatedAccountActive RelatedAccountStatus = "active"
	// RelatedAccountPending: linked but no account yet (invite outstanding).
	RelatedAccountPending RelatedAccountStatus = "pending"
)

// RelatedAccount is one guardian linked to a child, with portal-access status.
type RelatedAccount struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
	Email             string
	RelationshipType  string
	IsPrimary         bool
	Status            RelatedAccountStatus
}

// InviteRelatedAccountResult mirrors the invitation service outcome.
type InviteRelatedAccountResult struct {
	Outcome           string
	GuardianProfileID int64
}

// ListRelatedAccounts returns every guardian linked to the child, annotated
// with portal-access status. Ownership of the child is verified first.
func (s *service) ListRelatedAccounts(ctx context.Context, accountID, studentID int64) ([]*RelatedAccount, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	var out []*RelatedAccount
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		links, err := s.studentGuardianRepo.FindByStudentID(txCtx, studentID)
		if err != nil {
			return err
		}
		out = make([]*RelatedAccount, 0, len(links))
		for _, link := range links {
			profile, err := s.guardianProfileRepo.FindByID(txCtx, link.GuardianProfileID)
			if err != nil {
				return err
			}
			if profile == nil {
				continue
			}
			status := RelatedAccountPending
			if profile.HasAccount {
				status = RelatedAccountActive
			}
			email := ""
			if profile.Email != nil {
				email = *profile.Email
			}
			out = append(out, &RelatedAccount{
				GuardianProfileID: profile.ID,
				FirstName:         profile.FirstName,
				LastName:          profile.LastName,
				Email:             email,
				RelationshipType:  link.RelationshipType,
				IsPrimary:         link.IsPrimary,
				Status:            status,
			})
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return out, nil
}

// InviteRelatedAccount invites a further guardian to the parent's child by
// email. Gated by guardians.parent_invite_mode; staff_approval mode queues the
// request instead of acting immediately.
func (s *service) InviteRelatedAccount(ctx context.Context, accountID, studentID int64, email, firstName, lastName string) (*InviteRelatedAccountResult, error) {
	if strings.TrimSpace(email) == "" {
		return nil, ErrEmailRequired
	}
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	mode, err := s.settings.ResolveStringForTenant(ctx, child.tenantID, configModels.KeyGuardianParentInviteMode)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve invite mode: %w", err)
	}
	if mode == configModels.ParentInviteModeDisabled {
		return nil, ErrInviteDisabled
	}
	requireApproval := mode == configModels.ParentInviteModeStaffApproval

	requestedBy := accountID
	var result *authService.InviteToStudentResult
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		res, inviteErr := s.guardianInvites.InviteToStudent(txCtx, authService.InviteToStudentRequest{
			StudentID:                  studentID,
			Email:                      email,
			FirstName:                  firstName,
			LastName:                   lastName,
			CreatedBy:                  accountID,
			RequestedByParentAccountID: &requestedBy,
			RequireApproval:            requireApproval,
		})
		if inviteErr != nil {
			return inviteErr
		}
		result = res
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return &InviteRelatedAccountResult{
		Outcome:           string(result.Outcome),
		GuardianProfileID: result.GuardianProfileID,
	}, nil
}

// RemoveRelatedAccount removes another account's access to the parent's child.
// Gated by guardians.parent_can_remove; the primary guardian can never be
// removed by a parent (enforced in the invitation service).
func (s *service) RemoveRelatedAccount(ctx context.Context, accountID, studentID, guardianProfileID int64) error {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return err
	}

	canRemove, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyGuardianParentCanRemove)
	if err != nil {
		return fmt.Errorf("parent: resolve remove setting: %w", err)
	}
	if !canRemove {
		return ErrRemoveDisabled
	}

	return tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.guardianInvites.RevokeAccess(txCtx, authService.RevokeAccessRequest{
			StudentID:         studentID,
			GuardianProfileID: guardianProfileID,
			ActorAccountID:    accountID,
			ByParent:          true,
		})
	})
}
