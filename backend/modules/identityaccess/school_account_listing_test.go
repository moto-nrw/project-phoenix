package identityaccess_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Only the account-less "invited" row stands for a pending invitation.
func TestSchoolAccountListingPendingInvitation(t *testing.T) {
	t.Parallel()

	assert.True(t, identityaccess.SchoolAccountListing{Status: "invited"}.PendingInvitation())
	assert.False(t, identityaccess.SchoolAccountListing{AccountID: 3, Status: "invited"}.PendingInvitation())
	assert.False(t, identityaccess.SchoolAccountListing{Status: "active"}.PendingInvitation())
}
