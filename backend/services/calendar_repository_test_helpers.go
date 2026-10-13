package services

import (
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/uptrace/bun"
)

// NewSchoolCalendarWithAdministration composes the unobserved calendar owner
// over the given administration resolver, which the services test graphs
// answer with the recurrence gate, the care-offering guard and the
// federal-state setting once those services exist. Production roots compose
// the module themselves (api/base.go).
func NewSchoolCalendarWithAdministration(db *bun.DB, administration func() schoolCalendarCompose.AdministrationRuntime) (*schoolcalendar.Module, error) {
	return schoolCalendarCompose.New(schoolCalendarCompose.Dependencies{
		DB:             db,
		Observe:        func(schoolCalendarCompose.Observation) {},
		Administration: administration,
	})
}

// NewAppointments composes the unobserved appointment owner for test graphs.
// Production roots compose the module themselves (api/base.go) and hand it
// to the service graph.
func NewAppointments(db *bun.DB) (appointments.Capability, error) {
	return appointmentsCompose.New(appointmentsCompose.Dependencies{
		DB:      db,
		Observe: func(appointmentsCompose.Observation) {},
	})
}

type CalendarTestRepositories struct {
	TimetableTestRepositories
	Profile         identityaccess.AccountProfiles
	StaffGroups     schoolstructure.StaffGroupQuery
	GuardianProfile userModels.GuardianProfileRepository
	StudentGuardian userModels.StudentGuardianRepository
	// ParentPortalRepositories carries the guardian child repository
	// (ParentChild) without the school names.
	carePlanCompose.ParentPortalRepositories
	StaffCalendarFeedToken     identityaccess.StaffCalendarFeeds
	ParentCalendarFeed         identityaccess.ParentCalendarFeeds
	CalendarStaffFeedTombstone schoolcalendar.FeedHistory
	appointments               appointments.Capability
	schoolCalendar             schoolcalendar.Capability
}

func NewCalendarTestRepositories(db *bun.DB, members MembershipTestRepositories) (CalendarTestRepositories, error) {
	identity, err := NewUserContextTestRepositories(db, members)
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
	feeds := NewCalendarFeedTestRepositories(db)
	return CalendarTestRepositories{
		TimetableTestRepositories: identity.Timetable,
		Profile:                   identity.Profile, StaffGroups: identity.StaffGroups,
		GuardianProfile: parents.GuardianProfile, StudentGuardian: parents.StudentGuardian,
		ParentPortalRepositories: parents.ParentPortalRepositories, StaffCalendarFeedToken: feeds.StaffFeed, ParentCalendarFeed: feeds.ParentFeed,
		CalendarStaffFeedTombstone: feeds.Tombstone, appointments: appointments, schoolCalendar: calendar,
	}, nil
}

// ParentChildLinks lists the children a guardian account reaches, as the
// parent calendar reads them.
func (r CalendarTestRepositories) ParentChildLinks() carePlanCompose.ParentChildLinks {
	return carePlanCompose.NewParentChildLinks(r.ParentChild)
}

func (r CalendarTestRepositories) Appointments() appointments.Capability     { return r.appointments }
func (r CalendarTestRepositories) SchoolCalendar() schoolcalendar.Capability { return r.schoolCalendar }

type CalendarFeedTestRepositories struct {
	ParentFeed identityaccess.ParentCalendarFeeds
	StaffFeed  identityaccess.StaffCalendarFeeds
	Tombstone  schoolcalendar.FeedHistory
}

func NewCalendarFeedTestRepositories(db *bun.DB) CalendarFeedTestRepositories {
	identity := newPeopleIdentityAccess(db, nil)
	return CalendarFeedTestRepositories{StaffFeed: identity, ParentFeed: identity,
		Tombstone: schoolCalendarCompose.NewFeedHistory(db)}
}
