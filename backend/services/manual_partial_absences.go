package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

type ManualPartialAbsenceDates struct {
	source *repositories.ManualPartialAbsences
}

func NewManualPartialAbsenceDates(source carePlanCompose.PickupExceptionReader) *ManualPartialAbsenceDates {
	return &ManualPartialAbsenceDates{source: repositories.NewManualPartialAbsences(source)}
}

func (r *ManualPartialAbsenceDates) ManualPartialAbsenceDates(ctx context.Context, studentID int64, from, to timezone.Date) ([]timezone.Date, error) {
	rows, err := r.source.Dates(ctx, studentID, careplan.Date(from), careplan.Date(to))
	if err != nil {
		return nil, err
	}
	dates := make([]timezone.Date, len(rows))
	for i, row := range rows {
		dates[i] = timezone.Date(row)
	}
	return dates, nil
}
