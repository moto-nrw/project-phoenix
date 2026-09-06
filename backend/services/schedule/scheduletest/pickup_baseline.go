// Package scheduletest contains shared test doubles for schedule consumers.
package scheduletest

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModel "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/services/schedule"
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
) (*schedule.PickupBaselineProjection, error) {
	parsed, err := time.Parse("15:04", f.HHMM)
	if err != nil {
		return nil, err
	}
	offering := make(schedule.PickupPlansByStudent)
	weekly := make(schedule.PickupPlansByStudent)
	offering[f.StudentID] = make(schedule.PickupPlanByDate)
	weekly[f.StudentID] = make(schedule.PickupPlanByDate)
	for date := from; !date.After(to); date = date.AddDays(1) {
		row := &scheduleModel.StudentPickupSchedule{
			StudentID: f.StudentID, Weekday: f.Weekday, PickupTime: timezone.NormalizeWallClock(parsed),
			Source: scheduleModel.PickupScheduleSourceCareOffering, CareOfferingName: f.OfferingName,
		}
		offering[f.StudentID][date] = schedule.PickupWeek{f.Weekday: row}
		weekly[f.StudentID][date] = schedule.PickupWeek{f.Weekday: row}
	}
	return &schedule.PickupBaselineProjection{
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

// NewPickupBaselineService builds the legacy-mode test projection.
func NewPickupBaselineService(
	weekly scheduleModel.StudentPickupScheduleRepository,
	links schedule.ApprovedBookingReader,
	offerings enrollmentModel.CareOfferingRepository,
) schedule.PickupBaselineReader {
	return schedule.NewPickupBaselineServiceWithSettings(weekly, links, offerings, &configtest.Mock{})
}
