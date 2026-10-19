package identityaccess_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// An unset lifetime falls back, a configured one is kept up to its cap.
func TestTokenExpiriesAreBounded(t *testing.T) {
	t.Parallel()
	for configured, want := range map[int]time.Duration{-1: 48 * time.Hour, 0: 48 * time.Hour, 24: 24 * time.Hour, 168: 168 * time.Hour, 500: 168 * time.Hour} {
		assert.Equal(t, want, identityaccess.InvitationTokenExpiry(configured), "invitation hours %d", configured)
	}
	for configured, want := range map[int]time.Duration{-1: 30 * time.Minute, 0: 30 * time.Minute, 60: time.Hour, 1440: 24 * time.Hour, 5000: 24 * time.Hour} {
		assert.Equal(t, want, identityaccess.PasswordResetTokenExpiry(configured), "reset minutes %d", configured)
	}
}

// Only system roles are listed, in their order.
func TestSystemRolesKeepsSystemRoles(t *testing.T) {
	t.Parallel()
	const adminID, ownID, userID int64 = 10, 20, 30
	roles := []identityaccess.Role{{ID: adminID, Name: "admin", IsSystem: true}, {ID: ownID, Name: "Honorarkraft"}, {ID: userID, Name: "user", IsSystem: true}}
	assert.Equal(t, []identityaccess.Role{roles[0], roles[2]}, identityaccess.SystemRoles(roles))
	assert.Empty(t, identityaccess.SystemRoles(nil))
}

// A school-wide scope reaches every child, a group scope only its groups.
func TestReviewReachesGroup(t *testing.T) {
	t.Parallel()
	const ownGroup, otherGroup int64 = 10, 20
	own, other := ownGroup, otherGroup

	schoolWide := identityaccess.ReviewReachesGroup(true, nil)
	assert.True(t, schoolWide(&other))
	assert.True(t, schoolWide(nil))

	grouped := identityaccess.ReviewReachesGroup(false, []int64{ownGroup})
	assert.True(t, grouped(&own))
	assert.False(t, grouped(&other))
	assert.False(t, grouped(nil), "a child without a group lies outside every group scope")
	assert.False(t, identityaccess.ReviewReachesGroup(false, nil)(&own))
}

// The presets the operator and staff flows pass.
func TestProvisioningAndStaffInvitePresets(t *testing.T) {
	t.Parallel()
	assert.True(t, identityaccess.OperatorProvisioningCreatesPerson)
	assert.False(t, identityaccess.StaffInviteRequiresApproval)
}
