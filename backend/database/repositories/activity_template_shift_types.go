package repositories

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	activitiesRepo "github.com/moto-nrw/project-phoenix/modules/timetable/compose/repositoryadapter"
)

type activityTemplateShiftTypeDirectory struct {
	shifts scheduleModels.ShiftTypeRepository
}

func (d activityTemplateShiftTypeDirectory) ListShiftTypes(ctx context.Context) ([]activitiesRepo.TemplateShiftType, error) {
	rows, err := d.shifts.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]activitiesRepo.TemplateShiftType, 0, len(rows))
	for _, row := range rows {
		result = append(result, activitiesRepo.TemplateShiftType{ID: row.ID, Name: row.Name, Color: row.Color})
	}
	return result, nil
}

func (f *Factory) bindActivityTemplateShiftTypes() {
	repository, ok := f.ActivityGroup.(*activitiesRepo.GroupRepository)
	if !ok {
		panic("repository factory: raw activity group repository is unavailable")
	}
	repository.BindTemplateShiftTypes(activityTemplateShiftTypeDirectory{shifts: f.ShiftType})
}
