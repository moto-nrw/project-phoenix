package timetable

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

func (rs *Resource) resolveTemplateGradeLevelMax(ctx context.Context) (int, error) {
	if rs.SettingsService == nil {
		return 0, errors.New("settings service is not configured")
	}
	value, err := rs.SettingsService.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentGradeLevelMax, err)
	}
	if value < schoolclass.MinGradeLevel || value > schoolclass.MaxGradeLevel {
		return 0, fmt.Errorf(
			"resolve %s: value %d is outside %d..%d",
			configModel.KeyEnrollmentGradeLevelMax,
			value,
			schoolclass.MinGradeLevel,
			schoolclass.MaxGradeLevel,
		)
	}
	return value, nil
}

func renderTemplateTargetGradeLimit(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, scheduleSvc.ErrTemplateTargetGradeExceedsLimit) {
		return false
	}
	common.RenderError(w, r, common.ErrorInvalidRequest(err))
	return true
}
