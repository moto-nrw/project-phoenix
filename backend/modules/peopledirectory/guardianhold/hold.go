// Package guardianhold is the parent guard's protocol (#6529, #6638): the one
// sequence of locks and checks that holds a guardian's authority over a child
// from a check before the unit of work until the write commits. The Parent
// Portal's WithChildWrite runs it for every parent write of an existing
// child; Enrollment runs it before a parent's authority renews an existing
// child. Both bind the locks to the same owners: Identity & Access for the
// account and its school mapping, People Directory for child, guardian
// profiles and relationship.
//
// Hold runs inside the caller's school transaction and locks in the order
// Konto → Schule → Kind → Profil → Beziehung:
//
//  1. the live account FOR KEY SHARE, then its active mapping at the school
//     FOR SHARE (Membership): a deactivation or deletion waits for the
//     write, without cycling with a later foreign key check on the account;
//  2. the child row (Child), the lock every parent write and care exit of
//     the child takes first;
//  3. the guardian profiles the write changes, ascending (Profile);
//  4. the relationship row FOR UPDATE (Relationship): every change of its
//     permissions, role or existence locks or deletes it first.
//
// The locked relationship must still name a profile of the identity, and the
// relationship must still grant the access and every permission asked for.
// A revocation committed before Hold is seen and refuses; one that comes
// later waits for the caller's commit.
package guardianhold

import (
	"context"
	"errors"
	"slices"
	"strings"
)

var (
	// ErrNotLinked refuses an identity that no longer holds the school
	// membership, the relationship, the profile binding or the access.
	ErrNotLinked = errors.New("guardian hold: the guardian is not linked to the child")
	// ErrPermissionDenied refuses a relationship without an action
	// permission of the write.
	ErrPermissionDenied = errors.New("guardian hold: the relationship lacks a permission of the write")
	// ErrUnavailable refuses a hold composed without one of its locks;
	// writes fail closed without them.
	ErrUnavailable = errors.New("guardian hold: a lock is not composed")
)

// Locks are the owner commands Hold runs in the caller's school transaction.
// They use plain types only, so a consumer declares the same struct as its
// own port and converts it.
type Locks struct {
	// Membership pins the account's active mapping at the school after the
	// live account; false when either is gone.
	Membership func(ctx context.Context, accountID int64) (bool, error)
	// Child locks the child row FOR UPDATE.
	Child func(ctx context.Context, studentID int64) error
	// Profile locks a guardian profile FOR UPDATE; false when it does not
	// exist.
	Profile func(ctx context.Context, profileID int64) (bool, error)
	// Relationship locks the relationship of profile and child FOR UPDATE
	// and returns its grant check; nil when there is none.
	Relationship func(ctx context.Context, studentID, profileID int64) (func(permission string) bool, error)
	// Owner reads the account and address a guardian profile is bound to;
	// found is false when the profile does not exist.
	Owner func(ctx context.Context, profileID int64) (accountID *int64, email string, found bool, err error)
}

// Request is one identity's authority over one child.
type Request struct {
	// AccountID is the acting account; zero for a guardian named by Email.
	AccountID int64
	// Email names an accountless guardian, such as the recipient of a late
	// invite. Hold locks the profile so its address stays the identity; a
	// profile that carries an account needs that account's membership.
	Email     string
	StudentID int64
	// ProfileID is the guardian profile the check before the unit resolved.
	ProfileID int64
	// OwnProfile locks ProfileID before the relationship, for writes that
	// change the guardian's own contact rows.
	OwnProfile bool
	// Profiles are further profiles the write changes. A missing one is left
	// to the write, which reports it in its own terms.
	Profiles []int64
	// Access is the permission without which the guardian counts as not
	// linked (the portal access); empty asks for none.
	Access string
	// Permissions are the action permissions of the write.
	Permissions []string
	// NotLinked, Denied and Unavailable replace ErrNotLinked,
	// ErrPermissionDenied and ErrUnavailable with the caller's own refusals
	// when set.
	NotLinked, Denied, Unavailable error
}

