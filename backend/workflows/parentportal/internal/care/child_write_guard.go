package care

import (
	"context"
	"errors"
	"fmt"
)

// Every parent write for an existing child commits only while the guardian
// still holds what the check before its unit of work found (#6529): an active
// membership of the child's school, the relationship to the child bound to
// the account, parent_portal.access and every action permission of the
// write. The check before the unit of work ends with its own transaction, or,
// bundled (ADR 0161), reads without locks; a revocation committed since then
// must refuse the write, and one that comes later must wait for it.
//
// guardChildWrite is the one guard that holds these facts; its locks and
// checks run through GuardianHold, People Directory's guardian hold, which
// Enrollment's renewal of an existing child runs as well (#6638). It runs
// inside the write's school transaction before the first owner write and
// locks in the order Konto → Schule → Kind → Profil → Beziehung:
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
	// readReceipt skips the guard, see ReadReceiptOnly.
	readReceipt bool
}

// ReadReceiptOnly marks a WithPermittedChildForWrite whose only write is the
// reader's own read cursor of what it just read: the child check in the same
// transaction authorizes the read, no data of the child changes, so the
// guard's locks are not taken. The architecture rule follows such a write
// argument like unguarded code, so its writes need their exemption.
func ReadReceiptOnly(access *ChildWriteAccess) { access.readReceipt = true }

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

// GuardianHold is the port to People Directory's guardian hold
// (guardianhold.Hold, #6638): the one lock and check protocol of the parent
// guard, run over the guard's own locks. The composition binds it; it
// answers ErrChildNotLinked, ErrGuardianPermissionDenied or
// ErrChildWriteGuardUnavailable for its refusals and passes lock errors
// through.
type GuardianHold func(ctx context.Context, locks HoldLocks, req HoldRequest) error

// HoldLocks are the guard's locks in the shape of guardianhold.Locks.
type HoldLocks struct {
	Membership   func(ctx context.Context, accountID int64) (bool, error)
	Child        func(ctx context.Context, studentID int64) error
	Profile      func(ctx context.Context, profileID int64) (bool, error)
	Relationship func(ctx context.Context, studentID, profileID int64) (func(permission string) bool, error)
	Owner        func(ctx context.Context, profileID int64) (accountID *int64, email string, found bool, err error)
}

// HoldRequest is one write's authority in the shape of guardianhold.Request.
type HoldRequest struct {
	AccountID   int64
	Email       string
	StudentID   int64
	ProfileID   int64
	OwnProfile  bool
	Profiles    []int64
	Access      string
	Permissions []string
}

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
// the write's school transaction, with every fact locked until commit. The
// locks and checks are People Directory's guardian hold (guardianhold.Hold),
// the one protocol every parent authority over an existing child runs
// (#6638); this package binds its locks to the guard's ports.
func (s *Service) guardChildWrite(ctx context.Context, accountID, studentID int64, child *Child, access ChildWriteAccess) error {
	if access.readReceipt {
		return nil
	}
	if err := s.childWriteGuardable(accountID, studentID, child); err != nil {
		return err
	}
	if access.InviteEmail != "" {
		if s.InvitationAccounts == nil {
			return ErrChildWriteGuardUnavailable
		}
		if err := s.InvitationAccounts(ctx, accountID, access.InviteEmail); err != nil {
			return err
		}
	}
	return s.GuardianHold(ctx, s.childWriteLocks(access), HoldRequest{
		AccountID: accountID, StudentID: studentID, ProfileID: child.GuardianProfileID,
		OwnProfile: access.OwnProfile, Profiles: access.Profiles,
		Access: GuardianPermissionPortalAccess, Permissions: access.Permissions,
	})
}

// childWriteGuardable refuses a write the guard cannot hold: an unresolved
// child, or a service composed without the ports the guard locks through.
func (s *Service) childWriteGuardable(accountID, studentID int64, child *Child) error {
	if accountID <= 0 || studentID <= 0 || child == nil || child.GuardianProfileID <= 0 {
		return ErrChildNotLinked
	}
	if s.GuardianHold == nil || s.Memberships == nil || s.StudentRepo == nil || s.GuardianProfileRepo == nil || s.StudentGuardianRepo == nil {
		return ErrChildWriteGuardUnavailable
	}
	return nil
}

// childWriteLocks binds the guardian hold to the guard's ports: Identity &
// Access's membership lock and People Directory's child, profile and
// relationship rows. A child that cannot be locked is refused in the write's
// own terms when it names them (ChildMissing).
func (s *Service) childWriteLocks(access ChildWriteAccess) HoldLocks {
	return HoldLocks{
		Membership: func(ctx context.Context, accountID int64) (bool, error) {
			active, err := s.Memberships(ctx, accountID)
			if err != nil {
				return false, fmt.Errorf("parent: lock school membership: %w", err)
			}
			return active, nil
		},
		Child: func(ctx context.Context, studentID int64) error {
			_, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
			if err != nil && access.ChildMissing != nil {
				return access.ChildMissing(ctx, err)
			}
			return err
		},
		Profile: func(ctx context.Context, profileID int64) (bool, error) {
			err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, profileID)
			if errors.Is(err, ErrGuardianProfileNotFound) {
				return false, nil
			}
			return err == nil, err
		},
		Relationship: func(ctx context.Context, studentID, profileID int64) (func(string) bool, error) {
			link, err := s.findChildGuardianLinkForUpdate(ctx, studentID, profileID)
			if errors.Is(err, ErrGuardianNotLinked) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return func(permission string) bool { return s.guardianHasPermission(link, permission) }, nil
		},
		Owner: func(ctx context.Context, profileID int64) (*int64, string, bool, error) {
			profile, err := s.GuardianProfileRepo.FindByID(ctx, profileID)
			if errors.Is(err, ErrGuardianProfileNotFound) || (err == nil && profile == nil) {
				return nil, "", false, nil
			}
			if err != nil {
				return nil, "", false, err
			}
			email := ""
			if profile.Email != nil {
				email = *profile.Email
			}
			return profile.AccountID, email, true, nil
		},
	}
}
