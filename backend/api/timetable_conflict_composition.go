package api

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/services"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewTimetableConflictDetection composes the Timetable owner's conflict
// detection and staffing capability over the retained readers. It binds the
// two collaborators the owner may not name itself: Security Runtime's content
// fingerprint for the persisted conflict fingerprints and Care Plan's
// arrival baseline for the exception conflicts.
func NewTimetableConflictDetection(readers services.TimetableConflictReaders) (timetable.ConflictDetectionCapability, error) {
	return timetableCompose.NewConflictDetection(timetableConflictDependencies(readers))
}

func timetableConflictDependencies(r services.TimetableConflictReaders) timetableCompose.ConflictDetectionDependencies {
	deps := timetableCompose.ConflictDetectionDependencies{
		Instances:         r.Instances,
		InstanceStaff:     r.InstanceStaff,
		InstanceStudents:  r.InstanceStudents,
		Exceptions:        r.Exceptions,
		Schedules:         r.Schedules,
		Shifts:            r.Shifts,
		Staff:             r.Staff,
		CalendarPeriods:   r.CalendarPeriods,
		ArrivalExceptions: r.ArrivalExceptions,
		Presence:          r.Presence,
		Sessions:          r.Sessions,
		ContentHash:       securityruntime.Fingerprint,
		Logger:            r.Logger,
	}
	if r.ArrivalBaselines != nil {
		deps.ArrivalBaselines = ArrivalBaselineProjector{Reader: r.ArrivalBaselines}
	}
	return deps
}

// ArrivalBaselineProjector serves the Timetable conflict detection's arrival
// port from Care Plan's baseline projection.
type ArrivalBaselineProjector struct {
	Reader careplan.ArrivalBaselineReader
}

func (p ArrivalBaselineProjector) ProjectArrivals(ctx context.Context, studentIDs []int64, from, to timezone.Date) (timetableCompose.ArrivalBaselines, error) {
	projection, err := p.Reader.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	return arrivalBaselineProjection{projection: projection}, nil
}

type arrivalBaselineProjection struct {
	projection *careplan.ArrivalBaselineProjection
}

func (p arrivalBaselineProjection) ExpectedArrival(studentID int64, date timezone.Date) (time.Time, bool) {
	row := p.projection.ForDate(studentID, date)
	if row == nil {
		return time.Time{}, false
	}
	return row.ExpectedArrival, true
}