// Hold locks and re-checks the request's authority in the caller's school
// transaction; see the package comment for the order. Errors of the locks
// pass through unchanged.
func Hold(ctx context.Context, locks Locks, req Request) error {
	err := hold(ctx, locks, req)
	switch {
	case req.NotLinked != nil && errors.Is(err, ErrNotLinked):
		return req.NotLinked
	case req.Denied != nil && errors.Is(err, ErrPermissionDenied):
		return req.Denied
	case req.Unavailable != nil && errors.Is(err, ErrUnavailable):
		return req.Unavailable
	}
	return err
}

func hold(ctx context.Context, locks Locks, req Request) error {
	if req.StudentID <= 0 || req.ProfileID <= 0 || (req.AccountID <= 0 && strings.TrimSpace(req.Email) == "") {
		return ErrNotLinked
	}
	if locks.Membership == nil || locks.Child == nil || locks.Profile == nil || locks.Relationship == nil || locks.Owner == nil {
		return ErrUnavailable
	}
	account, err := holdAccount(ctx, locks, req)
	if err != nil {
		return err
	}
	if err := locks.Child(ctx, req.StudentID); err != nil {
		return err
	}
	if err := lockProfiles(ctx, locks, req); err != nil {
		return err
	}
	granted, err := locks.Relationship(ctx, req.StudentID, req.ProfileID)
	if err != nil {
		return err
	}
	if granted == nil {
		return ErrNotLinked
	}
	if err := requireOwner(ctx, locks, req, account); err != nil {
		return err
	}
	return requirePermissions(granted, req)
}

// holdAccount pins the membership of the account the request acts for: the
// acting account, or the account an accountless guardian's profile carries.
// It returns that account, zero for a profile without one.
func holdAccount(ctx context.Context, locks Locks, req Request) (int64, error) {
	account := req.AccountID
	if account <= 0 {
		owner, _, found, err := locks.Owner(ctx, req.ProfileID)
		if err != nil {
			return 0, err
		}
		if !found {
			return 0, ErrNotLinked
		}
		if owner == nil {
			return 0, nil
		}
		account = *owner
	}
	active, err := locks.Membership(ctx, account)
	if err != nil {
		return 0, err
	}
	if !active {
		return 0, ErrNotLinked
	}
	return account, nil
}

// lockProfiles locks the profiles of the request in ascending order. An
// accountless identity locks its own profile, whose address names it.
func lockProfiles(ctx context.Context, locks Locks, req Request) error {
	ids := append([]int64(nil), req.Profiles...)
	if req.OwnProfile || req.AccountID <= 0 {
		ids = append(ids, req.ProfileID)
	}
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		found, err := locks.Profile(ctx, id)
		switch {
		case err != nil:
			return err
		case !found && id == req.ProfileID:
			return ErrNotLinked
		}
		// The write reports a missing further contact itself.
	}
	return nil
}

// requireOwner refuses a relationship whose profile is no longer bound to
// the identity the check before the unit resolved. A profile cannot change
// its account while a relationship holds it; an accountless identity's
// profile is locked, so its address holds as well.
func requireOwner(ctx context.Context, locks Locks, req Request, account int64) error {
	owner, email, found, err := locks.Owner(ctx, req.ProfileID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotLinked
	}
	ownerAccount := int64(0)
	if owner != nil {
		ownerAccount = *owner
	}
	if ownerAccount != account {
		return ErrNotLinked
	}
	if req.AccountID <= 0 && !strings.EqualFold(strings.TrimSpace(email), strings.TrimSpace(req.Email)) {
		return ErrNotLinked
	}
	return nil
}

// requirePermissions checks the access and every action permission on the
// locked relationship.
func requirePermissions(granted func(string) bool, req Request) error {
	if req.Access != "" && !granted(req.Access) {
		return ErrNotLinked
	}
	for _, permission := range req.Permissions {
		if permission != "" && permission != req.Access && !granted(permission) {
			return ErrPermissionDenied
		}
	}
	return nil
}
