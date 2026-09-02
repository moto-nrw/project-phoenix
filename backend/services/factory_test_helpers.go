package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// NewFactoryForTests creates the partial graph used by legacy package tests.
// Production composition must provide every migrated module explicitly.
func NewFactoryForTests(repos *repositories.Factory, db *bun.DB, logger *slog.Logger, clocks ...func() time.Time) (*Factory, error) {
	organizations, persons, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	return newFactory(repos, db, logger, currentFactoryConfig(), organizations, persons, nil, nil, nil, nil, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, true, clocks...)
}

func NewFactoryForTestsWithConfig(repos *repositories.Factory, db *bun.DB, logger *slog.Logger, cfg FactoryConfig, clocks ...func() time.Time) (*Factory, error) {
	organizations, persons, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	return newFactory(repos, db, logger, cfg, organizations, persons, nil, nil, nil, nil, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, true, clocks...)
}

// NewFactoryForTestsWithFeedback keeps API integration tests on the real
// owner query without requiring the production-only full module graph.
func NewFactoryForTestsWithFeedback(
	repos *repositories.Factory,
	db *bun.DB,
	logger *slog.Logger,
	feedback users.FeedbackEntryCounter,
	bindFeedbackSettings FeedbackSettingsBinder,
	clocks ...func() time.Time,
) (*Factory, error) {
	organizations, persons, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return nil, err
	}
	return newFactory(repos, db, logger, currentFactoryConfig(), organizations, persons, nil, nil, feedback, bindFeedbackSettings, func(string, time.Duration, int, error) {}, func(string, string, string, time.Duration, error) {}, func(string, string, string, time.Duration, int, error) {}, true, clocks...)
}

func newOwnerCapabilitiesForTests(db *bun.DB) (SchoolCapability, peopledirectory.Capability, error) {
	organizations, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return nil, nil, err
	}
	persons, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return nil, nil, err
	}
	return organizations, persons, nil
}
