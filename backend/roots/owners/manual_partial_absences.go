package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ManualPartialAbsenceDates serves Student Presence's manual partial absence
// reader from Care Plan's pickup exceptions.
type ManualPartialAbsenceDates struct {
	source *carePlanCompose.ManualPartialAbsences
}

func NewManualPartialAbsenceDates(source carePlanCompose.PickupExceptionReader) *ManualPartialAbsenceDates {
	return &ManualPartialAbsenceDates{source: carePlanCompose.NewManualPartialAbsences(source)}
}

func (r *ManualPartialAbsenceDates) ManualPartialAbsenceDates(ctx context.Context, studentID int64, from, to timezone.Date) ([]timezone.Date, error) {
	rows, err := r.source.Dates(ctx, studentID, careplan.Date(from), careplan.Date(to))
	if err != nil {
		return nil, manualPartialAbsenceError(err)
	}
	dates := make([]timezone.Date, len(rows))
	for i, row := range rows {
		dates[i] = timezone.Date(row)
	}
	return dates, nil
}

// manualPartialAbsenceError keeps the retained repository error shape: a
// missing schedule row classifies as not found, everything else is wrapped
// with its operation.
func manualPartialAbsenceError(err error) error {
	const operation = "find by student id and date range"
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return transactionruntime.RecordErrors{}.MissingRecordError(operation)
	}
	return transactionruntime.RecordErrors{}.WrapRecordError(operation, err)
}
