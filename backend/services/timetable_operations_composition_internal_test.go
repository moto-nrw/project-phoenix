package services

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #2085: the attendance refresh is tenant-wide, so it must not name the
// child whose attendance was patched — every staff client of the school
// receives it, including colleagues outside gdpr.student_data_scope. The
// instance id (block scope, not child identity) is what clients refetch on.
// The Timetable owner hands over only tenant, session and block
// (compose's TestTimetableOperationsPatchAttendanceUpdatesRowAndBroadcasts).
func TestTimetableAttendanceAnnouncerNamesTheBlockNotTheChild(t *testing.T) {
	t.Parallel()
	broadcaster := testpkg.NewRecordingBroadcaster()
	announcer := timetableAttendanceAnnouncer{broadcaster: broadcaster}

	require.NoError(t, announcer.AnnounceAttendanceChanged(721, 293, 403))

	calls := broadcaster.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, int64(721), calls[0].TenantID)
	assert.Equal(t, "active_supervision_changed", string(calls[0].Event.Type))
	assert.Equal(t, "tenant", calls[0].Method)
	testpkg.AssertNoTenantWideStudentIdentity(t, broadcaster)
	require.NotNil(t, calls[0].Event.Data.InstanceID)
	assert.Equal(t, "403", *calls[0].Event.Data.InstanceID)
}
