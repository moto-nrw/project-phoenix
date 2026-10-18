//go:build testhooks

package api

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
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

// CalendarTestRepositories are the Identity & Access rows of the calendar
// portal suites: the account profiles and the feed tokens. The People
// Directory, Timetable, Facilities, guardian portal, School Calendar and
// Appointments rows come from MustNewMembershipTestRepositories,
// NewCalendarTimetableRows, MustNewTimetableRoomRows, NewCalendarParentRows,
// NewCalendarSchoolRows and NewCalendarAppointmentRows (#4080).
type CalendarTestRepositories struct {
	Profile                identityaccess.AccountProfiles
	StaffCalendarFeedToken identityaccess.StaffCalendarFeeds
	ParentCalendarFeed     identityaccess.ParentCalendarFeeds
}

func NewCalendarTestRepositories(db transactionruntime.Database, members MembershipTestRepositories) (CalendarTestRepositories, error) {
	identity, _, err := NewUserContextTestRepositories(db, members)
	if err != nil {
		return CalendarTestRepositories{}, err
	}
	feeds := NewCalendarFeedTestRepositories(db)
	return CalendarTestRepositories{Profile: identity.Profile, StaffCalendarFeedToken: feeds.StaffFeed, ParentCalendarFeed: feeds.ParentFeed}, nil
}

// NewCalendarTimetableRows are the Timetable rows the calendar suites
// arrange over members.
func NewCalendarTimetableRows(db transactionruntime.Database, members MembershipTestRepositories) TimetableTestRepositories {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		panic(fmt.Sprintf("calendar test rows: compose timetable: %v", err))
	}
	return tt
}

// CalendarParentRows are the guardian portal's child rows (Care Plan) the
// parent calendar reads, without the school names.
type CalendarParentRows struct {
	carePlanCompose.ParentPortalRepositories
}

// ParentChildLinks lists the children a guardian account reaches, as the
// parent calendar reads them.
func (r CalendarParentRows) ParentChildLinks() carePlanCompose.ParentChildLinks {
	return carePlanCompose.NewParentChildLinks(r.ParentChild)
}

// NewCalendarParentRows composes the child rows over members.
func NewCalendarParentRows(db transactionruntime.Database, members MembershipTestRepositories) CalendarParentRows {
	parents, err := NewParentRouteTestRepositories(db, members)
	if err != nil {
		panic(fmt.Sprintf("calendar test rows: compose parent rows: %v", err))
	}
	return CalendarParentRows{ParentPortalRepositories: parents.ParentPortalRepositories}
}

// CalendarSchoolRows are the School Calendar owner and its staff feed
// history.
type CalendarSchoolRows struct {
	CalendarStaffFeedTombstone schoolcalendar.FeedHistory
	schoolCalendar             schoolcalendar.Capability
}

func (r CalendarSchoolRows) SchoolCalendar() schoolcalendar.Capability { return r.schoolCalendar }

// NewCalendarSchoolRows composes the calendar owner over db.
func NewCalendarSchoolRows(db transactionruntime.Database) CalendarSchoolRows {
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("calendar test rows: compose school calendar: %v", err))
	}
	return CalendarSchoolRows{CalendarStaffFeedTombstone: NewCalendarFeedTestTombstones(db), schoolCalendar: calendar}
}

// CalendarAppointmentRows serve the Appointments owner.
type CalendarAppointmentRows struct {
	appointments appointments.Capability
}

func (r CalendarAppointmentRows) Appointments() appointments.Capability { return r.appointments }

// NewCalendarAppointmentRows composes the appointment owner over db.
func NewCalendarAppointmentRows(db transactionruntime.Database) CalendarAppointmentRows {
	owner, err := NewAppointments(db)
	if err != nil {
		panic(fmt.Sprintf("calendar test rows: compose appointments: %v", err))
	}
	return CalendarAppointmentRows{appointments: owner}
}

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
