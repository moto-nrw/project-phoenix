package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type requestDiffAdapter struct{ *weeklyApprovalAdapter }

func (a requestDiffAdapter) DepartureModesForStudent(ctx context.Context, id int64) (map[string][]string, error) {
	student, err := a.s.people.FindStudentRecord(ctx, id)
	if err != nil {
		return nil, requestStudentReadError(err)
	}
	plan := requestDeparturePlan(student)
	result := make(map[string][]string, len(plan.AllowedDepartureModes))
	for day, modes := range plan.AllowedDepartureModes {
		result[day] = nil
		for _, mode := range modes {
			result[day] = append(result[day], string(mode))
		}
	}
	return result, nil
}
func (a requestDiffAdapter) ExceptionPickupTime(ctx context.Context, id int64, date careplan.Date) (*time.Time, error) {
	rows, err := a.s.requestRecords.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0].PickupTime, nil
}
func (a requestDiffAdapter) EffectivePickupTime(ctx context.Context, id int64, date careplan.Date) (*time.Time, error) {
	if a.s.pickup == nil {
		return nil, nil
	}
	row, err := a.s.pickup.GetEffectivePickupTimeForDate(ctx, id, timezone.Date(date))
	if err != nil || row == nil {
		return nil, err
	}
	return row.PickupTime, nil
}
