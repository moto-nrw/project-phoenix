package repositories

import (
	"context"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

type SettingsTestRepositories struct {
	Values           configModels.SettingValueRepository
	Audit            configModels.SettingAuditRepository
	ClassRestriction func(context.Context) (bool, error)
	GradeRestriction func(context.Context) (bool, error)
	GradeCap         func(context.Context) (int, error)
}

func NewSettingsTestRepositories(db *bun.DB, runtime configRepo.Runtime) SettingsTestRepositories {
	phases := enrollmentCompose.New()
	return SettingsTestRepositories{
		Values: configRepo.NewSettingValueRepository(runtime), Audit: configRepo.NewSettingAuditRepository(runtime),
		ClassRestriction: phases.HasActiveClassRestrictedPhase,
		GradeRestriction: phases.HasActiveGradeRestrictedPhase,
		GradeCap:         phases.MaxActivePhaseGrade,
	}
}
