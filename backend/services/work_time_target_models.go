package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

type WorkTimeTargetModelRecords interface {
	FindByID(context.Context, int64) (*config.WorkTimeModel, error)
	FindByIDs(context.Context, []int64) ([]*config.WorkTimeModel, error)
}

type WorkTimeTargetModels struct{ records WorkTimeTargetModelRecords }

func NewWorkTimeTargetModels(records WorkTimeTargetModelRecords) *WorkTimeTargetModels {
	return &WorkTimeTargetModels{records: records}
}

func (r *WorkTimeTargetModels) FindByID(ctx context.Context, id int64) (*timetracking.WorkTimeTargetModel, error) {
	row, err := r.records.FindByID(ctx, id)
	return workTimeTargetModel(row), err
}

func (r *WorkTimeTargetModels) FindByIDs(ctx context.Context, ids []int64) ([]*timetracking.WorkTimeTargetModel, error) {
	rows, err := r.records.FindByIDs(ctx, ids)
	if rows == nil {
		return nil, err
	}
	result := make([]*timetracking.WorkTimeTargetModel, len(rows))
	for i, row := range rows {
		result[i] = workTimeTargetModel(row)
	}
	return result, err
}

func workTimeTargetModel(row *config.WorkTimeModel) *timetracking.WorkTimeTargetModel {
	if row == nil {
		return nil
	}
	return &timetracking.WorkTimeTargetModel{
		ID: row.ID, RotationAnchorDate: timezone.Date(row.RotationAnchorDate),
		DailyTarget: func(anchor, date timezone.Date) (int, bool) {
			return config.DailyTargetFromModel(row, config.CalendarDate(anchor), config.CalendarDate(date))
		},
	}
}
