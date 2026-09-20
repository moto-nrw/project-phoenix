package application

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ArrivalBaselineQueries struct{ source ports.ArrivalBaselineSource }

func NewArrivalBaselineQueries(source ports.ArrivalBaselineSource) *ArrivalBaselineQueries {
	if source == nil {
		panic("care plan: arrival baseline source is required")
	}
	return &ArrivalBaselineQueries{source: source}
}
func (q *ArrivalBaselineQueries) Project(ctx context.Context, ids []int64, from, to calendar.Date) (*careplan.ArrivalBaselineProjection, error) {
	out := &careplan.ArrivalBaselineProjection{
		WeeklyByStudentDate:          make(careplan.ArrivalPlansByStudent, len(ids)),
		DerivedByStudentDate:         make(careplan.ArrivalPlansByStudent, len(ids)),
		ClassExceptionsByStudentDate: make(map[int64]careplan.ClassArrivalExceptionsByDate, len(ids)),
	}
	ids = uniqueBaselineIDs(ids)
	if len(ids) == 0 || to.Before(from) {
		return out, nil
	}
	rows, err := q.source.StoredArrivalRows(ctx, ids)
	if err != nil {
		return nil, err
	}
	classes, err := q.source.StudentClasses(ctx, ids)
	if err != nil {
		return nil, err
	}
	times, err := q.source.ClassArrivalTimes(ctx, classes)
	if err != nil {
		return nil, err
	}
	days, err := q.careDays(ctx, ids, from, to)
	if err != nil {
		return nil, err
	}
	exceptions, err := q.source.ClassArrivalExceptions(ctx, classes, from, to)
	if err != nil {
		return nil, err
	}
	stored := arrivalRowsByStudent(rows)
	out.BookingsAuthoritative = days != nil
	for _, id := range ids {
		weekly, derived := domain.ProjectArrivalWeeks(id, stored[id], times[classes[id]], days, from, to)
		out.WeeklyByStudentDate[id] = weekly
		out.DerivedByStudentDate[id] = derived
		out.ClassExceptionsByStudentDate[id] = exceptions[classes[id]]
	}
	return out, nil
}
func arrivalRowsByStudent(rows []*careplan.ArrivalSchedule) map[int64]careplan.ArrivalWeek {
	out := make(map[int64]careplan.ArrivalWeek)
	for _, row := range rows {
		if row == nil {
			continue
		}
		if out[row.StudentID] == nil {
			out[row.StudentID] = make(careplan.ArrivalWeek)
		}
		out[row.StudentID][row.Weekday] = row
	}
	return out
}
func (q *ArrivalBaselineQueries) careDays(ctx context.Context, ids []int64, from, to calendar.Date) (careplan.CareDayIndex, error) {
	authoritative, err := q.source.BookingsAuthoritative(ctx)
	if err != nil || !authoritative {
		return nil, err
	}
	links, err := q.source.BookingRows(ctx, ids, from, to)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load booking links: %w", err)
	}
	offerings, err := q.source.OfferingRows(ctx, links)
	if err != nil {
		return nil, err
	}
	return domain.ProjectCareDayIndex(links, offerings, from, to), nil
}

type PickupBaselineQueries struct{ source ports.PickupBaselineSource }

func NewPickupBaselineQueries(source ports.PickupBaselineSource) *PickupBaselineQueries {
	if source == nil {
		panic("care plan: pickup baseline source is required")
	}
	return &PickupBaselineQueries{source: source}
}
func (q *PickupBaselineQueries) Project(ctx context.Context, ids []int64, from, to calendar.Date) (*careplan.PickupBaselineProjection, error) {
	out := &careplan.PickupBaselineProjection{WeeklyByStudentDate: make(careplan.PickupPlansByStudent, len(ids)), OfferingByStudentDate: make(careplan.PickupPlansByStudent, len(ids))}
	ids = uniqueBaselineIDs(ids)
	if len(ids) == 0 || to.Before(from) {
		return out, nil
	}
	authoritative, err := q.source.BookingsAuthoritative(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.source.StoredPickupRows(ctx, ids)
	if err != nil {
		return nil, err
	}
	links, err := q.source.BookingRows(ctx, ids, from, to)
	if err != nil {
		return nil, fmt.Errorf("project pickup baselines: load booking links: %w", err)
	}
	offerings, err := q.source.OfferingRows(ctx, links)
	if err != nil {
		return nil, err
	}
	offering, err := domain.ProjectOfferingLinks(links, offerings, from, to)
	if err != nil {
		return nil, err
	}
	out.BookingsAuthoritative = authoritative
	out.CareDays = domain.ProjectCareDayIndex(links, offerings, from, to)
	domain.MergePickupPlans(out, ids, manualPickupRows(rows), offering, from, to)
	return out, nil
}
func manualPickupRows(rows []*careplan.PickupSchedule) map[int64]map[int]*careplan.PickupSchedule {
	out := make(map[int64]map[int]*careplan.PickupSchedule)
	for _, row := range rows {
		if row == nil || row.Source == careplan.ScheduleSourceCareOffering {
			continue
		}
		if out[row.StudentID] == nil {
			out[row.StudentID] = make(map[int]*careplan.PickupSchedule)
		}
		out[row.StudentID][row.Weekday] = row
	}
	return out
}
func (q *PickupBaselineQueries) OfferingPickupForDate(ctx context.Context, id int64, date calendar.Date) (*careplan.PickupSchedule, error) {
	projection, err := q.Project(ctx, []int64{id}, date, date)
	if err != nil {
		return nil, err
	}
	return projection.OfferingForDate(id, date), nil
}
func (q *PickupBaselineQueries) HasBookedOfferingPickupForWeekday(ctx context.Context, id int64, weekday int) (bool, error) {
	if weekday < 1 || weekday > 5 {
		return false, nil
	}
	links, err := q.source.BookingRows(ctx, []int64{id}, calendar.TodayDate(), calendar.NewDate(9999, time.December, 31))
	if err != nil {
		return false, fmt.Errorf("find booked offering pickup: %w", err)
	}
	offerings, err := q.source.OfferingRows(ctx, links)
	if err != nil {
		return false, err
	}
	return domain.HasOfferingPickupForWeekday(id, weekday, links, offerings)
}
func uniqueBaselineIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

var _ careplan.ArrivalBaselineReader = (*ArrivalBaselineQueries)(nil)
var _ careplan.PickupBaselineReader = (*PickupBaselineQueries)(nil)
