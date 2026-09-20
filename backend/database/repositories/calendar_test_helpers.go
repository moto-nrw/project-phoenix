package repositories

import (
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	authModels "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	authRepo "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authpostgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/uptrace/bun"
)

type CalendarTestRepositories struct {
	TimetableTestRepositories
	Account                    authModels.AccountRepository
	AccountTenant              authModels.AccountTenantRepository
	Profile                    identityaccess.AccountProfiles
	GroupSubstitution          educationModels.GroupSubstitutionRepository
	GuardianProfile            userModels.GuardianProfileRepository
	StudentGuardian            userModels.StudentGuardianRepository
	ParentChild                parentModels.ChildRepository
	StaffCalendarFeedToken     identityaccess.StaffCalendarFeeds
	CalendarStaffFeedTombstone schoolcalendar.FeedHistory
	appointments               appointments.Capability
	schoolCalendar             schoolcalendar.Capability
}

func NewCalendarTestRepositories(db *bun.DB) (CalendarTestRepositories, error) {
	identity, err := NewUserContextTestRepositories(db)
	if err != nil {
		return CalendarTestRepositories{}, err
	}
	parents, err := NewParentRouteTestRepositories(db)
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
		Account:                   identity.Account, AccountTenant: authRepo.NewAccountTenantRepository(db),
		Profile: identity.Profile, GroupSubstitution: identity.Substitutions,
		GuardianProfile: parents.GuardianProfile, StudentGuardian: parents.StudentGuardian,
		ParentChild: parents.ParentChild, StaffCalendarFeedToken: feeds.StaffFeed,
		CalendarStaffFeedTombstone: feeds.Tombstone, appointments: appointments, schoolCalendar: calendar,
	}, nil
}

func (r CalendarTestRepositories) Appointments() appointments.Capability     { return r.appointments }
func (r CalendarTestRepositories) SchoolCalendar() schoolcalendar.Capability { return r.schoolCalendar }

type CalendarFeedTestRepositories struct {
	StaffFeed identityaccess.StaffCalendarFeeds
	Tombstone schoolcalendar.FeedHistory
}

func NewCalendarFeedTestRepositories(db *bun.DB) CalendarFeedTestRepositories {
	return CalendarFeedTestRepositories{StaffFeed: newIdentityAccess(db, nil),
		Tombstone: schoolCalendarCompose.NewFeedHistory(db)}
}
