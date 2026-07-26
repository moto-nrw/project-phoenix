package schedule

// Mock-based unit tests for the StaffAssignmentService ("Mein Tag", #1844).
// The integration test pins the happy path against a real DB; these cover the
// validation guards, the repository error propagation, and the enrichment edge
// cases (room override, vanished activity group, missing instance, cross-date
// sort) without a database. int64 literals are fake in-memory IDs.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each mock embeds its full repository interface and overrides only the one
// method the service calls, so the struct satisfies the interface with minimal
// boilerplate (unused methods panic if ever called, which they are not).

type asInstanceStaffRepo struct {
	scheduleModels.InstanceStaffRepository
	rows []*scheduleModels.InstanceStaff
	err  error
}

func (r *asInstanceStaffRepo) FindByStaffAndDateRange(context.Context, int64, timezone.Date, timezone.Date) ([]*scheduleModels.InstanceStaff, error) {
	return r.rows, r.err
}

type asInstanceRepo struct {
	scheduleModels.ActivityInstanceRepository
	instances []*scheduleModels.ActivityInstance
	err       error
}

func (r *asInstanceRepo) FindByIDs(context.Context, []int64) ([]*scheduleModels.ActivityInstance, error) {
	return r.instances, r.err
}

type asRoomRepo struct {
	facilitiesModels.RoomRepository
	rooms []*facilitiesModels.Room
	err   error
}

func (r *asRoomRepo) FindByIDs(context.Context, []int64) ([]*facilitiesModels.Room, error) {
	return r.rooms, r.err
}

type asGroupRepo struct {
	activitiesModels.GroupRepository
	groups []*activitiesModels.Group
	err    error
}

func (r *asGroupRepo) FindByIDs(context.Context, []int64) ([]*activitiesModels.Group, error) {
	return r.groups, r.err
}

func assignmentServiceFixture(is *asInstanceStaffRepo, ir *asInstanceRepo, rr *asRoomRepo, gr *asGroupRepo) StaffAssignmentService {
	return NewStaffAssignmentService(StaffAssignmentDependencies{
		InstanceStaffRepo:    is,
		ActivityInstanceRepo: ir,
		RoomRepo:             rr,
		ActivityGroupRepo:    gr,
	}, nil)
}

func instanceStaffRow(instanceID, staffID int64) *scheduleModels.InstanceStaff {
	row := &scheduleModels.InstanceStaff{
		InstanceID: instanceID,
		StaffID:    staffID,
	}
	return row
}

func TestAssignmentService_RejectsInvalidStaffID(t *testing.T) {
	svc := assignmentServiceFixture(&asInstanceStaffRepo{}, &asInstanceRepo{}, &asRoomRepo{}, &asGroupRepo{})
	date := timezone.NewDate(2026, time.March, 10)

	_, err := svc.ListAssignmentsForStaff(context.Background(), 0, date, date)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

func TestAssignmentService_RejectsInvalidRange(t *testing.T) {
	svc := assignmentServiceFixture(&asInstanceStaffRepo{}, &asInstanceRepo{}, &asRoomRepo{}, &asGroupRepo{})

	_, err := svc.ListAssignmentsForStaff(context.Background(), 7, timezone.Date{}, timezone.NewDate(2026, time.March, 10))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrShiftInvalid)
}

func TestAssignmentService_PropagatesInstanceStaffError(t *testing.T) {
	svc := assignmentServiceFixture(
		&asInstanceStaffRepo{err: errors.New("staff read failed")},
		&asInstanceRepo{}, &asRoomRepo{}, &asGroupRepo{},
	)
	date := timezone.NewDate(2026, time.March, 10)

	_, err := svc.ListAssignmentsForStaff(context.Background(), 7, date, date)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "staff read failed")
}

