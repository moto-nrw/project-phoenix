package owners

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
)

type stubGuardianInvitationReader []identityaccess.GuardianInvitation

type stubGuardianProfileAddresses string

func (s stubGuardianProfileAddresses) FindByIDs(_ context.Context, ids []int64) (map[int64]*peopledirectory.GuardianProfileSnapshot, error) {
	address := string(s)
	result := map[int64]*peopledirectory.GuardianProfileSnapshot{}
	for _, id := range ids {
		result[id] = &peopledirectory.GuardianProfileSnapshot{ID: id, Email: &address}
	}
	return result, nil
}

func (s stubGuardianInvitationReader) ListOpenGuardianInvitations(context.Context, []int64) ([]identityaccess.GuardianInvitation, error) {
	return s, nil
}

// #6294: only a link issued for the address the profile has now redeems. A
// legacy link without a recorded recipient, or one mailed to an address
// since corrected, must not promise the family an invitation.
func TestEnrollmentInvitationAvailability_IgnoresLinksWithoutRecipient(t *testing.T) {
	t.Parallel()
	open := identityaccess.GuardianInvitation{
		Email: "eltern@example.test", ExpiresAt: time.Now().Add(time.Hour),
		ApprovalStatus: identityaccess.GuardianInvitationApprovalNotRequired,
	}
	legacy := identityaccess.GuardianInvitation{
		ExpiresAt: time.Now().Add(-time.Hour), ApprovalStatus: identityaccess.GuardianInvitationApprovalNotRequired,
	}
	for _, tc := range []struct {
		name        string
		invitations stubGuardianInvitationReader
		address     string
		want        bool
	}{
		{"bound open link", stubGuardianInvitationReader{open}, " Eltern@Example.test ", true},
		{"legacy link without recipient", stubGuardianInvitationReader{legacy}, "eltern@example.test", false},
		{"address corrected after the link was mailed", stubGuardianInvitationReader{open}, "korrigiert@example.test", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := enrollmentGuardianInvitationAvailability{invitations: tc.invitations, profiles: stubGuardianProfileAddresses(tc.address)}.HasRedeemableGuardianInvitation(context.Background(), int64(len(tc.name)))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
