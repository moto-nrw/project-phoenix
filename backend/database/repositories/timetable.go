package repositories

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

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
	Workforce  workforce.Capability
	// ObserveIdentityAccess records the Identity & Access operations behind
	// the retained operator repositories and account lookups (#2720). The
	// serving root passes its metrics sink; nil composes the module
	// unobserved, which repository tests and CLI roots use.
	ObserveIdentityAccess IdentityAccessObserver
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
	capability, err := NewTimetable(db, students, rooms)
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
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		panic(fmt.Sprintf("compose workforce work-time: %v", err))
	}
	return TimetableDependencies{Capability: capability, Students: students, Groups: groups, Rooms: rooms, Calendar: calendar, Membership: membership, Workforce: workTime}
}

// NewTimetable composes the owner behind legacy repository adapters for test
// and CLI graphs. The production root replaces it with the observed module.
// The session facts come from Student Presence over the same database
// (#2762).
func NewTimetable(db *bun.DB, students peopledirectory.StudentQuery, rooms facilities.Query) (timetable.Capability, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	return timetableCompose.New(timetableCompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB:       db,
		Students: repositoryTimetableStudents{students: students},
		Rooms:    repositoryTimetableRooms{rooms: rooms},
		Sessions: NewPresenceFacts(db),
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
