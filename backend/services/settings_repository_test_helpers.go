package services

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/uptrace/bun"
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

// DisplayTestRepositories bundles the readers the info-point dashboard
// aggregates with the settings stores the dashboard resolves its feature
// switch through.
type DisplayTestRepositories struct {
	DisplayReadRepositories
	SettingsTestRepositories
}

// NewDisplayTestRepositories composes the dashboard's read dependencies.
func NewDisplayTestRepositories(db *bun.DB, runtime settingsCompose.PersistenceRuntime) (DisplayTestRepositories, error) {
	readers, err := NewDisplayTestRepositoriesWithMembers(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return DisplayTestRepositories{}, err
	}
	readers.TimetableTestRepositories = withTimetableTestAudit(db, readers.TimetableTestRepositories)
	return DisplayTestRepositories{DisplayReadRepositories: readers, SettingsTestRepositories: NewSettingsTestRepositories(runtime)}, nil
}
