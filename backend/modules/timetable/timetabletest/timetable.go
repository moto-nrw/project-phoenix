// Package timetabletest composes the Timetable & Activities owner for tests.
package timetabletest

import (
	"context"
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

type StudentDirectoryFunc func(context.Context) ([]TargetStudent, error)

type InstanceStudent struct {
	ID                 int64
	TenantID           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
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

type InstanceStudents struct {
	capability timetable.InstanceStudentCapability
}

func NewInstanceStudents(tb TB, db *bun.DB) InstanceStudents {
	return InstanceStudents{capability: New(tb, db)}
}

func (r InstanceStudents) Create(ctx context.Context, value InstanceStudent) (InstanceStudent, error) {
	created, err := r.capability.CreateInstanceStudent(ctx, instanceStudentInput(value))
	return testInstanceStudent(created), err
}

func (r InstanceStudents) Update(ctx context.Context, value InstanceStudent) (InstanceStudent, error) {
	updated, err := r.capability.UpdateInstanceStudent(ctx, value.ID, instanceStudentInput(value))
	return testInstanceStudent(updated), err
}

func (r InstanceStudents) ListByInstanceIDs(ctx context.Context, ids []int64) ([]InstanceStudent, error) {
	values, err := r.capability.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: ids})
	result := make([]InstanceStudent, 0, len(values))
	for _, value := range values {
		result = append(result, testInstanceStudent(value))
	}
	return result, err
}

func instanceStudentInput(value InstanceStudent) timetable.InstanceStudentInput {
	return timetable.InstanceStudentInput{InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID,
		Status: value.Status, Substatus: value.Substatus, Note: value.Note, CheckedInAt: value.CheckedInAt,
		CheckedOutAt: value.CheckedOutAt, IsUnplanned: value.IsUnplanned, NotScheduled: value.NotScheduled,
		ManualStatusAt: value.ManualStatusAt, StudentStatusDayID: value.StudentStatusDayID, PickupExceptionID: value.PickupExceptionID}
}

func testInstanceStudent(value timetable.InstanceStudent) InstanceStudent {
	return InstanceStudent{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID, Status: value.Status,
		Substatus: value.Substatus, Note: value.Note, CheckedInAt: value.CheckedInAt, CheckedOutAt: value.CheckedOutAt,
		IsUnplanned: value.IsUnplanned, NotScheduled: value.NotScheduled, ManualStatusAt: value.ManualStatusAt,
		StudentStatusDayID: value.StudentStatusDayID, PickupExceptionID: value.PickupExceptionID}
}

func New(tb TB, db *bun.DB) timetable.Capability {
	return newModule(tb, db, timetableCompose.StudentDirectoryFunc(func(context.Context) ([]timetableCompose.TargetStudent, error) {
		return []timetableCompose.TargetStudent{}, nil
	}))
}

func NewWithStudentDirectory(tb TB, db *bun.DB, students StudentDirectoryFunc) timetable.Capability {
	if students == nil {
		tb.Fatalf("compose test Timetable & Activities: student directory is required")
	}
	return NewWithDirectories(tb, db, students, testRooms())
}

func NewWithDirectories(tb TB, db *bun.DB, students StudentDirectoryFunc, rooms timetable.RoomDirectory) timetable.Capability {
	if students == nil || rooms == nil {
		tb.Fatalf("compose test Timetable & Activities: student and room directories are required")
	}
	return newModule(tb, db, timetableCompose.StudentDirectoryFunc(func(ctx context.Context) ([]timetableCompose.TargetStudent, error) {
		values, err := students(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]timetableCompose.TargetStudent, 0, len(values))
		for _, value := range values {
			result = append(result, timetableCompose.TargetStudent(value))
		}
		return result, nil
	}), rooms)
}

func newModule(tb TB, db *bun.DB, students timetableCompose.StudentDirectory, rooms ...timetable.RoomDirectory) timetable.Capability {
	tb.Helper()
	roomDirectory := testRooms()
	if len(rooms) > 0 {
		roomDirectory = rooms[0]
	}
	capability, err := timetableCompose.New(timetableCompose.Dependencies{
		DB:       db,
		CarePlan: unusedCarePlanQueries{},
		Students: students,
		Rooms:    roomDirectory,
		CareDays: timetable.NewCareDayLocker(
			func(context.Context, int64, string) error { return nil },
			func(context.Context, int64, string) error { return nil },
		),
		Observe: func(timetableCompose.Observation) {},
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
