package services

import (
	"context"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// ClassArrivalPlans binds Care Plan's class timetable port to its native owner.
func ClassArrivalPlans(records timetable.ClassArrivals) careplanCompose.ArrivalClassPlans {
	if records == nil {
		return nil
	}
	return classArrivalPlans{records}
}

type classArrivalPlans struct{ records timetable.ClassArrivals }

func (s classArrivalPlans) FindByClasses(ctx context.Context, classes []string) ([]*careplanCompose.ArrivalClassPlan, error) {
	return (&arrivalBaselineDirectory{classes: s.records}).ClassPlans(ctx, classes)
}

func (s classArrivalPlans) LockClass(ctx context.Context, class string) error {
	return s.records.LockClassArrivalPlan(ctx, class)
}

func (s classArrivalPlans) Upsert(ctx context.Context, row *careplanCompose.ArrivalClassPlan) error {
	stored, err := s.records.UpsertClassArrivalPlan(ctx, timetable.ClassArrivalPlanInput{SchoolClass: row.SchoolClass, ArrivalTimes: row.ArrivalTimes, UpdatedBy: row.UpdatedBy})
	if err == nil {
		*row = careplanCompose.ArrivalClassPlan{ID: stored.ID, TenantID: stored.TenantID, CreatedAt: stored.CreatedAt, UpdatedAt: stored.UpdatedAt,
			SchoolClass: stored.SchoolClass, ArrivalTimes: stored.ArrivalTimes, UpdatedBy: stored.UpdatedBy}
	}
	return err
}
