package timetablehttp

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

func (rs *Resource) resolveTemplateGradeLevelMax(ctx context.Context) (int, error) {
	if rs.SettingsService == nil {
		return 0, errors.New("settings service is not configured")
	}
	value, err := rs.SettingsService.ResolveInt(ctx, settings.KeyEnrollmentGradeLevelMax)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", settings.KeyEnrollmentGradeLevelMax, err)
	}
	if value < timetableModule.MinSchoolGradeLevel || value > timetableModule.MaxSchoolGradeLevel {
		return 0, fmt.Errorf(
			"resolve %s: value %d is outside %d..%d",
			settings.KeyEnrollmentGradeLevelMax,
			value,
			timetableModule.MinSchoolGradeLevel,
			timetableModule.MaxSchoolGradeLevel,
		)
	}
	return value, nil
}