func TestAssignmentService_EmptyWhenNoRows(t *testing.T) {
	svc := assignmentServiceFixture(&asInstanceStaffRepo{rows: nil}, &asInstanceRepo{}, &asRoomRepo{}, &asGroupRepo{})
	date := timezone.NewDate(2026, time.March, 10)

	got, err := svc.ListAssignmentsForStaff(context.Background(), 7, date, date)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestAssignmentService_PropagatesInstanceLoadError(t *testing.T) {
	svc := assignmentServiceFixture(
		&asInstanceStaffRepo{rows: []*scheduleModels.InstanceStaff{instanceStaffRow(1, 7)}},
		&asInstanceRepo{err: errors.New("instance load failed")},
		&asRoomRepo{}, &asGroupRepo{},
	)
	date := timezone.NewDate(2026, time.March, 10)

	_, err := svc.ListAssignmentsForStaff(context.Background(), 7, date, date)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instance load failed")
}

func TestAssignmentService_PropagatesRoomError(t *testing.T) {
	inst := &scheduleModels.ActivityInstance{Title: "Lernzeit", RoomID: 100, StartTime: wall(10, 0), EndTime: wall(11, 0)}
	inst.ID = 1
	svc := assignmentServiceFixture(
		&asInstanceStaffRepo{rows: []*scheduleModels.InstanceStaff{instanceStaffRow(1, 7)}},
		&asInstanceRepo{instances: []*scheduleModels.ActivityInstance{inst}},
		&asRoomRepo{err: errors.New("room read failed")},
		&asGroupRepo{},
	)
	date := timezone.NewDate(2026, time.March, 10)

	_, err := svc.ListAssignmentsForStaff(context.Background(), 7, date, date)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "room read failed")
}

func TestAssignmentService_PropagatesGroupError(t *testing.T) {
	groupID := int64(50)
	inst := &scheduleModels.ActivityInstance{Title: "Lernzeit", RoomID: 100, ActivityGroupID: &groupID, StartTime: wall(10, 0), EndTime: wall(11, 0)}
	inst.ID = 1
	svc := assignmentServiceFixture(
		&asInstanceStaffRepo{rows: []*scheduleModels.InstanceStaff{instanceStaffRow(1, 7)}},
		&asInstanceRepo{instances: []*scheduleModels.ActivityInstance{inst}},
		&asRoomRepo{rooms: []*facilitiesModels.Room{{Model: modelBase.Model{ID: 100}, Name: "Raum A"}}},
		&asGroupRepo{err: errors.New("group read failed")},
	)
	date := timezone.NewDate(2026, time.March, 10)

	_, err := svc.ListAssignmentsForStaff(context.Background(), 7, date, date)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "group read failed")
}

