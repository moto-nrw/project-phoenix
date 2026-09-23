package timetable

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

func (rs *Resource) resolveTemplateGradeLevelMax(ctx context.Context) (int, error) {
	if rs.SettingsService == nil {
		return 0, errors.New("settings service is not configured")
	}
	value, err := rs.SettingsService.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentGradeLevelMax, err)
	}
	if value < timetableModule.MinSchoolGradeLevel || value > timetableModule.MaxSchoolGradeLevel {
		return 0, fmt.Errorf(
			"resolve %s: value %d is outside %d..%d",
			configModel.KeyEnrollmentGradeLevelMax,
			value,
			timetableModule.MinSchoolGradeLevel,
			timetableModule.MaxSchoolGradeLevel,
		)
	}
	return value, nil
}

func renderTemplateTargetGradeLimit(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, timetableModule.ErrTemplateTargetGradeExceedsLimit) {
		return false
	}
	common.RenderError(w, r, common.ErrorInvalidRequest(err))
	return true
}
