//go:build testhooks

package api

import (
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewSettingsTestService composes the tenant settings service over the
// caller's settings runtime without a school store, for suites that switch a
// tenant setting the way an admin does.
func NewSettingsTestService(runtime interface {
	settingsCompose.PersistenceRuntime
	settingsCompose.SettingsRuntime
}) settingsCompose.SettingsService {
	repos := owners.NewSettingsTestRepositories(runtime)
	return settingsCompose.NewSettingsService(repos.Values, repos.Audit, nil, runtime, slog.Default())
}

// NewDisplayTestRepositories composes the readers the info-point dashboard
// aggregates and returns them with the settings stores the dashboard
// resolves its feature switch through.
func NewDisplayTestRepositories(db transactionruntime.Database, runtime settingsCompose.PersistenceRuntime) (DisplayReadRepositories, owners.SettingsTestRepositories, error) {
	readers, err := NewDisplayTestRepositoriesWithMembers(db, owners.MustNewMembershipTestRepositories(db))
	if err != nil {
		return DisplayReadRepositories{}, owners.SettingsTestRepositories{}, err
	}
	readers.TimetableTestRepositories = withTimetableTestAudit(db, readers.TimetableTestRepositories)
	return readers, owners.NewSettingsTestRepositories(runtime), nil
}
