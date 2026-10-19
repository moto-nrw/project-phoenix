package owners

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// recordingStatusFlagArchive records the archive requests the Worker's
// end-of-day clear reaches Care Plan with.
type recordingStatusFlagArchive struct {
	careplan.Capability
	archives []careplan.StatusFlagArchive
}

func (r *recordingStatusFlagArchive) ArchiveStudentStatusFlags(_ context.Context, archive careplan.StatusFlagArchive) (int64, error) {
	r.archives = append(r.archives, archive)
	return 2, nil
}

// The Worker names the flag; the root archives the sick flag as a sick day
// and the excused flag as an excused day, both from the end-of-day source.
func TestEndOfDayStatusFlagArchiveBindsCarePlanStatuses(t *testing.T) {
	t.Parallel()

	carePlan := &recordingStatusFlagArchive{}
	archive := NewStudentStatusDayRepository(carePlan)
	date := calendar.NewDate(2026, time.October, 16)
	reported := date.BerlinMidnight().Add(18 * time.Hour)

	affected, err := archive.ArchiveEndOfDayStatusFlag(context.Background(), "sick", "sick_since", date, reported)
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)
	_, err = archive.ArchiveEndOfDayStatusFlag(context.Background(), "excused", "excused_since", date, reported)
	require.NoError(t, err)

	require.Len(t, carePlan.archives, 2)
	assert.Equal(t, careplan.StatusFlagArchive{
		FlagColumn: "sick", SinceColumn: "sick_since", Status: "sick",
		Date: careplan.Date(date), ReportedFallback: reported, Source: "end_of_day",
	}, carePlan.archives[0])
	assert.Equal(t, "excused", carePlan.archives[1].Status)
	assert.Equal(t, "end_of_day", carePlan.archives[1].Source)

	_, err = archive.ArchiveEndOfDayStatusFlag(context.Background(), "trip", "trip_since", date, reported)
	require.ErrorContains(t, err, `unsupported status flag column "trip"`)
	assert.Len(t, carePlan.archives, 2, "an unknown flag never reaches Care Plan")
}

type endedSessions struct {
	studentpresence.SessionMaintenance
	result *studentpresence.DailySessionCleanupResult
}

func (e endedSessions) EndDailySessions(context.Context) (*studentpresence.DailySessionCleanupResult, error) {
	return e.result, nil
}

// The Worker completes the timetable instances of the active groups the
// session end closed, so their IDs have to reach it.
func TestSchedulerSessionMaintenanceReportsTheEndedGroups(t *testing.T) {
	t.Parallel()

	maintenance := newSchedulerSessionMaintenance(endedSessions{result: &studentpresence.DailySessionCleanupResult{
		SessionsEnded: 2, VisitsEnded: 5, SupervisorsEnded: 1, EndedActiveGroupIDs: []int64{7, 9}, Success: true,
	}})

	ended, err := maintenance.EndDailySessions(context.Background())
	require.NoError(t, err)
	require.NotNil(t, ended)
	assert.Equal(t, []int64{7, 9}, ended.EndedActiveGroupIDs)
	assert.Equal(t, 2, ended.SessionsEnded)
	assert.Equal(t, 5, ended.VisitsEnded)
	assert.Equal(t, 1, ended.SupervisorsEnded)

	none, err := newSchedulerSessionMaintenance(endedSessions{}).EndDailySessions(context.Background())
	require.NoError(t, err)
	assert.Nil(t, none)
	assert.Nil(t, newSchedulerSessionMaintenance(nil), "a missing owner leaves the port unset")
	assert.Nil(t, newSchedulerPresenceCleanup(nil))
	assert.Nil(t, pwaUsageCleanup(nil))
}

// The Worker's overdue announcements reach the whole school as the live
// events the dashboards and supervision views listen for, and a transport
// failure comes back to the Worker.
func TestSchedulerOverdueBroadcasterSendsTheLiveEventsTenantWide(t *testing.T) {
	t.Parallel()

	hub := testpkg.NewRecordingBroadcaster()
	broadcaster := schedulerOverdueBroadcaster{hub: hub}

	require.NoError(t, broadcaster.BroadcastInstanceOverdue(42, 151, "2026-04-20", "10:00:00", 251))
	require.NoError(t, broadcaster.BroadcastInstanceOverdueRefresh(42, 151))

	calls := hub.CallsByMethod("tenant")
	require.Len(t, calls, 2)
	assert.Equal(t, int64(42), calls[0].TenantID)
	assert.Equal(t, realtime.EventInstanceOverdue, calls[0].Event.Type)
	require.NotNil(t, calls[0].Event.Data.InstanceID)
	assert.Equal(t, "151", *calls[0].Event.Data.InstanceID)
	require.NotNil(t, calls[0].Event.Data.RoomID)
	assert.Equal(t, "251", *calls[0].Event.Data.RoomID)
	assert.Equal(t, int64(42), calls[1].TenantID)
	assert.Equal(t, realtime.EventActiveSupervisionChanged, calls[1].Event.Type)
	require.NotNil(t, calls[1].Event.Data.Reason)
	assert.Equal(t, "instance_overdue", *calls[1].Event.Data.Reason)

	hub.Err = errors.New("hub closed")
	assert.ErrorIs(t, broadcaster.BroadcastInstanceOverdue(42, 151, "2026-04-20", "10:00:00", 251), hub.Err)
	assert.ErrorIs(t, broadcaster.BroadcastInstanceOverdueRefresh(42, 151), hub.Err)
	assert.Nil(t, newSchedulerOverdueBroadcaster(nil), "a missing hub leaves the port unset")
}
