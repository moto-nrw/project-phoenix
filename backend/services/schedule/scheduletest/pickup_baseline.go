// Package scheduletest contains shared test doubles for schedule consumers.
package scheduletest

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// FixedPickupBaseline projects one recurring offering time for one student.
type FixedPickupBaseline struct {
	StudentID    int64
	Weekday      int
	HHMM         string
	OfferingName string
}

func (f FixedPickupBaseline) Project(
	_ context.Context,
	_ []int64,
	from, to timezone.Date,
) (*scheduleService.PickupBaselineProjection, error) {
	parsed, err := time.Parse("15:04", f.HHMM)
	if err != nil {
		return nil, err
	}
	offering := make(scheduleService.PickupPlansByStudent)
	weekly := make(scheduleService.PickupPlansByStudent)
	offering[f.StudentID] = make(scheduleService.PickupPlanByDate)
	weekly[f.StudentID] = make(scheduleService.PickupPlanByDate)
	for date := from; !date.After(to); date = date.AddDays(1) {
		row := &scheduleModel.StudentPickupSchedule{
			StudentID: f.StudentID, Weekday: f.Weekday, PickupTime: timezone.WallClock(parsed),
			Source: scheduleModel.PickupScheduleSourceCareOffering, CareOfferingName: f.OfferingName,
		}
		offering[f.StudentID][date] = scheduleService.PickupWeek{f.Weekday: row}
		weekly[f.StudentID][date] = scheduleService.PickupWeek{f.Weekday: row}
	}
	return &scheduleService.PickupBaselineProjection{
		WeeklyByStudentDate: weekly, OfferingByStudentDate: offering,
	}, nil
}

func (f FixedPickupBaseline) OfferingPickupForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*scheduleModel.StudentPickupSchedule, error) {
	projection, err := f.Project(ctx, []int64{studentID}, date, date)
	if err != nil {
		return nil, err
	}
	return projection.OfferingForDate(studentID, date), nil
}

func (f FixedPickupBaseline) HasBookedOfferingPickupForWeekday(_ context.Context, studentID int64, weekday int) (bool, error) {
	return studentID == f.StudentID && weekday == f.Weekday, nil
}

func (f FixedPickupBaseline) BookedOfferingPickupsByWeekday(_ context.Context, studentID int64) (map[int][]*scheduleModel.StudentPickupSchedule, error) {
	if studentID != f.StudentID {
		return map[int][]*scheduleModel.StudentPickupSchedule{}, nil
	}
	parsed, err := time.Parse("15:04", f.HHMM)
	if err != nil {
		return nil, err
	}
	return map[int][]*scheduleModel.StudentPickupSchedule{f.Weekday: {{
		StudentID: studentID, Weekday: f.Weekday, PickupTime: timezone.WallClock(parsed),
	}}}, nil
}
