package compose_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This double observes the consumer's ID set and models a room disappearing
// from the Facilities result without changing the timetable occurrence.
type slotListRoomDouble struct {
	list func(context.Context, []int64) ([]facilities.Room, error)
}

func (r slotListRoomDouble) ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error) {
	return r.list(ctx, ids)
}

func TestSlotListsRoomEnrichmentPreservesSelectionAndMissingNames(t *testing.T) {
	t.Parallel()
	f := buildMensaFixture(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, f.db, "SL-Batch-Shared")
	missing := testpkg.CreateTestRoom(t, f.db, "SL-Batch-Missing")
	cancelled := testpkg.CreateTestRoom(t, f.db, "SL-Batch-Cancelled")
	otherKind := testpkg.CreateTestRoom(t, f.db, "SL-Batch-OtherKind")
	insert := func(title string, roomID int64, status string, kind classday.ListKind) int64 {
		listKind := string(kind)
		inst := &testpkg.ActivityInstance{
			Date: testpkg.ScheduleDate(listDate), Title: title, RoomID: roomID, Status: status, ListKind: &listKind,
			StartTime: atOn(listDate, 11, 30), EndTime: atOn(listDate, 13, 0),
		}
		inst.TenantID = testpkg.Tenant(t)
		testpkg.InsertActivityInstanceRow(t, ctx, f.db, inst)
		return inst.ID
	}
	firstID := insert("shared first", room.ID, testpkg.ScheduleInstanceStatusPlanned, classday.ListKindMensa)
	secondID := insert("shared second", room.ID, testpkg.ScheduleInstanceStatusPlanned, classday.ListKindMensa)
	missingID := insert("missing", missing.ID, testpkg.ScheduleInstanceStatusPlanned, classday.ListKindMensa)
	insert("cancelled", cancelled.ID, testpkg.ScheduleInstanceStatusCancelled, classday.ListKindMensa)
	otherID := insert("other", otherKind.ID, testpkg.ScheduleInstanceStatusPlanned, classday.ListKindLearningTime)
	var calls [][]int64
	rooms := newTestRoomRepository(f.db)
	svc := newTestServiceWithCustomRoomRepo(f.db, slotListRoomDouble{list: func(ctx context.Context, ids []int64) ([]facilities.Room, error) {
		calls = append(calls, append([]int64(nil), ids...))
		assert.NotContains(t, ids, cancelled.ID)
		assert.Len(t, ids, len(int64IDSet(ids)), "shared rooms must be requested once")
		values, err := rooms.ListRoomsByID(ctx, ids)
		filtered := make([]facilities.Room, 0, len(values))
		for _, value := range values {
			if value.ID != missing.ID {
				filtered = append(filtered, value)
			}
		}
		return filtered, err
	}}, stubSlotListSettings{})
	result, err := svc.BuildList(ctx, classday.Params{
		Date: classday.Date(listDate.String()), Target: classday.TargetSlots, Source: classday.SourcePlanned,
		ListKind: classday.ListKindMensa, InstanceIDsSet: true, InstanceIDs: []int64{f.instanceID},
	})
	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.Len(t, calls[0], 3)
	assert.NotContains(t, calls[0], otherKind.ID)
	require.Len(t, result.Slots, 4, "unselected matching slots remain published")
	assertSlotRoomNames(t, result.Slots, map[int64]string{firstID: room.Name, secondID: room.Name, missingID: ""})
	for _, row := range result.Rows {
		assert.Equal(t, f.instanceID, row.InstanceID)
	}
	options, err := svc.ListOptions(ctx, classday.Date(listDate.String()))
	require.NoError(t, err)
	require.Len(t, calls, 2)
	assert.Len(t, calls[1], 4)
	require.Len(t, options.Slots, 5)
	assertSlotRoomNames(t, options.Slots, map[int64]string{firstID: room.Name, secondID: room.Name, missingID: "", otherID: otherKind.Name})
}

func int64IDSet(ids []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func assertSlotRoomNames(t *testing.T, slots []classday.Slot, want map[int64]string) {
	t.Helper()
	got := make(map[int64]string, len(slots))
	for _, slot := range slots {
		got[slot.InstanceID] = slot.RoomName
	}
	for id, name := range want {
		require.Contains(t, got, id)
		assert.Equal(t, name, got[id])
	}
}

func TestSlotListsCancelledRoomsAreNotRead(t *testing.T) {
	t.Parallel()
	f := buildMensaFixture(t)
	ctx := testpkg.Ctx(t)
	_, err := f.db.NewUpdate().TableExpr("schedule.activity_instances").
		Set("status = ?", testpkg.ScheduleInstanceStatusCancelled).
		Where("id = ? AND tenant_id = ?", f.instanceID, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	svc := newTestServiceWithCustomRoomRepo(f.db, failingRoomRepo{err: errors.New("room read must not run")}, stubSlotListSettings{})
	options, err := svc.ListOptions(ctx, classday.Date(listDate.String()))
	require.NoError(t, err)
	assert.Empty(t, options.Slots)
	result, err := svc.BuildList(ctx, classday.Params{
		Date: classday.Date(listDate.String()), Target: classday.TargetSlots, Source: classday.SourcePlanned,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Slots)
}
