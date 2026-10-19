//go:build testhooks

package api

import (
	"context"
	"errors"
	"fmt"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// federalStateResolver is the slice of the settings service the tenant
// holiday reads need.
type federalStateResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

// NewSchoolCalendarWithAdministration composes the unobserved calendar owner
// with the federal-state setting (#1418 3a), the recurrence gate and the
// Care Plan catalog's care-offering guard, whose refusal is mapped onto the
// calendar's own sentinel; a nil catalog guards nothing. Production roots
// compose the module themselves.
func NewSchoolCalendarWithAdministration(db transactionruntime.Database, settings federalStateResolver, lockRecurrence func(context.Context) error, careOfferings careplan.CareOfferingGuards) (*schoolcalendar.Module, error) {
	observe := func(schoolCalendarCompose.Observation) {}
	records, err := schoolCalendarCompose.NewRecords(schoolCalendarCompose.RecordsDependencies{
		DB: db.Pool(), Observe: observe,
		FederalState: func(ctx context.Context) (string, error) {
			region, err := settings.ResolveString(ctx, configModel.KeyFederalState)
			if err != nil {
				return "", fmt.Errorf("failed to resolve federal state setting: %w", err)
			}
			return region, nil
		},
	})
	if err != nil {
		return nil, err
	}
	guard := func(context.Context, int64, *schoolcalendar.CalendarPeriodFields) error { return nil }
	if careOfferings != nil {
		guard = func(ctx context.Context, periodID int64, replacement *schoolcalendar.CalendarPeriodFields) error {
			err := careOfferings.ValidateCalendarPeriodChange(ctx, periodID, calendarPeriodReplacement(replacement))
			if errors.Is(err, careplan.ErrCalendarPeriodCareOfferingConflict) {
				return fmt.Errorf("%w: %w", schoolcalendar.ErrCalendarPeriodRequiredByCareOffering, err)
			}
			return err
		}
	}
	return schoolCalendarCompose.NewAdministration(records, schoolCalendarCompose.AdministrationDependencies{
		DB: db.Pool(), Observe: observe, RecurrenceGate: lockRecurrence, CareOfferingGuard: guard,
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

func NewCalendarTestRepositories(db transactionruntime.Database, members owners.MembershipTestRepositories) (CalendarTestRepositories, error) {
	identity, _, err := NewUserContextTestRepositories(db, members)
	if err != nil {
		return CalendarTestRepositories{}, err
	}
	feeds := NewCalendarFeedTestRepositories(db)
	return CalendarTestRepositories{Profile: identity.Profile, StaffCalendarFeedToken: feeds.StaffFeed, ParentCalendarFeed: feeds.ParentFeed}, nil
}

// NewCalendarTimetableRows are the Timetable rows the calendar suites
// arrange over members.
func NewCalendarTimetableRows(db transactionruntime.Database, members owners.MembershipTestRepositories) owners.TimetableTestRepositories {
	tt, err := owners.NewTimetableTestRepositoriesWithMembers(db, members)
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
func NewCalendarParentRows(db transactionruntime.Database, members owners.MembershipTestRepositories) CalendarParentRows {
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
	calendar, err := owners.NewSchoolCalendar(db)
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
	identity := owners.NewPeopleIdentityAccess(db, nil)
	return CalendarFeedTestRepositories{StaffFeed: identity, ParentFeed: identity}
}

// NewCalendarFeedTestTombstones is the School Calendar's staff feed history
// over db.
func NewCalendarFeedTestTombstones(db transactionruntime.Database) schoolcalendar.FeedHistory {
	return schoolCalendarCompose.NewFeedHistory(db.Pool())
}
