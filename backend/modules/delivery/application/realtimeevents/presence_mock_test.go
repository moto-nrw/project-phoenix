package realtimeevents

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func presenceCtx() context.Context {
	return testpkg.ContextForTenant(context.Background(), 42)
}

func TestPublishVisitCheckInRoutesToSessionAndEducationTopics(t *testing.T) {
	t.Parallel()
	publisher := testpkg.NewRecordingBroadcaster()
	education := int64(7)
	substatus := "late"
	PublishVisitCheckIn(presenceCtx(), publisher, nil, VisitChange{
		ActiveGroupID: "12", StudentID: "99", EducationGroupID: &education, Source: "web",
		Attendance: &AttendanceDetail{Status: "present", Substatus: &substatus},
	})

	calls := publisher.Calls()
	require.Len(t, calls, 2)
	assert.Equal(t, "12", calls[0].Topic)
	assert.Equal(t, "edu:7", calls[1].Topic)
	for _, call := range calls {
		assert.EqualValues(t, 42, call.TenantID)
		assert.Equal(t, "student_checkin", string(call.Event.Type))
		assert.Equal(t, "12", call.Event.ActiveGroupID)
		require.NotNil(t, call.Event.Data.StudentID)
		assert.Equal(t, "99", *call.Event.Data.StudentID)
		require.NotNil(t, call.Event.Data.Source)
		assert.Equal(t, "web", *call.Event.Data.Source)
		require.NotNil(t, call.Event.Data.GroupIDs)
		assert.Equal(t, []string{"7"}, *call.Event.Data.GroupIDs)
		require.NotNil(t, call.Event.Data.AttendanceStatus)
		assert.Equal(t, "present", *call.Event.Data.AttendanceStatus)
		assert.Equal(t, &substatus, call.Event.Data.AttendanceSubstatus)
		assert.Nil(t, call.Event.Data.AttendanceNote)
	}
}

func TestPublishVisitCheckOutOmitsAbsentScopeFields(t *testing.T) {
	t.Parallel()
	publisher := testpkg.NewRecordingBroadcaster()
	PublishVisitCheckOut(presenceCtx(), publisher, nil, VisitChange{ActiveGroupID: "12", StudentID: "99"})

	calls := publisher.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "student_checkout", string(calls[0].Event.Type))
	assert.Nil(t, calls[0].Event.Data.Source, "an empty source stays absent")
	assert.Nil(t, calls[0].Event.Data.GroupIDs, "an unknown group keeps the field absent so clients refresh broadly")
	assert.Nil(t, calls[0].Event.Data.AttendanceStatus)
}

func TestPublishRoomlessAttendanceChangeReachesOnlyTheEducationTopic(t *testing.T) {
	t.Parallel()
	publisher := testpkg.NewRecordingBroadcaster()
	PublishRoomlessAttendanceChange(presenceCtx(), publisher, nil, false, VisitChange{StudentID: "99"})
	assert.Empty(t, publisher.Calls(), "without an educational group there is nobody to notify")

	education := int64(7)
	PublishRoomlessAttendanceChange(presenceCtx(), publisher, nil, true, VisitChange{ActiveGroupID: "ignored", StudentID: "99", EducationGroupID: &education, Source: "toggle"})
	calls := publisher.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "edu:7", calls[0].Topic)
	assert.Equal(t, "student_checkin", string(calls[0].Event.Type))
	assert.Empty(t, calls[0].Event.ActiveGroupID, "a roomless change carries no session")
	require.NotNil(t, calls[0].Event.Data.Source)
	assert.Equal(t, "toggle", *calls[0].Event.Data.Source)
}

