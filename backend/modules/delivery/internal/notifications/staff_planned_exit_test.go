package notifications_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
)

// The planned staff exit notices go to whoever may plan an exit (#4809):
// offered and default-on for holders of users:delete only.
func TestStaffPlannedExitNoticeFollowsTheRight(t *testing.T) {
	t.Parallel()
	def, ok := notifications.GetType(notifications.TypeStaffPlannedExit)
	require.True(t, ok)
	assert.Equal(t, permissions.UsersDelete, def.Permission)
	assert.True(t, def.DefaultOn)
	assert.Equal(t, notifications.PortalStaff, def.Portal)
}

// A shared child note reaches whoever may read the notes (#6854): offered and
// default-on for holders of users:read only.
func TestStaffStudentNoteSharedFollowsTheRight(t *testing.T) {
	t.Parallel()
	def, ok := notifications.GetType(notifications.TypeStaffStudentNoteShared)
	require.True(t, ok)
	assert.Equal(t, permissions.UsersRead, def.Permission)
	assert.True(t, def.DefaultOn)
	assert.Equal(t, notifications.PortalStaff, def.Portal)
	assert.Equal(t, notifications.GroupChildren, def.Group)
}
