package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

// ManualPartialAbsences projects the dates owned by staff-entered partial excusals.
type ManualPartialAbsences struct {
	Source carePlanBindingPorts.PickupExceptionReader
}

func (r *ManualPartialAbsences) Dates(ctx context.Context, studentID int64, from, to careplan.Date) ([]careplan.Date, error) {
	rows, err := r.Source.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{
		StudentIDs: []int64{studentID}, From: from, To: to,
	})
	if err != nil {
		return nil, err
	}
	dates := make([]careplan.Date, 0, len(rows))
	for _, row := range rows {
		if row.ExcusedFrom != nil && !row.ExcusedAuto {
			dates = append(dates, row.ExceptionDate)
		}
	}
	return dates, nil
}