func TestPublishBulkStudentChangeSplitsSessionAndEducationTopics(t *testing.T) {
	t.Parallel()
	publisher := testpkg.NewRecordingBroadcaster()
	PublishBulkStudentChange(presenceCtx(), publisher, nil, false, "12", []string{"1", "2"}, []string{"7"})
	PublishBulkStudentChangeToEducationGroup(presenceCtx(), publisher, nil, true, "", 7, []string{"2"})

	calls := publisher.Calls()
	require.Len(t, calls, 2)
	assert.Equal(t, "12", calls[0].Topic)
	assert.Equal(t, "bulk_student_checkout", string(calls[0].Event.Type))
	require.NotNil(t, calls[0].Event.Data.StudentIDs)
	assert.Equal(t, []string{"1", "2"}, *calls[0].Event.Data.StudentIDs)
	require.NotNil(t, calls[0].Event.Data.GroupIDs)
	assert.Equal(t, []string{"7"}, *calls[0].Event.Data.GroupIDs)

	assert.Equal(t, "edu:7", calls[1].Topic)
	assert.Equal(t, "bulk_student_checkin", string(calls[1].Event.Type))
	assert.Empty(t, calls[1].Event.ActiveGroupID)
	require.NotNil(t, calls[1].Event.Data.StudentIDs)
	assert.Equal(t, []string{"2"}, *calls[1].Event.Data.StudentIDs)
	require.NotNil(t, calls[1].Event.Data.GroupIDs)
	assert.Equal(t, []string{"7"}, *calls[1].Event.Data.GroupIDs)
}

func TestPublishActivityLifecycleCarriesRoomFacts(t *testing.T) {
	t.Parallel()
	publisher := testpkg.NewRecordingBroadcaster()
	PublishActivityStart(presenceCtx(), publisher, nil, ActivitySession{ActiveGroupID: "12", ActivityName: "Fußball", RoomID: "3", RoomName: "Halle", SupervisorIDs: []string{"5"}})
	PublishActivityEnd(presenceCtx(), publisher, nil, ActivitySession{ActiveGroupID: "12", ActivityName: "Fußball", RoomID: "3", RoomName: "Halle"})

	calls := publisher.Calls()
	require.Len(t, calls, 2)
	assert.Equal(t, "activity_start", string(calls[0].Event.Type))
	require.NotNil(t, calls[0].Event.Data.SupervisorIDs)
	assert.Equal(t, []string{"5"}, *calls[0].Event.Data.SupervisorIDs)
	assert.Equal(t, "activity_end", string(calls[1].Event.Type))
	assert.Nil(t, calls[1].Event.Data.SupervisorIDs)
	for _, call := range calls {
		assert.Equal(t, "12", call.Topic)
		require.NotNil(t, call.Event.Data.ActivityName)
		assert.Equal(t, "Fußball", *call.Event.Data.ActivityName)
		require.NotNil(t, call.Event.Data.RoomID)
		assert.Equal(t, "3", *call.Event.Data.RoomID)
		require.NotNil(t, call.Event.Data.RoomName)
		assert.Equal(t, "Halle", *call.Event.Data.RoomName)
	}
}

func TestTenantRefreshesScopeOptionally(t *testing.T) {
	t.Parallel()
	publisher := testpkg.NewRecordingBroadcaster()
	PublishSupervisionRefresh(presenceCtx(), publisher, nil, "12", "student_moved", []string{"7"})
	PublishDashboardCountsChanged(presenceCtx(), publisher, nil, nil)

	calls := publisher.CallsByMethod("tenant")
	require.Len(t, calls, 2)
	assert.Equal(t, "dashboard_counts_changed", string(calls[0].Event.Type))
	assert.Equal(t, "12", calls[0].Event.ActiveGroupID)
	require.NotNil(t, calls[0].Event.Data.Reason)
	assert.Equal(t, "student_moved", *calls[0].Event.Data.Reason)
	require.NotNil(t, calls[0].Event.Data.GroupIDs)
	assert.Equal(t, []string{"7"}, *calls[0].Event.Data.GroupIDs)

	assert.Equal(t, "dashboard_counts_changed", string(calls[1].Event.Type))
	assert.Empty(t, calls[1].Event.ActiveGroupID)
	assert.Nil(t, calls[1].Event.Data.Reason)
	assert.Nil(t, calls[1].Event.Data.GroupIDs)
}

func TestPresenceProducersIgnoreMissingPublisher(t *testing.T) {
	t.Parallel()
	ctx := presenceCtx()
	assert.NotPanics(t, func() {
		PublishVisitCheckIn(ctx, nil, nil, VisitChange{ActiveGroupID: "12", StudentID: "99"})
		PublishBulkStudentChange(ctx, nil, nil, true, "12", nil, nil)
		PublishActivityStart(ctx, nil, nil, ActivitySession{ActiveGroupID: "12"})
		PublishSupervisionRefresh(ctx, nil, nil, "12", "student_moved", nil)
		PublishDashboardCountsChanged(ctx, nil, nil, nil)
	})
}
