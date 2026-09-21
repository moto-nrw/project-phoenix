package compose

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s classArrivalExceptions) LockClassArrivalPlan(ctx context.Context, class string) error {
	if strings.TrimSpace(class) == "" {
		return timetable.ErrInvalidClassArrivalPlan
	}
	started := time.Now()
	stats, err := s.store.LockClassArrivalPlan(ctx, strings.ToLower(strings.TrimSpace(class)))
	s.observe(Observation{Operation: "lock_class_arrival_plan", Duration: time.Since(started), Stats: stats, Err: err})
	return err
}

func (s classArrivalExceptions) UpsertClassArrivalPlan(ctx context.Context, input timetable.ClassArrivalPlanInput) (timetable.ClassArrivalPlan, error) {
	if !input.Valid() {
		return timetable.ClassArrivalPlan{}, timetable.ErrInvalidClassArrivalPlan
	}
	started := time.Now()
	row, stats, err := s.store.SaveClassArrivalPlan(ctx, domain.ClassArrivalPlan{SchoolClass: input.SchoolClass, ArrivalTimes: input.ArrivalTimes, UpdatedBy: input.UpdatedBy})
	s.observe(Observation{Operation: "upsert_class_arrival_plan", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return timetable.ClassArrivalPlan{}, err
	}
	return nativeClassArrivalPlan(row), nil
}

func nativeClassArrivalPlan(row domain.ClassArrivalPlan) timetable.ClassArrivalPlan {
	return timetable.ClassArrivalPlan{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		SchoolClass: row.SchoolClass, ArrivalTimes: row.ArrivalTimes, UpdatedBy: row.UpdatedBy}
}
