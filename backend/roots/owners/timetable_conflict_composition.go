package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewTimetableConflictDetection composes the Timetable owner's conflict
// detection and staffing capability over deps, the retained readers the
// caller fills (#3550). It binds the two collaborators the owner may not
// name itself: Security Runtime's content fingerprint for the persisted
// conflict fingerprints and Care Plan's arrival baseline for the exception
// conflicts. arrivalBaselines is optional (the exception-conflict read fails
// without it).
func NewTimetableConflictDetection(deps timetablePorts.ConflictDetectionDependencies, arrivalBaselines careplan.ArrivalBaselineReader) (timetable.ConflictDetectionCapability, error) {
	deps.ContentHash = securityruntime.Fingerprint
	if arrivalBaselines != nil {
		deps.ArrivalBaselines = ArrivalBaselineProjector{Reader: arrivalBaselines}
	}
	return timetableCompose.NewConflictDetection(deps)
}

// ArrivalBaselineProjector serves the Timetable conflict detection's arrival
// port from Care Plan's baseline projection.
type ArrivalBaselineProjector struct {
	Reader careplan.ArrivalBaselineReader
}

func (p ArrivalBaselineProjector) ProjectArrivals(ctx context.Context, studentIDs []int64, from, to timezone.Date) (timetablePorts.ArrivalBaselines, error) {
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
