//go:build testhooks

package owners

import (
	"context"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
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
