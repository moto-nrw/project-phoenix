package repositories

import (
	"context"
	"fmt"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type TimetableDependencies struct {
	Capability timetable.Capability
	Students   peopledirectory.Capability
	Groups     schoolstructure.Query
	Rooms      facilities.Capability
	Calendar   schoolcalendar.Capability
	Membership schoolmembership.Capability
}

func NewUnobservedTimetableDependencies(db *bun.DB) TimetableDependencies {
	students, err := NewPeopleDirectory(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable students: %v", err))
	}
	rooms, err := NewFacilities(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable rooms: %v", err))
	}
	locks, err := carePlanCompose.NewDayLocks(db, students.LockStudent, peopledirectory.ErrStudentNotFound)
	if err != nil {
		panic(fmt.Sprintf("compose timetable care-day locks: %v", err))
	}
	capability, err := NewTimetable(db, students, rooms, locks)
	if err != nil {
		panic(fmt.Sprintf("compose timetable: %v", err))
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable groups: %v", err))
	}
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable calendar: %v", err))
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("compose timetable membership: %v", err))
	}
	return TimetableDependencies{Capability: capability, Students: students, Groups: groups, Rooms: rooms, Calendar: calendar, Membership: membership}
}

// NewTimetable composes the owner behind legacy repository adapters for test
// and CLI graphs. The production root replaces it with the observed module.
func NewTimetable(db *bun.DB, students peopledirectory.StudentQuery, rooms facilities.Query, careDays timetable.CareDayLocker) (timetable.Capability, error) {
	queries, err := NewTimetableCarePlanQueries(db, func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	return timetableCompose.New(timetableCompose.Dependencies{
		DB:       db,
		Students: repositoryTimetableStudents{students: students},
		Rooms:    repositoryTimetableRooms{rooms: rooms},
		CareDays: careDays,
		CarePlan: queries,
		Observe:  func(timetableCompose.Observation) {},
	})
}

func NewTimetableCarePlanQueries(db *bun.DB, observe func(carePlanCompose.Observation)) (timetable.CarePlanDirectory, error) {
	queries, err := carePlanCompose.NewExceptionQueries(db, observe)
	if err != nil {
		return nil, err
	}
	return timetableCarePlanDirectory{query: pickupExceptionDirectory{query: queries}}, nil
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
