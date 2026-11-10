package care

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// Every parent write for an existing child commits only while the guardian
// still holds what the check before its unit of work found (#6529): an active
// membership of the child's school, the relationship to the child bound to
// the account, parent_portal.access and every action permission of the
// write. The check before the unit of work ends with its own transaction, or,
// bundled (ADR 0161), reads without locks; a revocation committed since then
// must refuse the write, and one that comes later must wait for it.
//
// guardChildWrite is the one guard that holds these facts. It runs inside the
// write's school transaction before the first owner write and locks in the
// order Konto → Schule → Kind → Profil → Beziehung:
//
//  1. the live account FOR KEY SHARE, then its active mapping at the school
//     FOR SHARE through Identity & Access: a deactivation or deletion waits
//     for the write, without cycling with a later audit account FK check;
//  2. the child row, the lock every parent write and care exit of the child
//     takes first;
//  3. for writes of the guardian's own contact rows the guardian profile,
//     the lock order of every contact write of this package;
//  4. the relationship row, FOR UPDATE through People Directory: every change
//     of its permissions, role or existence locks or deletes it first.
//
// Invitations first take both participating accounts FOR UPDATE in ID order;
// otherwise an invite could hold the child while waiting for the invitee's
// account, whose own write holds that account and waits for the child.
//
// The relationship must still name the account's guardian profile; the
// profile cannot change its account while a relationship holds it, so the
// locked relationship also binds the account. Whether the child's care still
// runs is a separate rule of each write (RequireCareRunningForUpdate), so a
// write the school allows after the care ended keeps its own decision.
type ChildWriteAccess struct {
	// InviteEmail takes both invitation accounts before the normal guard locks.
	InviteEmail string
	// Permissions are the action permissions of the write; portal access is
	// always required.
	Permissions []string
	// OwnProfile locks the guardian's own profile before the relationship,
	// for writes that change the guardian's contact rows.
	OwnProfile bool
	// Profiles are further guardian profiles the write locks and changes,
	// locked before the relationship in ascending id order. A profile that
	// does not exist is left to the write, which reports it in its own terms.
	Profiles []int64
	// ChildMissing names the write's refusal when the child row cannot be
	// locked, e.g. because it was deleted; nil returns the lock error.
	ChildMissing func(ctx context.Context, err error) error
}

// childWriteNeeds is the access of a write that changes no contact row of
// the guardian.
func childWriteNeeds(permissions ...string) ChildWriteAccess {
	return ChildWriteAccess{Permissions: permissions}
}

// SchoolMembershipLock is the port to Identity & Access that pins an
// account's membership of the school in context until the caller's
// transaction ends. It runs inside the caller's school transaction.
type SchoolMembershipLock func(ctx context.Context, accountID int64) (bool, error)

// InvitationAccountLock pins inviter and invitee accounts in ID order before
// the guard takes the membership and child locks, in the same transaction.
type InvitationAccountLock func(ctx context.Context, accountID int64, email string) error

// ErrChildWriteGuardUnavailable reports a service composed without the
// membership, child, profile or relationship port the guard needs; parent
// writes fail closed without them.
var ErrChildWriteGuardUnavailable = errors.New("parent: child write guard is not composed")

// WithChildWrite opens the child's school unit for a parent write and runs
// guardChildWrite before write. child is the result of the check before the
// unit (ResolvePermittedChild). It is the only way a parent write for an
// existing child opens its unit; WithPermittedChildForWrite runs the same
// guard in its bundled unit.
func (s *Service) WithChildWrite(
	ctx context.Context,
	accountID, studentID int64,
	child *Child,
	access ChildWriteAccess,
	write func(ctx context.Context) error,
) error {
	if child == nil {
		return ErrChildNotLinked
	}
	if write == nil {
		return fmt.Errorf("parent: child write is required")
	}
	return s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		if err := s.guardChildWrite(txCtx, accountID, studentID, child, access); err != nil {
			return err
		}
		return write(txCtx)
	})
}

