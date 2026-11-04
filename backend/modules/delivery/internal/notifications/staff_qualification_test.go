package notifications_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaffQualificationNoticeFollowsTheRight pins the Leitung's reminder to
// staff:stammdaten and keeps the person's own reminder offered to everyone.
func TestStaffQualificationNoticeFollowsTheRight(t *testing.T) {
	t.Parallel()
	def, ok := notifications.GetType(notifications.TypeStaffQualificationExpiry)
	require.True(t, ok)
	assert.Equal(t, permissions.StaffStammdaten, def.Permission)
	assert.True(t, def.DefaultOn)
	assert.Equal(t, notifications.PortalStaff, def.Portal)

	own, ok := notifications.GetType(notifications.TypeStaffQualificationOwnExpiry)
	require.True(t, ok)
	assert.Empty(t, own.Permission)
	assert.False(t, own.DefaultOn)
}
