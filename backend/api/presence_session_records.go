package api

import (
	"context"
	"fmt"
	"time"

	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

func newPresenceSessionRecords(deps presenceCompose.SessionRecordDependencies) *presenceCompose.SessionRecords {
	deps.Observe = presenceCompose.FailureLogger()
	deps.Errors = transactionruntime.RecordErrors{}
	records, err := presenceCompose.NewSessionRecords(deps)
	if err != nil {
		panic(fmt.Sprintf("compose presence session records: %v", err))
	}
	return records
}

// SessionRecordOwners are the owners the Student Presence session records
// project their rooms, activities, devices and supervising staff through.
// Devices may be nil: device reads then fail loudly.
type SessionRecordOwners struct {
	// Now fixes the supervision "today"; nil is the Berlin clock.
	Now        func() time.Time
	Devices    devicefleet.Query
	Rooms      facilitiesModule.Query
	Timetable  timetable.Capability
	Membership schoolmembership.Query
	Persons    peopledirectory.Query
}

// sessionRecordDependencies binds every owner directory of the Student
// Presence session records (#2743); the caller adds the database.
func sessionRecordDependencies(owners SessionRecordOwners) presenceCompose.SessionRecordDependencies {
	deps := presenceCompose.SessionRecordDependencies{
		Now: owners.Now, Rooms: activeRoomDirectory{rooms: owners.Rooms},
		Activities: NewSessionActivities(timetableCompose.NewTimetableGroupReads(timetableCompose.NewActivityGroupRecords(owners.Timetable))),
		Staff:      presenceSupervisionStaff{membership: owners.Membership, persons: owners.Persons},
	}
	if owners.Devices != nil {
		deps.Devices = activeDeviceDirectory{devices: owners.Devices}
	}
	return deps
}

// presenceSupervisionStaff resolves the staff members behind supervision rows
// through School Membership and, when the People Directory is given, their
// person facts.
type presenceSupervisionStaff struct {
	membership schoolMembershipCompose.StaffLookup
	persons    peopledirectory.Query
}

func (d presenceSupervisionStaff) SupervisionStaff(ctx context.Context, staffIDs []int64) (map[int64]*presenceCompose.SessionStaff, error) {
	result := make(map[int64]*presenceCompose.SessionStaff)
	if d.membership == nil {
		return result, nil
	}
	members, err := schoolMembershipCompose.StaffByID(ctx, d.membership, staffIDs, studentpresence.SupervisionStaffIncludesRemoved)
	if err != nil {
		return nil, err
	}
	for id, member := range members {
		result[id] = sessionStaff(member)
	}
	if d.persons == nil {
		return result, nil
	}
	return result, attachSupervisionPersons(ctx, d.persons, result)
}

// personGroupSupervisorRepository attaches Staff.Person to active-group
// supervisions.
func attachSupervisionPersons(ctx context.Context, query peopledirectory.Query, staff map[int64]*presenceCompose.SessionStaff) error {
	ids := make([]int64, 0, len(staff))
	for _, member := range staff {
		ids = append(ids, member.PersonID)
	}
	persons, err := peopleCompose.PersonsByID(ctx, query, ids)
	if err != nil {
		return err
	}
	for _, member := range staff {
		if person, found := persons[member.PersonID]; found {
			// The birthday stays with the owner; no supervision reader
			// renders it.
			member.Person = &presenceCompose.SessionStaffPerson{
				ID: person.ID, TenantID: person.TenantID, CreatedAt: person.CreatedAt, UpdatedAt: person.UpdatedAt,
				FirstName: person.FirstName, LastName: person.LastName,
				TagID: person.TagID, AccountID: person.AccountID,
			}
		}
	}
	return nil
}

// sessionStaff is the supervising staff member of an active-group
// supervision. The replaced LEFT JOIN carried no soft-delete filter, so an
// offboarded colleague keeps resolving here.
func sessionStaff(member schoolmembership.Staff) *presenceCompose.SessionStaff {
	return &presenceCompose.SessionStaff{
		ID: member.ID, TenantID: member.TenantID, CreatedAt: member.CreatedAt, UpdatedAt: member.UpdatedAt,
		PersonID: member.PersonID, StaffNotes: member.StaffNotes, EmploymentType: member.EmploymentType,
		WorkTimeModelID: member.WorkTimeModelID, RotationAnchorDate: peopleCompose.ParseRetainedCalendarDate(member.RotationAnchorDate),
		BirthdayDisplayOptOut: member.BirthdayDisplayOptOut,
	}
}
