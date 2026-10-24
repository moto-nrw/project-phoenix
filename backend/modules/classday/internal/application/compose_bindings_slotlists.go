package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type CareDayBinding struct{ Resolver CareDayResolver }

func (b CareDayBinding) ResolveForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]ports.CareDay, error) {
	verdicts, err := b.Resolver.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]ports.CareDay, len(verdicts))
	for studentID, verdict := range verdicts {
		out[studentID] = ports.CareDay(verdict)
	}
	return out, nil
}

type EffectiveTimeBinding struct {
	Pickups  PickupTimeReader
	Arrivals ArrivalTimeReader
}

func (b EffectiveTimeBinding) PickupTimes(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*time.Time, error) {
	effective, err := b.Pickups.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*time.Time, len(effective))
	for studentID, entry := range effective {
		if entry != nil {
			out[studentID] = entry.PickupTime
		}
	}
	return out, nil
}
func (b EffectiveTimeBinding) ArrivalTimes(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*time.Time, error) {
	effective, err := b.Arrivals.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*time.Time, len(effective))
	for studentID, entry := range effective {
		if entry != nil {
			out[studentID] = entry.ArrivalTime
		}
	}
	return out, nil
}

type PickupBaselineBinding struct {
	Baselines PickupBaselineReader
}

func (b PickupBaselineBinding) RegularPickupTimes(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]string, error) {
	projection, err := b.Baselines.Project(ctx, studentIDs, date, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(studentIDs))
	for _, studentID := range studentIDs {
		if row := projection.ForDate(studentID, date); row != nil {
			out[studentID] = row.PickupTime.Format("15:04")
		}
	}
	return out, nil
}
