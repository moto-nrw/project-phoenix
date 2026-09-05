package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/schedule"
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
	timetableCapability, err := repositories.NewTimetable(db, persons, rooms, schedule.TimetableCareDayLocker(db))
	if err != nil {
		return ownerCapabilities{}, err
	}
	return ownerCapabilities{
		organizations: organizations, persons: persons, groups: groups, rooms: rooms,
		membership: membership, calendar: calendar, timetable: timetableCapability,
	}, nil
}
