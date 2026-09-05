package repositories

import (
	"context"

	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
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
	phases := enrollmentRepo.NewPhaseRepository(db)
	return SettingsTestRepositories{
		Values: configRepo.NewSettingValueRepository(runtime), Audit: configRepo.NewSettingAuditRepository(runtime),
		ClassRestriction: phases.ExistsActiveWithEligibleClasses,
		GradeRestriction: phases.ExistsActiveWithEligibleGradeLevels,
		GradeCap:         phases.MaxActiveEligibleGradeLevel,
	}
}
