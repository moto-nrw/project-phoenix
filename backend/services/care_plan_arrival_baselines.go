package services

import (
	"context"
	"errors"
	"strings"

	timezone "github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type arrivalBaselineStudents interface {
	ListStudentRecordsByID(context.Context, []int64) ([]peopledirectory.StudentRecord, error)
}

// NewArrivalBaselines binds the caller-supplied native People and Timetable
// projections to Care Plan. It constructs no repository or service graph.
func NewArrivalBaselines(
	records compose.ArrivalBaselineRecords,
	students arrivalBaselineStudents,
	classes timetable.ClassArrivalQuery,
	links careplan.ApprovedBookingReader,
	bookingMode func(context.Context) (bool, error),
) (careplan.ArrivalBaselineReader, error) {
	if students == nil || classes == nil {
		return nil, errors.New("arrival baselines: students and class projections are required")
	}
	directory := arrivalBaselineDirectory{students, classes}
	return compose.NewArrivalBaselines(compose.ArrivalBaselineDependencies{
		Records: records, Bookings: links, BookingMode: bookingMode,
		StudentClasses: directory.StudentClasses, ClassPlans: directory.ClassPlans, ClassExceptions: directory.ClassArrivalExceptions,
	})
}

type arrivalBaselineDirectory struct {
	students arrivalBaselineStudents
	classes  timetable.ClassArrivalQuery
}

func (s *arrivalBaselineDirectory) ClassPlans(ctx context.Context, classes []string) ([]*compose.ArrivalClassPlan, error) {
	rows, err := s.classes.ListClassArrivalPlans(ctx, classes)
	if err != nil {
		return nil, err
	}
	result := make([]*compose.ArrivalClassPlan, len(rows))
	for i, row := range rows {
		result[i] = &compose.ArrivalClassPlan{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			SchoolClass: row.SchoolClass, ArrivalTimes: row.ArrivalTimes, UpdatedBy: row.UpdatedBy}
	}
	return result, nil
}
func (s *arrivalBaselineDirectory) StudentClasses(ctx context.Context, ids []int64) (map[int64]string, error) {
	rows, err := s.students.ListStudentRecordsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(rows))
	for _, row := range rows {
		out[row.ID] = row.SchoolClass
	}
	return out, nil
}
func (s *arrivalBaselineDirectory) ClassArrivalExceptions(ctx context.Context, classes []string, from, to timezone.Date) (map[string]careplan.ClassArrivalExceptionsByDate, error) {
	if len(classes) == 0 {
		return nil, nil
	}
	rows, err := s.classes.ListClassArrivalExceptions(ctx, classes, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	out := make(map[string]careplan.ClassArrivalExceptionsByDate)
	for _, row := range rows {
		key := strings.ToLower(strings.TrimSpace(row.SchoolClass))
		if out[key] == nil {
			out[key] = make(careplan.ClassArrivalExceptionsByDate)
		}
		out[key][timezone.Date(row.Date)] = &careplan.ArrivalBaselineException{SchoolClass: row.SchoolClass, ArrivalTime: row.ArrivalTime, Label: (&careplan.ClassArrivalException{SchoolClass: row.SchoolClass, Reason: row.Reason}).Label()}
	}
	return out, nil
}
