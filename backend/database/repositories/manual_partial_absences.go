package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

type ManualPartialAbsences struct {
	source *carePlanCompose.ManualPartialAbsences
}

func NewManualPartialAbsences(source carePlanCompose.PickupExceptionReader) *ManualPartialAbsences {
	return &ManualPartialAbsences{source: carePlanCompose.NewManualPartialAbsences(source)}
}

func (r *ManualPartialAbsences) Dates(ctx context.Context, studentID int64, from, to careplan.Date) ([]careplan.Date, error) {
	rows, err := r.source.Dates(ctx, studentID, from, to)
	return rows, legacyScheduleError("find by student id and date range", err)
}
