//go:build testhooks

package api

import (
	"context"
	"log/slog"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// SettingsTestRepositories are the settings stores the Settings composition
// builds (ADR 0049) plus the Enrollment guards the settings service checks.
type SettingsTestRepositories struct {
	Values           configModels.SettingValueRepository
	Audit            configModels.SettingAuditRepository
	ClassRestriction func(context.Context) (bool, error)
	GradeRestriction func(context.Context) (bool, error)
	GradeCap         func(context.Context) (int, error)
}

// NewSettingsTestRepositories composes the settings stores over the caller's
// settings runtime.
func NewSettingsTestRepositories(runtime settingsCompose.PersistenceRuntime) SettingsTestRepositories {
	phases := enrollmentCompose.New()
	return SettingsTestRepositories{
		Values: settingsCompose.NewSettingValueRepository(runtime), Audit: settingsCompose.NewSettingAuditRepository(runtime),
		ClassRestriction: phases.HasActiveClassRestrictedPhase,
		GradeRestriction: phases.HasActiveGradeRestrictedPhase,
		GradeCap:         phases.MaxActivePhaseGrade,
	}
}

// NewSettingsTestService composes the tenant settings service over the
// caller's settings runtime without a school store, for suites that switch a
// tenant setting the way an admin does.
func NewSettingsTestService(runtime interface {
	settingsCompose.PersistenceRuntime
	settingsCompose.SettingsRuntime
}) settingsCompose.SettingsService {
	repos := NewSettingsTestRepositories(runtime)
	return settingsCompose.NewSettingsService(repos.Values, repos.Audit, nil, runtime, slog.Default())
}

// NewDisplayTestRepositories composes the readers the info-point dashboard
// aggregates and returns them with the settings stores the dashboard
// resolves its feature switch through.
func NewDisplayTestRepositories(db transactionruntime.Database, runtime settingsCompose.PersistenceRuntime) (DisplayReadRepositories, SettingsTestRepositories, error) {
	readers, err := NewDisplayTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return DisplayReadRepositories{}, SettingsTestRepositories{}, err
	}
	readers.TimetableTestRepositories = withTimetableTestAudit(db, readers.TimetableTestRepositories)
	return readers, NewSettingsTestRepositories(runtime), nil
}