// TestAssignmentService_EnrichesAndSorts exercises the full enrichment: a
// cross-date sort, a per-row room override, a spontaneous (group-less) block, a
// vanished activity group (warned + name omitted), and a row whose instance was
// not returned (skipped). It also drives the nil-logger getLogger() path via
// the vanished-group warning.
func TestAssignmentService_EnrichesAndSorts(t *testing.T) {
	grp1 := int64(50)
	grp2 := int64(51) // referenced but not returned → vanished
	override := int64(200)

	instA := &scheduleModels.ActivityInstance{ // later date, primary block, resolved group
		Date: timezone.NewDate(2026, time.March, 11), Title: "Lernzeit",
		ActivityGroupID: &grp1, RoomID: 100, StartTime: wall(9, 0), EndTime: wall(10, 0),
		Status: scheduleModels.InstanceStatusPlanned,
	}
	instA.ID = 1
	cancelReason := "Ausflug"
	instB := &scheduleModels.ActivityInstance{ // earlier date, cancelled, vanished group, room override
		Date: timezone.NewDate(2026, time.March, 10), Title: "Fußball-AG",
		ActivityGroupID: &grp2, RoomID: 100, StartTime: wall(14, 0), EndTime: wall(15, 0),
		Status: scheduleModels.InstanceStatusCancelled, CancelReason: &cancelReason, UnderstaffedAck: true,
	}
	instB.ID = 2
	instC := &scheduleModels.ActivityInstance{ // same date as B, earlier time, spontaneous (no group)
		Date: timezone.NewDate(2026, time.March, 10), Title: "Spontan",
		RoomID: 100, StartTime: wall(8, 0), EndTime: wall(9, 0),
		Status: scheduleModels.InstanceStatusPlanned,
	}
	instC.ID = 3

	rowA := instanceStaffRow(1, 7)
	rowA.IsPrimary = true
	rowB := instanceStaffRow(2, 7)
	rowB.IsSubstitute = true
	rowB.RoomID = &override // per-row room override
	absence := "Krank"
	rowB.AbsenceReason = &absence
	rowB.IsAbsent = true
	rowC := instanceStaffRow(3, 7)
	rowMissing := instanceStaffRow(999, 7) // instance not returned → skipped

	svc := assignmentServiceFixture(
		&asInstanceStaffRepo{rows: []*scheduleModels.InstanceStaff{rowA, rowB, rowC, rowMissing}},
		&asInstanceRepo{instances: []*scheduleModels.ActivityInstance{instA, instB, instC}},
		&asRoomRepo{rooms: []*facilitiesModels.Room{
			{Model: modelBase.Model{ID: 100}, Name: "Raum A"},
			{Model: modelBase.Model{ID: 200}, Name: "Raum B"},
		}},
		&asGroupRepo{groups: []*activitiesModels.Group{
			{Name: "Lernzeit"}, // ID set below
		}},
	)
	// Set the returned group's ID to grp1; grp2 stays absent (vanished).
	svc.(*staffAssignmentService).deps.ActivityGroupRepo.(*asGroupRepo).groups[0].ID = grp1

	got, err := svc.ListAssignmentsForStaff(context.Background(), 7, timezone.NewDate(2026, time.March, 10), timezone.NewDate(2026, time.March, 11))
	require.NoError(t, err)
	require.Len(t, got, 3, "the row referencing a missing instance is skipped")

	// Sorted by date then start time: C (10th 08:00), B (10th 14:00), A (11th 09:00).
	assert.Equal(t, "Spontan", got[0].Title)
	assert.Nil(t, got[0].GroupName, "spontaneous block has no activity group")

	assert.Equal(t, "Fußball-AG", got[1].Title)
	assert.True(t, got[1].Cancelled)
	assert.True(t, got[1].IsSubstitute)
	assert.True(t, got[1].IsAbsent)
	require.NotNil(t, got[1].AbsenceReason)
	assert.Equal(t, "Krank", *got[1].AbsenceReason)
	require.NotNil(t, got[1].CancelReason)
	assert.Equal(t, "Ausflug", *got[1].CancelReason)
	assert.True(t, got[1].UnderstaffedAck)
	assert.Equal(t, "Raum B", got[1].RoomName, "per-row room override wins over the instance room")
	assert.Nil(t, got[1].GroupName, "vanished activity group yields no name")

	assert.Equal(t, "Lernzeit", got[2].Title)
	assert.True(t, got[2].IsPrimary)
	require.NotNil(t, got[2].GroupName)
	assert.Equal(t, "Lernzeit", *got[2].GroupName)
	assert.Equal(t, "Raum A", got[2].RoomName)
}

// TestAssignmentService_ToleratesMissingInstancesAndRooms covers the
// short-circuits where no instances (and therefore no rooms/groups) resolve:
// the room and group id sets are empty, so both resolver batch reads are
// skipped and every row is dropped.
func TestAssignmentService_ToleratesMissingInstancesAndRooms(t *testing.T) {
	svc := assignmentServiceFixture(
		&asInstanceStaffRepo{rows: []*scheduleModels.InstanceStaff{instanceStaffRow(1, 7)}},
		&asInstanceRepo{instances: nil}, // instance not found
		&asRoomRepo{err: errors.New("must not be called")},
		&asGroupRepo{err: errors.New("must not be called")},
	)
	date := timezone.NewDate(2026, time.March, 10)

	got, err := svc.ListAssignmentsForStaff(context.Background(), 7, date, date)
	require.NoError(t, err)
	assert.Empty(t, got)
}
