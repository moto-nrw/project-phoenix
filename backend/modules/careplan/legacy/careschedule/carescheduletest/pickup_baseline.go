// Package carescheduletest contains shared test doubles for care schedule consumers.
package carescheduletest

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModel "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/careschedule"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
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
) (*careschedule.PickupBaselineProjection, error) {
	parsed, err := time.Parse("15:04", f.HHMM)
	if err != nil {
		return nil, err
	}
	offering := make(careschedule.PickupPlansByStudent)
	weekly := make(careschedule.PickupPlansByStudent)
	offering[f.StudentID] = make(careschedule.PickupPlanByDate)
	weekly[f.StudentID] = make(careschedule.PickupPlanByDate)
	for date := from; !date.After(to); date = date.AddDays(1) {
		row := &scheduleModel.StudentPickupSchedule{
			StudentID: f.StudentID, Weekday: f.Weekday, PickupTime: timezone.NormalizeWallClock(parsed),
			Source: scheduleModel.PickupScheduleSourceCareOffering, CareOfferingName: f.OfferingName,
		}
		offering[f.StudentID][date] = careschedule.PickupWeek{f.Weekday: row}
		weekly[f.StudentID][date] = careschedule.PickupWeek{f.Weekday: row}
	}
	return &careschedule.PickupBaselineProjection{
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
	links careschedule.ApprovedBookingReader,
	offerings enrollmentModel.CareOfferingRepository,
) careschedule.PickupBaselineReader {
	return careschedule.NewPickupBaselineServiceWithSettings(weekly, links, offerings, &configtest.Mock{})
}
