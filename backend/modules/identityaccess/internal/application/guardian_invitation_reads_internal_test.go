package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func TestProvisionGuardianAccount_LocksExistingAccount(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.addAccount(40, "parent@example.test", "hash:secret", true)

	account, err := f.lifecycle.provisionGuardianAccount(tenantContext(), "parent@example.test", domain.GuardianRegistration{})
	require.NoError(t, err)
	require.Equal(t, int64(40), account.ID)
	require.Contains(t, f.store.recorded(), "LockLoginAccountByEmail",
		"acceptance serializes with account retention before it grants access")
}

// The invitation reads serve the lists from the store alone, in the public
// vocabulary, and report a failed read in the operation envelope the module
// always answered with.
func TestGuardianInvitationReads_ListThroughTheStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	profileID := int64(len(t.Name()))

	store := newLifecycleInvitations()
	stored := store.add(domain.GuardianInvitation{
		GuardianProfileID: profileID, ExpiresAt: time.Now().Add(time.Hour),
		ApprovalStatus: domain.GuardianInvitationApprovalNotRequired,
	})
	reads := NewGuardianInvitationReads(store)

	all, err := reads.ListGuardianInvitations(ctx, profileID)
	require.NoError(t, err)
	require.Equal(t, []identityaccess.GuardianInvitation{identityaccess.GuardianInvitation(stored)}, all)
	open, err := reads.ListOpenGuardianInvitations(ctx, []int64{profileID})
	require.NoError(t, err)
	require.Equal(t, all, open)
	redeemable, err := reads.ListRedeemableGuardianInvitations(ctx)
	require.NoError(t, err)
	require.Equal(t, all, redeemable)

	failure := errors.New("invitation store down")
	store.listErr = failure
	lists := map[string]func() ([]identityaccess.GuardianInvitation, error){
		"all": func() ([]identityaccess.GuardianInvitation, error) {
			return reads.ListGuardianInvitations(ctx, profileID)
		},
		"open": func() ([]identityaccess.GuardianInvitation, error) {
			return reads.ListOpenGuardianInvitations(ctx, []int64{profileID})
		},
		"redeemable": func() ([]identityaccess.GuardianInvitation, error) {
			return reads.ListRedeemableGuardianInvitations(ctx)
		},
	}
	for name, list := range lists {
		got, err := list()
		require.Nil(t, got, name)
		require.ErrorIs(t, err, failure, name)
		var envelope *identityaccess.AuthenticationError
		require.ErrorAs(t, err, &envelope, name)
		require.Equal(t, opGuardianInviteFetch, envelope.Op, name)
	}
}
