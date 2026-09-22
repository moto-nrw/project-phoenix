// Package timetabletest composes the Timetable & Activities owner for tests.
package timetabletest

import (
	"context"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type TB interface {
	Helper()
	Fatalf(string, ...any)
}

type TargetStudent struct {
	ID               int64
	SchoolClass      string
	EducationGroupID *int64
	EnrolledUntil    string
}

func New(tb TB, db *bun.DB) timetable.Capability {
	return newModule(tb, db, timetableCompose.StudentDirectoryFunc(func(context.Context) ([]timetableCompose.TargetStudent, error) {
		return []timetableCompose.TargetStudent{}, nil
	}))
}

func newModule(tb TB, db *bun.DB, students timetableCompose.StudentDirectory, rooms ...timetable.RoomDirectory) timetable.Capability {
	return newModuleWithSessions(tb, db, students, NoSessions{}, rooms...)
}

func newModuleWithSessions(tb TB, db *bun.DB, students timetableCompose.StudentDirectory, sessions timetable.SessionFacts, rooms ...timetable.RoomDirectory) timetable.Capability {
	tb.Helper()
	roomDirectory := testRooms()
	if len(rooms) > 0 {
		roomDirectory = rooms[0]
	}
	capability, err := timetableCompose.New(timetableCompose.Dependencies{
		// This owner-only harness stubs foreign capabilities: no block has
		// started and nothing was observed. Lifecycle integration tests
		// compose the real Student Presence owner and the Membership locker.
		LockStaffAssignment: func(context.Context, int64) error { return nil },
		DB:                  db,
		Students:            students,
		Rooms:               roomDirectory,
		Sessions:            sessions,
		Observe:             func(timetableCompose.Observation) {},
	})
	if err != nil {
		tb.Fatalf("compose test Timetable & Activities: %v", err)
	}
	return capability
}

func testRooms() timetable.RoomDirectory {
	return timetable.RoomDirectoryFunc(func(context.Context, []int64) ([]timetable.RoomRef, error) {
		return []timetable.RoomRef{}, nil
	})
}

// NoSessions is the session facts port of a school where no block has
// started and no presence was observed.
type NoSessions struct{}

func (NoSessions) StartedInstanceIDs(context.Context, []int64) ([]int64, error)     { return nil, nil }
func (NoSessions) CompletedInstanceIDs(context.Context, []int64) ([]int64, error)   { return nil, nil }
func (NoSessions) ObservedParticipantIDs(context.Context, []int64) ([]int64, error) { return nil, nil }
func (NoSessions) NotScheduledParticipantIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (NoSessions) ExecutionFacts(context.Context, []int64, []int64) ([]int64, []int64, error) {
	return nil, nil, nil
}

// FixedSessions is the session facts port with the answers a test fixes up
// front: which blocks started or ended, which participants were observed or
// marked as not scheduled.
type FixedSessions struct {
	Started, Completed, Observed, NotScheduled []int64
}

func intersectIDs(wanted, known []int64) []int64 {
	result := make([]int64, 0, len(known))
	for _, id := range known {
		if slices.Contains(wanted, id) {
			result = append(result, id)
		}
	}
	return result
}

func (s *FixedSessions) StartedInstanceIDs(_ context.Context, ids []int64) ([]int64, error) {
	return intersectIDs(ids, append(slices.Clone(s.Started), s.Completed...)), nil
}
func (s *FixedSessions) CompletedInstanceIDs(_ context.Context, ids []int64) ([]int64, error) {
	return intersectIDs(ids, s.Completed), nil
}
func (s *FixedSessions) ObservedParticipantIDs(_ context.Context, ids []int64) ([]int64, error) {
	return intersectIDs(ids, s.Observed), nil
}
func (s *FixedSessions) NotScheduledParticipantIDs(_ context.Context, ids []int64) ([]int64, error) {
	return intersectIDs(ids, s.NotScheduled), nil
}
func (s *FixedSessions) ExecutionFacts(_ context.Context, instanceIDs, participantIDs []int64) ([]int64, []int64, error) {
	return intersectIDs(instanceIDs, s.Completed), intersectIDs(participantIDs, s.NotScheduled), nil
}

// NewWithSessions composes the owner over the given session facts.
func NewWithSessions(tb TB, db *bun.DB, sessions timetable.SessionFacts) timetable.Capability {
	return newModuleWithSessions(tb, db, timetableCompose.StudentDirectoryFunc(func(context.Context) ([]timetableCompose.TargetStudent, error) {
		return []timetableCompose.TargetStudent{}, nil
	}), sessions)
}

// LegacyRows reads blocks with their execution and participants with their
// attendance in one statement each, the way the retained repositories do
// since the presence cutover (#2762).
type LegacyRows struct {
	reads timetableCompose.PresenceReads
}

func NewLegacyRows(tb TB, db *bun.DB) LegacyRows {
	tb.Helper()
	reads, err := timetableCompose.NewPresenceReads(db)
	if err != nil {
		tb.Fatalf("compose timetable presence reads: %v", err)
	}
	return LegacyRows{reads: reads}
}

// LegacyInstance is one block of a day with its execution, as the retained
// legacy rows show it.
type LegacyInstance struct {
	ID            int64
	Title         string
	Date          string
	StartTime     time.Time
	EndTime       time.Time
	RoomID        int64
	Status        string
	ActiveGroupID *int64
	ListKind      *string
}

// LegacyParticipant is one roster row with its attendance, as the retained
// legacy rows show it.
type LegacyParticipant struct {
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
func (r LegacyRows) InstancesOn(ctx context.Context, date string) ([]LegacyInstance, error) {
	rows, err := r.reads.ListLegacyInstances(ctx, timetableCompose.LegacyInstanceFilter{Date: &date, OrderByDateAndTime: true})
	if err != nil {
		return nil, err
	}
	result := make([]LegacyInstance, 0, len(rows))
	for _, row := range rows {
		result = append(result, LegacyInstance{
			ID: row.ID, Title: row.Title, Date: row.Date.String(), StartTime: row.StartTime, EndTime: row.EndTime,
			RoomID: row.RoomID, Status: row.Status, ActiveGroupID: row.ActiveGroupID, ListKind: row.ListKind,
		})
	}
	return result, nil
}

// Roster lists the participants of the blocks with their attendance, ordered
// by block and participant.
func (r LegacyRows) Roster(ctx context.Context, instanceIDs []int64) ([]LegacyParticipant, error) {
	rows, err := r.reads.ListLegacyParticipants(ctx, timetableCompose.LegacyParticipantFilter{InstanceIDs: instanceIDs, OrderByInstanceStudent: true})
	if err != nil {
		return nil, err
	}
	result := make([]LegacyParticipant, 0, len(rows))
	for _, row := range rows {
		result = append(result, LegacyParticipant{
			ID: row.ID, InstanceID: row.InstanceID, StudentID: row.StudentID, RoomID: row.RoomID,
			Status: row.Status, Substatus: row.Substatus, Note: row.Note, CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt,
			IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
		})
	}
	return result, nil
}