// guardChildWrite repeats the relationship check of ResolvePermittedChild in
// the write's school transaction, with every fact locked until commit.
func (s *Service) guardChildWrite(ctx context.Context, accountID, studentID int64, child *Child, access ChildWriteAccess) error {
	if accountID <= 0 || studentID <= 0 || child == nil || child.GuardianProfileID <= 0 {
		return ErrChildNotLinked
	}
	if s.Memberships == nil || s.StudentRepo == nil || s.GuardianProfileRepo == nil || s.StudentGuardianRepo == nil {
		return ErrChildWriteGuardUnavailable
	}
	if access.InviteEmail != "" {
		if s.InvitationAccounts == nil {
			return ErrChildWriteGuardUnavailable
		}
		if err := s.InvitationAccounts(ctx, accountID, access.InviteEmail); err != nil {
			return err
		}
	}
	if err := s.lockSchoolMembership(ctx, accountID); err != nil {
		return err
	}
	if _, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID); err != nil {
		if access.ChildMissing != nil {
			return access.ChildMissing(ctx, err)
		}
		return err
	}
	if err := s.lockWriteProfiles(ctx, child.GuardianProfileID, access); err != nil {
		return err
	}
	link, err := s.lockAccountRelationship(ctx, accountID, studentID, child.GuardianProfileID)
	if err != nil {
		return err
	}
	return s.requireWritePermissions(link, access)
}

// lockSchoolMembership pins the account's active mapping at the school.
func (s *Service) lockSchoolMembership(ctx context.Context, accountID int64) error {
	active, err := s.Memberships(ctx, accountID)
	if err != nil {
		return fmt.Errorf("parent: lock school membership: %w", err)
	}
	if !active {
		return ErrChildNotLinked
	}
	return nil
}

// lockAccountRelationship locks the relationship and checks that its
// guardian profile still belongs to the account.
func (s *Service) lockAccountRelationship(ctx context.Context, accountID, studentID, guardianProfileID int64) (*GuardianRelationship, error) {
	link, err := s.findChildGuardianLinkForUpdate(ctx, studentID, guardianProfileID)
	if errors.Is(err, ErrGuardianNotLinked) {
		return nil, ErrChildNotLinked
	}
	if err != nil {
		return nil, err
	}
	if err := s.requireProfileOfAccount(ctx, guardianProfileID, accountID); err != nil {
		return nil, err
	}
	return link, nil
}

// requireWritePermissions checks portal access and every action permission
// on the locked relationship.
func (s *Service) requireWritePermissions(link *GuardianRelationship, access ChildWriteAccess) error {
	if !s.guardianHasPermission(link, GuardianPermissionPortalAccess) {
		return ErrChildNotLinked
	}
	for _, permission := range access.Permissions {
		if permission != "" && permission != GuardianPermissionPortalAccess && !s.guardianHasPermission(link, permission) {
			return ErrGuardianPermissionDenied
		}
	}
	return nil
}

// lockWriteProfiles locks the guardian profiles of a write before the
// relationship, the order of every contact write of this package.
func (s *Service) lockWriteProfiles(ctx context.Context, ownProfileID int64, access ChildWriteAccess) error {
	ids := append([]int64(nil), access.Profiles...)
	if access.OwnProfile {
		ids = append(ids, ownProfileID)
	}
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, id)
		switch {
		case err == nil:
		case errors.Is(err, ErrGuardianProfileNotFound) && id == ownProfileID:
			return ErrChildNotLinked
		case errors.Is(err, ErrGuardianProfileNotFound):
			// The write reports a missing contact itself.
		default:
			return err
		}
	}
	return nil
}

// requireProfileOfAccount refuses a relationship whose guardian profile no
// longer belongs to the account the check before the unit resolved.
func (s *Service) requireProfileOfAccount(ctx context.Context, guardianProfileID, accountID int64) error {
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if errors.Is(err, ErrGuardianProfileNotFound) {
		return ErrChildNotLinked
	}
	if err != nil {
		return err
	}
	if profile == nil || profile.AccountID == nil || *profile.AccountID != accountID {
		return ErrChildNotLinked
	}
	return nil
}
