//go:build testhooks

package owners

import (
	"context"
	"slices"
	"time"

	directoryPorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	enrollmentProvenanceView "github.com/moto-nrw/project-phoenix/modules/timetableenrollmentprovenance/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

type TestTimetableTB interface {
	Helper()
	Fatalf(string, ...any)
}

func NewTestTimetable(tb TestTimetableTB, db transactionruntime.Database) timetable.Capability {
	return newTestTimetable(tb, db, directoryPorts.StudentDirectoryFunc(func(context.Context) ([]directoryPorts.TargetStudent, error) {
		return []directoryPorts.TargetStudent{}, nil
	}))
}

func newTestTimetable(tb TestTimetableTB, db transactionruntime.Database, students directoryPorts.StudentDirectory, rooms ...timetable.RoomDirectory) timetable.Capability {
	return newTestTimetableWithSessions(tb, db, students, TestTimetableNoSessions{}, rooms...)
}

func newTestTimetableWithSessions(tb TestTimetableTB, db transactionruntime.Database, students directoryPorts.StudentDirectory, sessions timetable.SessionFacts, rooms ...timetable.RoomDirectory) timetable.Capability {
	tb.Helper()
	roomDirectory := testTimetableRooms()
	if len(rooms) > 0 {
		roomDirectory = rooms[0]
	}
	capability, err := timetableCompose.New(timetableCompose.Dependencies{
		// This owner-only harness stubs foreign capabilities: no block has
		// started and nothing was observed. Lifecycle integration tests
		// compose the real Student Presence owner and the Membership locker.
		LockStaffAssignment: func(context.Context, int64) error { return nil },
		DB:                  db.Pool(),
		Students:            students,
		Rooms:               roomDirectory,
		Sessions:            sessions,
		Observe:             func(timetableCompose.Observation) {},
		Provenance:          enrollmentProvenanceView.NewReader(),
	})
	if err != nil {
		tb.Fatalf("compose test Timetable & Activities: %v", err)
	}
	return capability
}

func testTimetableRooms() timetable.RoomDirectory {
	return timetable.RoomDirectoryFunc(func(context.Context, []int64) ([]timetable.RoomRef, error) {
		return []timetable.RoomRef{}, nil
	})
}

// TestTimetableNoSessions is the session facts port of a school where no block has
// started and no presence was observed.
type TestTimetableNoSessions struct{}

func (TestTimetableNoSessions) StartedInstanceIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (TestTimetableNoSessions) CompletedInstanceIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (TestTimetableNoSessions) ObservedParticipantIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (TestTimetableNoSessions) NotScheduledParticipantIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (TestTimetableNoSessions) ExecutionFacts(context.Context, []int64, []int64) ([]int64, []int64, error) {
	return nil, nil, nil
}
func (TestTimetableNoSessions) RunningSessionsByInstance(context.Context, []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

// TestTimetableFixedSessions is the session facts port with the answers a test fixes up
// front: which blocks started or ended, which participants were observed or
// marked as not scheduled.
type TestTimetableFixedSessions struct {
	Started, Completed, Observed, NotScheduled []int64
}

func testTimetableIntersectIDs(wanted, known []int64) []int64 {
	result := make([]int64, 0, len(known))
	for _, id := range known {
		if slices.Contains(wanted, id) {
			result = append(result, id)
		}
	}
	return result
}

func (s *TestTimetableFixedSessions) StartedInstanceIDs(_ context.Context, ids []int64) ([]int64, error) {
	return testTimetableIntersectIDs(ids, append(slices.Clone(s.Started), s.Completed...)), nil
}
func (s *TestTimetableFixedSessions) CompletedInstanceIDs(_ context.Context, ids []int64) ([]int64, error) {
	return testTimetableIntersectIDs(ids, s.Completed), nil
}
func (s *TestTimetableFixedSessions) ObservedParticipantIDs(_ context.Context, ids []int64) ([]int64, error) {
	return testTimetableIntersectIDs(ids, s.Observed), nil
}
func (s *TestTimetableFixedSessions) NotScheduledParticipantIDs(_ context.Context, ids []int64) ([]int64, error) {
	return testTimetableIntersectIDs(ids, s.NotScheduled), nil
}
func (s *TestTimetableFixedSessions) ExecutionFacts(_ context.Context, instanceIDs, participantIDs []int64) ([]int64, []int64, error) {
	return testTimetableIntersectIDs(instanceIDs, s.Completed), testTimetableIntersectIDs(participantIDs, s.NotScheduled), nil
}
func (s *TestTimetableFixedSessions) RunningSessionsByInstance(context.Context, []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

// NewTestTimetableWithSessions composes the owner over the given session facts.
func NewTestTimetableWithSessions(tb TestTimetableTB, db transactionruntime.Database, sessions timetable.SessionFacts) timetable.Capability {
	return newTestTimetableWithSessions(tb, db, directoryPorts.StudentDirectoryFunc(func(context.Context) ([]directoryPorts.TargetStudent, error) {
		return []directoryPorts.TargetStudent{}, nil
	}), sessions)
}

// TimetableTestLegacyRows reads blocks with their execution and participants with their
// attendance in one statement each, the way the retained repositories do
// since the presence cutover (#2762).
type TimetableTestLegacyRows struct {
	reads timetableCompose.PresenceReads
}

func NewTimetableTestLegacyRows(tb TestTimetableTB, projection directoryPorts.PresenceProjection) TimetableTestLegacyRows {
	tb.Helper()
	reads, err := timetableCompose.NewPresenceReads(projection)
	if err != nil {
		tb.Fatalf("compose timetable presence reads: %v", err)
	}
	return TimetableTestLegacyRows{reads: reads}
}

// TimetableTestLegacyInstance is one block of a day with its execution, as the retained
// legacy rows show it.
type TimetableTestLegacyInstance struct {
	ID               int64
	Title            string
	Date             string
	StartTime        time.Time
	EndTime          time.Time
	RoomID           int64
	Status           string
	ActiveGroupID    *int64
	ListKind         *string
	ActivityGroupID  *int64
	CancelReason     *string
	Notes            *string
	UnderstaffedNote *string
}

// TimetableTestLegacyParticipant is one roster row with its attendance, as the retained
// legacy rows show it.
type TimetableTestLegacyParticipant struct {
	ID                 int64
	InstanceID         int64
	StudentID          int64
	RoomID             *int64
	Status             string
	Substatus          *string
	Note               *string
	CheckedInAt        *time.Time
	CheckedOutAt       *time.Time
	IsUnplanned        bool
	NotScheduled       bool
	ManualStatusAt     *time.Time
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}

// InstancesOn lists the blocks of the ISO day with their execution, ordered
// by start time.
func (r TimetableTestLegacyRows) InstancesOn(ctx context.Context, date string) ([]TimetableTestLegacyInstance, error) {
	return r.instances(ctx, directoryPorts.LegacyInstanceFilter{Date: &date, OrderByDateAndTime: true})
}

// InstancesBetween lists the blocks of the ISO days from through to with
// their execution, ordered by date and start time: the tenant's date-range
// read the composition root binds for the plan exports.
func (r TimetableTestLegacyRows) InstancesBetween(ctx context.Context, from, to string) ([]TimetableTestLegacyInstance, error) {
	return r.instances(ctx, directoryPorts.LegacyInstanceFilter{FromDate: &from, ToDate: &to, OrderByDateAndTime: true})
}

// InstancesByID lists the given blocks with their execution, ordered by date
// and start time.
func (r TimetableTestLegacyRows) InstancesByID(ctx context.Context, ids []int64) ([]TimetableTestLegacyInstance, error) {
	if len(ids) == 0 {
		return []TimetableTestLegacyInstance{}, nil
	}
	return r.instances(ctx, directoryPorts.LegacyInstanceFilter{IDs: ids, OrderByDateAndTime: true})
}

func (r TimetableTestLegacyRows) instances(ctx context.Context, filter directoryPorts.LegacyInstanceFilter) ([]TimetableTestLegacyInstance, error) {
	rows, err := r.reads.ListLegacyInstances(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]TimetableTestLegacyInstance, 0, len(rows))
	for _, row := range rows {
		result = append(result, TimetableTestLegacyInstance{
			ID: row.ID, Title: row.Title, Date: row.Date.String(), StartTime: row.StartTime, EndTime: row.EndTime,
			RoomID: row.RoomID, Status: row.Status, ActiveGroupID: row.ActiveGroupID, ListKind: row.ListKind,
			ActivityGroupID: row.ActivityGroupID, CancelReason: row.CancelReason, Notes: row.Notes,
			UnderstaffedNote: row.UnderstaffedNote,
		})
	}
	return result, nil
}

// Roster lists the participants of the blocks with their attendance, ordered
// by block and participant.
func (r TimetableTestLegacyRows) Roster(ctx context.Context, instanceIDs []int64) ([]TimetableTestLegacyParticipant, error) {
	rows, err := r.reads.ListLegacyParticipants(ctx, directoryPorts.LegacyParticipantFilter{InstanceIDs: instanceIDs, OrderByInstanceStudent: true})
	if err != nil {
		return nil, err
	}
	result := make([]TimetableTestLegacyParticipant, 0, len(rows))
	for _, row := range rows {
		result = append(result, TimetableTestLegacyParticipant{
			ID: row.ID, InstanceID: row.InstanceID, StudentID: row.StudentID, RoomID: row.RoomID,
			Status: row.Status, Substatus: row.Substatus, Note: row.Note, CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt,
			IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
		})
	}
	return result, nil
}
