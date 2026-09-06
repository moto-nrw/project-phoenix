package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/uptrace/bun"
)

type manualPlanningReader struct{ db *bun.DB }

func (r manualPlanningReader) ListManualPlanningOccurrences(ctx context.Context, studentID int64, from, to string) ([]enrollment.ManualPlanningOccurrence, error) {
	rows, err := repositories.NewManualPlanningQuery(r.db).ListManualPlanningOccurrences(ctx, studentID, from, to)
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.ManualPlanningOccurrence, 0, len(rows))
	for _, row := range rows {
		result = append(result, enrollment.ManualPlanningOccurrence{ActivityGroupID: row.ActivityGroupID, ActivityGroupName: row.ActivityGroupName, InstanceID: row.InstanceID, Date: row.Date})
	}
	return result, nil
}
