package timetable

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type arrivalFixtureStudents interface {
	ListStudentRecordsByID(context.Context, []int64) ([]peopledirectory.StudentRecord, error)
}

// newArrivalBaselinesFixture binds native People and Timetable projections to
// Care Plan without reaching the root composition packages.
func newArrivalBaselinesFixture(records careplanCompose.ArrivalBaselineRecords, students arrivalFixtureStudents, classes timetableModule.ClassArrivalQuery, links careplan.ApprovedBookingReader, bookingMode func(context.Context) (bool, error)) (careplan.ArrivalBaselineReader, error) {
	return careplanCompose.NewArrivalBaselines(careplanCompose.ArrivalBaselineDependencies{
		Records: records, Bookings: links, BookingMode: bookingMode,
		StudentClasses: func(ctx context.Context, ids []int64) (map[int64]string, error) {
			rows, err := students.ListStudentRecordsByID(ctx, ids)
			if err != nil {
				return nil, err
			}
			out := make(map[int64]string, len(rows))
			for _, row := range rows {
				out[row.ID] = row.SchoolClass
			}
			return out, nil
		},
		ClassPlans: func(ctx context.Context, names []string) ([]*careplanCompose.ArrivalClassPlan, error) {
			rows, err := classes.ListClassArrivalPlans(ctx, names)
			if err != nil {
				return nil, err
			}
			out := make([]*careplanCompose.ArrivalClassPlan, len(rows))
			for i, row := range rows {
				out[i] = &careplanCompose.ArrivalClassPlan{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
					SchoolClass: row.SchoolClass, ArrivalTimes: row.ArrivalTimes, UpdatedBy: row.UpdatedBy}
			}
			return out, nil
		},
		ClassExceptions: func(ctx context.Context, names []string, from, to calendar.Date) (map[string]careplan.ClassArrivalExceptionsByDate, error) {
			if len(names) == 0 {
				return nil, nil
			}
			rows, err := classes.ListClassArrivalExceptions(ctx, names, from.String(), to.String())
			if err != nil {
				return nil, err
			}
			out := make(map[string]careplan.ClassArrivalExceptionsByDate)
			for _, row := range rows {
				key := strings.ToLower(strings.TrimSpace(row.SchoolClass))
				if out[key] == nil {
					out[key] = make(careplan.ClassArrivalExceptionsByDate)
				}
				out[key][calendar.Date(row.Date)] = &careplan.ArrivalBaselineException{SchoolClass: row.SchoolClass, ArrivalTime: row.ArrivalTime, Label: (&careplan.ClassArrivalException{SchoolClass: row.SchoolClass, Reason: row.Reason}).Label()}
			}
			return out, nil
		},
	})
}
