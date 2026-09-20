package compose

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ArrivalBaselineRecords interface {
	ListArrivalSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error)
	ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
}

// ArrivalBaselineDependencies binds tenant-safe projections from People and
// Timetable. BookingMode is required even when a caller selects stored plans.
type ArrivalBaselineDependencies struct {
	Records         ArrivalBaselineRecords
	Bookings        careplan.ApprovedBookingReader
	BookingMode     func(context.Context) (bool, error)
	StudentClasses  func(context.Context, []int64) (map[int64]string, error)
	ClassPlans      func(context.Context, []string) ([]*ArrivalClassPlan, error)
	ClassExceptions func(context.Context, []string, calendar.Date, calendar.Date) (map[string]careplan.ClassArrivalExceptionsByDate, error)
}

func NewArrivalBaselines(deps ArrivalBaselineDependencies) (careplan.ArrivalBaselineReader, error) {
	if deps.Records == nil || deps.Bookings == nil || deps.BookingMode == nil || deps.StudentClasses == nil || deps.ClassPlans == nil || deps.ClassExceptions == nil {
		return nil, errors.New("arrival baselines: records, bookings, booking mode, students, class plans, and exceptions are required")
	}
	return application.NewArrivalBaselineQueries(arrivalBaselineSource{deps}), nil
}

type arrivalBaselineSource struct{ deps ArrivalBaselineDependencies }

func (s arrivalBaselineSource) StoredArrivalRows(ctx context.Context, ids []int64) ([]*careplan.ArrivalSchedule, error) {
	if len(ids) == 0 {
		return []*careplan.ArrivalSchedule{}, nil
	}
	rows, err := s.deps.Records.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load stored schedules: %w", err)
	}
	out := make([]*careplan.ArrivalSchedule, len(rows))
	for i := range rows {
		out[i] = &rows[i]
	}
	return out, nil
}

func (s arrivalBaselineSource) StudentClasses(ctx context.Context, ids []int64) (map[int64]string, error) {
	rows, err := s.deps.StudentClasses(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load students: %w", err)
	}
	out := make(map[int64]string, len(rows))
	for id, class := range rows {
		if key := strings.ToLower(strings.TrimSpace(class)); key != "" {
			out[id] = key
		}
	}
	return out, nil
}

func (s arrivalBaselineSource) ClassArrivalTimes(ctx context.Context, classes map[int64]string) (map[string]*domain.ClassArrivalBaseline, error) {
	if len(classes) == 0 {
		return nil, nil
	}
	rows, err := s.deps.ClassPlans(ctx, arrivalBaselineClasses(classes))
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load class arrival times: %w", err)
	}
	out := make(map[string]*domain.ClassArrivalBaseline, len(rows))
	for _, row := range rows {
		if row == nil || len(row.ArrivalTimes) == 0 {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(row.SchoolClass))] = domain.ClassArrivalBaselineFromTimes(row.SchoolClass, row.ArrivalTimes)
	}
	return out, nil
}

func (s arrivalBaselineSource) ClassArrivalExceptions(ctx context.Context, classes map[int64]string, from, to calendar.Date) (map[string]careplan.ClassArrivalExceptionsByDate, error) {
	if len(classes) == 0 {
		return nil, nil
	}
	rows, err := s.deps.ClassExceptions(ctx, arrivalBaselineClasses(classes), from, to)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load class arrival exceptions: %w", err)
	}
	return rows, nil
}

func arrivalBaselineClasses(classes map[int64]string) []string {
	out := make([]string, 0, len(classes))
	for _, class := range classes {
		out = append(out, class)
	}
	return out
}

func (s arrivalBaselineSource) BookingsAuthoritative(ctx context.Context) (bool, error) {
	value, err := s.deps.BookingMode(ctx)
	if err != nil {
		return false, fmt.Errorf("project arrival baselines: resolve booking mode: %w", err)
	}
	return value, nil
}

func (s arrivalBaselineSource) BookingRows(ctx context.Context, ids []int64, from, to calendar.Date) ([]*careplan.ApprovedBooking, error) {
	return s.deps.Bookings.ListApprovedByStudentIDsInRange(ctx, ids, from, to)
}

func (s arrivalBaselineSource) OfferingRows(ctx context.Context, links []*careplan.ApprovedBooking) (map[int64]*careplan.CareOffering, error) {
	rows, err := baselineOfferingRows(ctx, s.deps.Records, links)
	if err != nil {
		return nil, fmt.Errorf("project arrival baselines: load care offerings: %w", err)
	}
	return rows, nil
}
