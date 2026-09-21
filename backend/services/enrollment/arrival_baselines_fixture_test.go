package enrollment_test

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// newArrivalBaselinesFixture binds native People and Timetable projections to
// Care Plan without reaching the root composition packages.
func newArrivalBaselinesFixture(records careplanCompose.ArrivalBaselineRecords, students careplantest.ArrivalStudentRecords, classes timetableModule.ClassArrivalQuery, links careplan.ApprovedBookingReader, bookingMode func(context.Context) (bool, error)) (careplan.ArrivalBaselineReader, error) {
	return careplanCompose.NewArrivalBaselines(careplanCompose.ArrivalBaselineDependencies{
		Records: records, Bookings: links, BookingMode: bookingMode,
		StudentClasses: careplantest.ArrivalStudentClasses(students),
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
		ClassExceptions: func(ctx context.Context, names []string, from, to timezone.Date) (map[string]careplan.ClassArrivalExceptionsByDate, error) {
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
				out[key][timezone.Date(row.Date)] = &careplan.ArrivalBaselineException{SchoolClass: row.SchoolClass, ArrivalTime: row.ArrivalTime, Label: (&careplan.ClassArrivalException{SchoolClass: row.SchoolClass, Reason: row.Reason}).Label()}
			}
			return out, nil
		},
	})
}
