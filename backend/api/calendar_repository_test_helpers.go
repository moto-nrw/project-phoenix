//go:build testhooks

package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
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

type CalendarTestRepositories struct {
	TimetableTestRepositories
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

func NewCalendarFeedTestRepositories(db transactionruntime.Database) CalendarFeedTestRepositories {
	identity := NewPeopleIdentityAccess(db, nil)
	return CalendarFeedTestRepositories{StaffFeed: identity, ParentFeed: identity,
		Tombstone: schoolCalendarCompose.NewFeedHistory(db.Pool())}
}
