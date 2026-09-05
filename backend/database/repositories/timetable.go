package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// NewTimetable composes the owner behind legacy repository adapters for test
// and CLI graphs. The production root replaces it with the observed module.
func NewTimetable(db *bun.DB, students peopledirectory.StudentQuery, rooms facilities.Query, careDays timetable.CareDayLocker) (timetable.Capability, error) {
	return timetableCompose.New(timetableCompose.Dependencies{
		DB:       db,
		Students: repositoryTimetableStudents{students: students},
		Rooms:    repositoryTimetableRooms{rooms: rooms},
		CareDays: careDays,
		Observe:  func(timetableCompose.Observation) {},
	})
}

type repositoryTimetableStudents struct{ students peopledirectory.StudentQuery }

func (d repositoryTimetableStudents) ListEnrolledStudents(ctx context.Context) ([]timetableCompose.TargetStudent, error) {
	values, err := d.students.ListEnrolledStudents(ctx)
	result := make([]timetableCompose.TargetStudent, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.TargetStudent{
			ID: value.ID, SchoolClass: value.SchoolClass, EducationGroupID: value.GroupID,
			EnrolledUntil: value.EnrolledUntil,
		})
	}
	return result, err
}

type repositoryTimetableRooms struct{ rooms facilities.Query }

func (d repositoryTimetableRooms) LockRoomsByID(ctx context.Context, ids []int64) ([]timetable.RoomRef, error) {
	values, err := d.rooms.LockRoomsByID(ctx, ids)
	result := make([]timetable.RoomRef, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.RoomRef{ID: value.ID, TenantID: value.TenantID})
	}
	return result, err
}
