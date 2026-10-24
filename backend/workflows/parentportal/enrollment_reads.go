package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// CarePeriod and OfferingBooking are native workflow read snapshots. Root
// callbacks bind the Enrollment facade without exporting composition aliases.
type CarePeriod care.CarePeriod
type OfferingBooking care.OfferingBooking

func NewCarePeriodReads(read func(context.Context, int64) ([]*CarePeriod, error)) care.CarePeriodReads {
	return carePeriodReads{read}
}

type carePeriodReads struct {
	read func(context.Context, int64) ([]*CarePeriod, error)
}

func (r carePeriodReads) CarePeriods(ctx context.Context, id int64) ([]*care.CarePeriod, error) {
	rows, err := r.read(ctx, id)
	if rows == nil {
		return nil, err
	}
	result := make([]*care.CarePeriod, len(rows))
	for i, row := range rows {
		result[i] = (*care.CarePeriod)(row)
	}
	return result, err
}
func NewOfferingHistoryReads(read func(context.Context, int64) ([]*OfferingBooking, error)) care.OfferingHistoryReads {
	return offeringHistoryReads{read}
}

type offeringHistoryReads struct {
	read func(context.Context, int64) ([]*OfferingBooking, error)
}

func (r offeringHistoryReads) OfferingHistory(ctx context.Context, id int64) ([]*care.OfferingBooking, error) {
	rows, err := r.read(ctx, id)
	if rows == nil {
		return nil, err
	}
	result := make([]*care.OfferingBooking, len(rows))
	for i, row := range rows {
		result[i] = (*care.OfferingBooking)(row)
	}
	return result, err
}
