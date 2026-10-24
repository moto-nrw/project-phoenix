package contracttest_test

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// fixedPickupBaseline projects one recurring offering time for one student.
type fixedPickupBaseline struct {
	StudentID    int64
	Weekday      int
	HHMM         string
	OfferingName string
}

func (f fixedPickupBaseline) Project(
	_ context.Context,
	_ []int64,
	from, to calendar.Date,
) (*careplan.PickupBaselineProjection, error) {
	parsed, err := time.Parse("15:04", f.HHMM)
	if err != nil {
		return nil, err
	}
	offering := make(careplan.PickupPlansByStudent)
	weekly := make(careplan.PickupPlansByStudent)
	offering[f.StudentID] = make(careplan.PickupPlanByDate)
	weekly[f.StudentID] = make(careplan.PickupPlanByDate)
	for date := from; !date.After(to); date = date.AddDays(1) {
		row := &careplan.PickupSchedule{
			StudentID: f.StudentID, Weekday: f.Weekday, PickupTime: calendar.NormalizeWallClock(parsed),
			Source: careplan.ScheduleSourceCareOffering, CareOfferingName: f.OfferingName,
		}
		offering[f.StudentID][date] = careplan.PickupWeek{f.Weekday: row}
		weekly[f.StudentID][date] = careplan.PickupWeek{f.Weekday: row}
	}
	return &careplan.PickupBaselineProjection{
		WeeklyByStudentDate: weekly, OfferingByStudentDate: offering,
	}, nil
}

func (f fixedPickupBaseline) OfferingPickupForDate(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
) (*careplan.PickupSchedule, error) {
	projection, err := f.Project(ctx, []int64{studentID}, date, date)
	if err != nil {
		return nil, err
	}
	return projection.OfferingForDate(studentID, date), nil
}

func (f fixedPickupBaseline) HasBookedOfferingPickupForWeekday(_ context.Context, studentID int64, weekday int) (bool, error) {
	return studentID == f.StudentID && weekday == f.Weekday, nil
}

// newPickupBaselineService builds the legacy-mode test projection.
func newPickupBaselineService(
	records carePlanPorts.PickupBaselineRecords,
	links careplan.ApprovedBookingReader,
) careplan.PickupBaselineReader {
	baselines, err := compose.NewPickupBaselines(records, links, func(context.Context) (bool, error) { return false, nil })
	if err != nil {
		panic(err)
	}
	return baselines
}
