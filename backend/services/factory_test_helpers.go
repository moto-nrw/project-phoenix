package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// ownerCapabilities bundles the migrated owner modules the legacy test graph
// still composes explicitly.
type ownerCapabilities struct {
	organizations SchoolCapability
	persons       peopledirectory.Capability
	groups        schoolstructure.Query
	rooms         facilitiesModule.Capability
	membership    schoolmembership.Capability
	calendar      schoolcalendar.Capability
	timetable     timetable.Capability
}

// NewFactoryForTests creates the partial graph used by legacy package tests.
// Production composition must provide every migrated module explicitly.
func NewFactoryForTests(repos *repositories.Factory, db *bun.DB, logger *slog.Logger, clocks ...func() time.Time) (*Factory, error) {
	owners, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	return newFactory(repos, db, logger, currentFactoryConfig(), owners.organizations, owners.persons, owners.groups, owners.rooms, owners.membership, owners.calendar, owners.timetable, nil, nil, nil, nil, nil, nil, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, true, clocks...)
}

func NewFactoryForTestsWithConfig(repos *repositories.Factory, db *bun.DB, logger *slog.Logger, cfg FactoryConfig, clocks ...func() time.Time) (*Factory, error) {
	owners, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	return newFactory(repos, db, logger, cfg, owners.organizations, owners.persons, owners.groups, owners.rooms, owners.membership, owners.calendar, owners.timetable, nil, nil, nil, nil, nil, nil, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, true, clocks...)
}

// NewFactoryForTestsWithFeedback keeps API integration tests on the real
// feedback module while the remaining legacy dependencies stay optional.
func NewFactoryForTestsWithFeedback(
	repos *repositories.Factory,
	db *bun.DB,
	logger *slog.Logger,
	feedback users.FeedbackEntryCounter,
	bindFeedbackSettings FeedbackSettingsBinder,
	clocks ...func() time.Time,
) (*Factory, error) {
	owners, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	return newFactory(repos, db, logger, currentFactoryConfig(), owners.organizations, owners.persons, owners.groups, owners.rooms, owners.membership, owners.calendar, owners.timetable, nil, nil, nil, nil, feedback, bindFeedbackSettings, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, true, clocks...)
}

func newOwnerCapabilitiesForTests(db *bun.DB) (ownerCapabilities, error) {
	organizations, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	persons, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	groups, err := repositories.NewSchoolStructure(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	calendar, err := repositories.NewSchoolCalendar(db)
	if err != nil {
		return ownerCapabilities{}, err
	}
	timetableCapability, err := timetableCompose.New(timetableCompose.Dependencies{
		DB: db, Students: timetableStudents(persons), Observe: func(timetableCompose.Observation) {},
	})
	if err != nil {
		return ownerCapabilities{}, err
	}
	return ownerCapabilities{
		organizations: organizations, persons: persons, groups: groups, rooms: rooms,
		membership: membership, calendar: calendar, timetable: timetableCapability,
	}, nil
}

func timetableStudents(students peopledirectory.StudentQuery) timetableCompose.StudentDirectory {
	return timetableCompose.StudentDirectoryFunc(func(ctx context.Context) ([]timetableCompose.TargetStudent, error) {
		values, err := students.ListEnrolledStudents(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]timetableCompose.TargetStudent, 0, len(values))
		for _, value := range values {
			result = append(result, timetableCompose.TargetStudent{
				ID: value.ID, SchoolClass: value.SchoolClass, EducationGroupID: value.GroupID,
				EnrolledUntil: value.EnrolledUntil,
			})
		}
		return result, nil
	})
}
