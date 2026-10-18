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

// sessionRecordDependencies binds every owner directory of the Student
// Presence session records (#2743): now fixes the supervision "today" (nil
// is the Berlin clock); devices may be nil, device reads then fail loudly.
// The caller adds the database.
func sessionRecordDependencies(now func() time.Time, devices devicefleet.Query, rooms facilitiesModule.Query, activities timetable.Capability, membership schoolmembership.Query, persons peopledirectory.Query) presenceCompose.SessionRecordDependencies {
	deps := presenceCompose.SessionRecordDependencies{
		Now: now, Rooms: activeRoomDirectory{rooms: rooms},
		Activities: NewSessionActivities(timetableCompose.NewTimetableGroupReads(timetableCompose.NewActivityGroupRecords(activities))),
		Staff:      presenceSupervisionStaff{membership: membership, persons: persons},
	}
	if devices != nil {
		deps.Devices = activeDeviceDirectory{devices: devices}
	}
	return deps
}

// presenceSupervisionStaff resolves the staff members behind supervision rows
// through School Membership and, when the People Directory is given, their
// person facts.
type presenceSupervisionStaff struct {
	membership schoolMembershipCompose.StaffLookup
	persons    personsByIDQuery
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
func attachSupervisionPersons(ctx context.Context, query personsByIDQuery, staff map[int64]*presenceCompose.SessionStaff) error {
	ids := make([]int64, 0, len(staff))
	for _, member := range staff {
		ids = append(ids, member.PersonID)
	}
	persons, err := livePersonsByID(ctx, query, ids)
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
