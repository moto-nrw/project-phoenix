package owners

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type stubGuardianInvitationReader []identityaccess.GuardianInvitation

func (s stubGuardianInvitationReader) ListOpenGuardianInvitations(context.Context, []int64) ([]identityaccess.GuardianInvitation, error) {
	return s, nil
}

// #6294: the open list also carries legacy links without a recorded
// recipient; the address binding ended them and nobody can redeem one, so
// the enrollment status must not promise the family an invitation.
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
		want        bool
	}{
		{"bound open link", stubGuardianInvitationReader{open}, true},
		{"legacy link without recipient", stubGuardianInvitationReader{legacy}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := enrollmentGuardianInvitationAvailability{invitations: tc.invitations}.HasRedeemableGuardianInvitation(context.Background(), int64(len(tc.name)))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
