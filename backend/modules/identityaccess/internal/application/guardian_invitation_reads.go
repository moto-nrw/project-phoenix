package application

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
)

// GuardianInvitationReads are the invitation lists other owners read: the
// People Directory guardian list and the parents portal. They read the store
// alone, so a root composes them before the module that needs the People
// Directory (#4080); the module's own reads go through them as well.
type GuardianInvitationReads struct {
	invitations ports.GuardianInvitationStore
	run         func(context.Context, func(context.Context) error) error
}

var _ identityaccess.GuardianInvitationReads = GuardianInvitationReads{}

// NewGuardianInvitationReads reads the invitations through the store.
func NewGuardianInvitationReads(invitations ports.GuardianInvitationStore, run func(context.Context, func(context.Context) error) error) GuardianInvitationReads {
	return GuardianInvitationReads{invitations: invitations, run: run}
}

// ListGuardianInvitations returns every invitation ever issued for the
// guardian contact, newest first.
func (r GuardianInvitationReads) ListGuardianInvitations(ctx context.Context, guardianProfileID int64) ([]identityaccess.GuardianInvitation, error) {
	var invitations []domain.GuardianInvitation
	err := r.run(ctx, func(txCtx context.Context) error {
		var err error
		invitations, err = r.invitations.ListGuardianInvitationsByProfile(txCtx, guardianProfileID)
		return err
	})
	return guardianInvitationList(invitations, err)
}

// ListOpenGuardianInvitations returns the invitations of those contacts that
// are still going somewhere: redeemable, or waiting for a staff decision.
// One read serves the whole guardian list.
func (r GuardianInvitationReads) ListOpenGuardianInvitations(ctx context.Context, guardianProfileIDs []int64) ([]identityaccess.GuardianInvitation, error) {
	var invitations []domain.GuardianInvitation
	err := r.run(ctx, func(txCtx context.Context) error {
		var err error
		invitations, err = r.invitations.ListOpenGuardianInvitations(txCtx, guardianProfileIDs, time.Now())
		return err
	})
	return guardianInvitationList(invitations, err)
}

// ListRedeemableGuardianInvitations returns the invitations of the school in
// context whose link can still be spent.
func (r GuardianInvitationReads) ListRedeemableGuardianInvitations(ctx context.Context) ([]identityaccess.GuardianInvitation, error) {
	var invitations []domain.GuardianInvitation
	err := r.run(ctx, func(txCtx context.Context) error {
		var err error
		invitations, err = r.invitations.ListRedeemableGuardianInvitations(txCtx, time.Now())
		return err
	})
	return guardianInvitationList(invitations, err)
}

func guardianInvitationList(invitations []domain.GuardianInvitation, err error) ([]identityaccess.GuardianInvitation, error) {
	if err != nil {
		return nil, invitationError(failed(opGuardianInviteFetch, err))
	}
	return publicGuardianInvitations(invitations), nil
}

func (l *AccountLifecycle) ListGuardianInvitations(ctx context.Context, guardianProfileID int64) ([]identityaccess.GuardianInvitation, error) {
	if l == nil {
		return nil, errAccountLifecycleUnavailable
	}
	return NewGuardianInvitationReads(l.invitations, l.sessions.tx.RunRead).ListGuardianInvitations(bindRuntime(l.auth.attach, ctx), guardianProfileID)
}

func (l *AccountLifecycle) ListOpenGuardianInvitations(ctx context.Context, guardianProfileIDs []int64) ([]identityaccess.GuardianInvitation, error) {
	if l == nil {
		return nil, errAccountLifecycleUnavailable
	}
	return NewGuardianInvitationReads(l.invitations, l.sessions.tx.RunRead).ListOpenGuardianInvitations(bindRuntime(l.auth.attach, ctx), guardianProfileIDs)
}

func (l *AccountLifecycle) ListRedeemableGuardianInvitations(ctx context.Context) ([]identityaccess.GuardianInvitation, error) {
	if l == nil {
		return nil, errAccountLifecycleUnavailable
	}
	return NewGuardianInvitationReads(l.invitations, l.sessions.tx.RunRead).ListRedeemableGuardianInvitations(bindRuntime(l.auth.attach, ctx))
}

// NewGuardianInvitationInvalidation binds the guarded command independently of
// the full lifecycle, so contact edits need neither late binding nor a cycle.
func NewGuardianInvitationInvalidation(invitations ports.GuardianInvitationStore) identityaccess.GuardianInvitationInvalidation {
	return func(ctx context.Context, profileID int64) error {
		if profileID <= 0 {
			return failed("invalidate guardian invitations", identityaccess.ErrGuardianInvitationNotFound)
		}
		// The store refuses a missing school or transaction. This leaf command
		// never starts an independent unit or switches the caller's scope.
		return invitationError(invitations.InvalidateGuardianInvitations(ctx, profileID, time.Now(), true))
	}
}
