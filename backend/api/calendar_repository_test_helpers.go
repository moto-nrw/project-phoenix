//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewSchoolCalendarWithAdministration composes the unobserved calendar owner
// over the given administration resolver, which the services test graphs
// answer with the recurrence gate, the care-offering guard and the
// federal-state setting once those services exist. Production roots compose
// the module themselves (api/base.go).
func NewSchoolCalendarWithAdministration(db transactionruntime.Database, administration func() schoolCalendarCompose.AdministrationRuntime) (*schoolcalendar.Module, error) {
	return schoolCalendarCompose.New(schoolCalendarCompose.Dependencies{
		DB:             db.Pool(),
		Observe:        func(schoolCalendarCompose.Observation) {},
		Administration: administration,
	})
}

// NewAppointments composes the unobserved appointment owner for test graphs.
// Production roots compose the module themselves (api/base.go) and hand it
// to the service graph.
func NewAppointments(db transactionruntime.Database) (appointments.Capability, error) {
	return appointmentsCompose.New(appointmentsCompose.Dependencies{
		DB:      db.Pool(),
		Observe: func(appointmentsCompose.Observation) {},
	})
}

type CalendarTestRepositories struct {
	TimetableTestRepositories
	// The other owners' rows beside the Timetable rows (#4080).
	MembershipTestRepositories
	TimetablePresenceRows
	TimetableCarePlanRows
	TimetableRoomRows
	TimetableDeviationAudit
	Profile         identityaccess.AccountProfiles
	StaffGroups     schoolstructure.StaffGroupQuery
	GuardianProfile peoplePorts.GuardianProfileRepository
	StudentGuardian peoplePorts.StudentGuardianRepository
	// ParentPortalRepositories carries the guardian child repository
	// (ParentChild) without the school names.
	carePlanCompose.ParentPortalRepositories
	StaffCalendarFeedToken     identityaccess.StaffCalendarFeeds
	ParentCalendarFeed         identityaccess.ParentCalendarFeeds
	CalendarStaffFeedTombstone schoolcalendar.FeedHistory
	appointments               appointments.Capability
	schoolCalendar             schoolcalendar.Capability
}

func NewCalendarTestRepositories(db transactionruntime.Database, members MembershipTestRepositories) (CalendarTestRepositories, error) {
	identity, staffGroups, err := NewUserContextTestRepositories(db, members)
	if err != nil {
		return CalendarTestRepositories{}, err
	}
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return CalendarTestRepositories{}, err
	}
	parents, err := NewParentRouteTestRepositories(db, members)
	if err != nil {
		return CalendarTestRepositories{}, err
	}
	appointments, err := NewAppointments(db)
	if err != nil {
		return CalendarTestRepositories{}, err
	}
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		return CalendarTestRepositories{}, err
	}
	presence, care, rooms, _, err := newTimetableTestCompanions(db, tt, members)
	if err != nil {
		return CalendarTestRepositories{}, err
	}
	feeds := NewCalendarFeedTestRepositories(db)
	return CalendarTestRepositories{
		TimetableTestRepositories: tt, MembershipTestRepositories: members, TimetablePresenceRows: presence, TimetableCarePlanRows: care, TimetableRoomRows: rooms,
		Profile: identity.Profile, StaffGroups: staffGroups,
		GuardianProfile: members.GuardianProfile, StudentGuardian: members.StudentGuardian,
		ParentPortalRepositories: parents.ParentPortalRepositories, StaffCalendarFeedToken: feeds.StaffFeed, ParentCalendarFeed: feeds.ParentFeed,
		CalendarStaffFeedTombstone: NewCalendarFeedTestTombstones(db), appointments: appointments, schoolCalendar: calendar,
	}, nil
}

// ParentChildLinks lists the children a guardian account reaches, as the
// parent calendar reads them.
func (r CalendarTestRepositories) ParentChildLinks() carePlanCompose.ParentChildLinks {
	return carePlanCompose.NewParentChildLinks(r.ParentChild)
}

func (r CalendarTestRepositories) Appointments() appointments.Capability     { return r.appointments }
func (r CalendarTestRepositories) SchoolCalendar() schoolcalendar.Capability { return r.schoolCalendar }

// CalendarFeedTestRepositories are the Identity & Access feed tokens; the
// staff feed tombstones come from NewCalendarFeedTestTombstones.
type CalendarFeedTestRepositories struct {
	ParentFeed identityaccess.ParentCalendarFeeds
	StaffFeed  identityaccess.StaffCalendarFeeds
}

func NewCalendarFeedTestRepositories(db transactionruntime.Database) CalendarFeedTestRepositories {
	identity := NewPeopleIdentityAccess(db, nil)
	return CalendarFeedTestRepositories{StaffFeed: identity, ParentFeed: identity}
}

// NewCalendarFeedTestTombstones is the School Calendar's staff feed history
// over db.
func NewCalendarFeedTestTombstones(db transactionruntime.Database) schoolcalendar.FeedHistory {
	return schoolCalendarCompose.NewFeedHistory(db.Pool())
}
