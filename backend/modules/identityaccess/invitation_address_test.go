package identityaccess_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Every invitation, the external caregiver's included (#4573), stores a
// trimmed, lower-case plain address of at most 254 characters.
func TestNormalizeInvitationEmail(t *testing.T) {
	t.Parallel()

	email, err := identityaccess.NormalizeInvitationEmail("  Lea.Gast@Example.TEST ")
	require.NoError(t, err)
	assert.Equal(t, "lea.gast@example.test", email)

	longest := strings.Repeat("a", 254-len("@example.test")) + "@example.test"
	email, err = identityaccess.NormalizeInvitationEmail(longest)
	require.NoError(t, err)
	assert.Equal(t, longest, email)

	_, err = identityaccess.NormalizeInvitationEmail("   ")
	require.ErrorIs(t, err, identityaccess.ErrInvitationEmailRequired)
	for _, raw := range []string{"keine-adresse", "Lea <lea@example.test>", "a" + longest} {
		_, err = identityaccess.NormalizeInvitationEmail(raw)
		require.ErrorIs(t, err, identityaccess.ErrInvitationEmailInvalid, raw)
	}
}

func TestPendingInvitationsByPerson(t *testing.T) {
	t.Parallel()

	first, second := int64(10), int64(20)
	byPerson := identityaccess.PendingInvitationsByPerson([]identityaccess.SchoolInvitation{
		{ID: 1, PersonID: &first}, {ID: 2}, {ID: 3, PersonID: &first}, {ID: 4, PersonID: &second},
	})
	require.Len(t, byPerson, 2, "an invitation naming no person is left out")
	assert.Equal(t, []int64{1, 3}, invitationIDs(byPerson[first]))
	assert.Equal(t, []int64{4}, invitationIDs(byPerson[second]))
}

func invitationIDs(invitations []identityaccess.SchoolInvitation) []int64 {
	ids := make([]int64, 0, len(invitations))
	for _, invitation := range invitations {
		ids = append(ids, invitation.ID)
	}
	return ids
}
