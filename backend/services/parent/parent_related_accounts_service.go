package parent

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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
	// ErrInvalidInviteInput means the invite request failed input validation.
	ErrInvalidInviteInput = errors.New("parent: invalid related-account invite input")
)

// RelatedAccountStatus describes a linked guardian's portal-access state.
type RelatedAccountStatus string

const (
	// RelatedAccountActive: the guardian has a portal account (login).
	RelatedAccountActive RelatedAccountStatus = "active"
	// RelatedAccountPending: linked but no account yet (invite outstanding).
	RelatedAccountPending RelatedAccountStatus = "pending"
	// RelatedAccountNoAccount: a staff-maintained contact link without portal
	// access and without an open invite.
	RelatedAccountNoAccount RelatedAccountStatus = "no_account"
	// RelatedAccountActiveNoAccess: the guardian has a portal account, but
	// this child's link carries no parent_portal.access permission (e.g. a
	// restrictive contact role) — the child is invisible to them (#2172).
	RelatedAccountActiveNoAccess RelatedAccountStatus = "active_no_access"
)

// RelatedAccount is one guardian linked to a child, with portal-access status.
type RelatedAccount struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
	Email             string
	RelationshipType  string
	// GuardianRole is the per-child role preset on the link. The UI uses it to
	// hide the grant-access action for school-managed social-worker contacts,
	// which the invite flow refuses anyway (#2172).
	GuardianRole string
	IsPrimary    bool
	Status       RelatedAccountStatus
	// IsSelf marks the row belonging to the requesting parent's own account.
	// The backend rejects self-removal (ErrCannotRemoveOwnAccess), so the UI
	// uses this to hide the remove action on the caller's own row.
	IsSelf bool
}

// InviteRelatedAccountResult mirrors the invitation service outcome.
type InviteRelatedAccountResult struct {
	Outcome           string
	GuardianProfileID int64
	// ExistingRole carries the current guardian role for the
	// existing_contact_restricted outcome; empty otherwise.
	ExistingRole string
}

// ListRelatedAccounts returns every guardian linked to the child, annotated
// with portal-access status. Ownership of the child is verified first.
func (s *service) ListRelatedAccounts(ctx context.Context, accountID, studentID int64) ([]*RelatedAccount, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	var out []*RelatedAccount
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		links, err := s.StudentGuardianRepo.FindByStudentID(txCtx, studentID)
		if err != nil {
			return err
		}
		out = make([]*RelatedAccount, 0, len(links))
		for _, link := range links {
			profile, err := s.GuardianProfileRepo.FindByID(txCtx, link.GuardianProfileID)
			if err != nil {
				return err
			}
			if profile == nil {
				continue
			}
			status := s.relatedAccountStatus(txCtx, profile, link, studentID)
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
				GuardianRole:      link.GuardianRole,
				IsPrimary:         link.IsPrimary,
				Status:            status,
				IsSelf:            profile.AccountID != nil && *profile.AccountID == accountID,
			})
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return out, nil
}

func (s *service) relatedAccountStatus(ctx context.Context, profile *userModels.GuardianProfile, link *userModels.StudentGuardian, studentID int64) RelatedAccountStatus {
	hasAccess := authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess)
	if profile.HasAccount && hasAccess {
		return RelatedAccountActive
	}
	// An open invitation (e.g. a confirmed upgrade awaiting staff approval)
	// wins over the stale account states below.
	if s.GuardianInviteRepo != nil {
		invitations, err := s.GuardianInviteRepo.FindByGuardianProfileID(ctx, profile.ID)
		if err == nil {
			now := time.Now()
			for _, inv := range invitations {
				if inv.StudentID == nil || *inv.StudentID != studentID {
					continue
				}
				if authService.GuardianInvitationValid(inv, now) && inv.ApprovalStatus != authModels.GuardianInvitationApprovalRejected {
					return RelatedAccountPending
				}
			}
		}
	}
	if profile.HasAccount {
		return RelatedAccountActiveNoAccess
	}
	return RelatedAccountNoAccount
}

// InviteRelatedAccount invites a further guardian to the parent's child by
// email. Gated by guardians.parent_invite_mode; staff_approval mode queues the
// request instead of acting immediately.
func (s *service) InviteRelatedAccount(ctx context.Context, accountID, studentID int64, email, firstName, lastName string, confirmRoleUpgrade bool) (*InviteRelatedAccountResult, error) {
	if strings.TrimSpace(email) == "" {
		return nil, ErrEmailRequired
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(email)); err != nil {
		return nil, fmt.Errorf("%w: invalid email", ErrInvalidInviteInput)
	}
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487). Inviting a
	// further guardian account is such a change, and ChildFeatures already
	// switches the button off — this is the half that also holds for a direct
	// API call.
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}

	mode, err := s.Settings.ResolveStringForTenant(ctx, child.tenantID, configModels.KeyGuardianParentInviteMode)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve invite mode: %w", err)
	}
	if mode == configModels.ParentInviteModeDisabled {
		return nil, ErrInviteDisabled
	}
	requireApproval := mode == configModels.ParentInviteModeStaffApproval

	requestedBy := accountID
	var result *authService.InviteToStudentResult
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		res, inviteErr := s.GuardianInvites.InviteToStudent(txCtx, authService.InviteToStudentRequest{
			StudentID:                  studentID,
			Email:                      email,
			FirstName:                  firstName,
			LastName:                   lastName,
			CreatedBy:                  accountID,
			RequestedByParentAccountID: &requestedBy,
			RequireApproval:            requireApproval,
			ConfirmRoleUpgrade:         confirmRoleUpgrade,
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
		ExistingRole:      result.ExistingRole,
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
	// Removing a guardian account is a change like any other (#2487).
	if err := child.requireCareRunning(); err != nil {
		return err
	}

	mode, err := s.Settings.ResolveStringForTenant(ctx, child.tenantID, configModels.KeyGuardianParentInviteMode)
	if err != nil {
		return fmt.Errorf("parent: resolve invite mode: %w", err)
	}
	if mode == configModels.ParentInviteModeDisabled {
		return ErrRemoveDisabled
	}
	canRemove, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyGuardianParentCanRemove)
	if err != nil {
		return fmt.Errorf("parent: resolve remove setting: %w", err)
	}
	if !canRemove {
		return ErrRemoveDisabled
	}

	return tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.GuardianInvites.RevokeAccess(txCtx, authService.RevokeAccessRequest{
			StudentID:         studentID,
			GuardianProfileID: guardianProfileID,
			ActorAccountID:    accountID,
			ByParent:          true,
		})
	})
}
