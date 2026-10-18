package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
)

// NewAbsenceTypeAdministration binds Workforce to /api/absence-types.
func NewAbsenceTypeAdministration(catalog timetracking.AbsenceTypeCatalog, logger *slog.Logger) workforce.AbsenceTypeAdministration {
	return timetracking.NewAbsenceTypeAdministration(catalog, logger)
}

// NewTimeTrackingShifts binds the time-tracking planned-shift port to the
// public Workforce shift query (#3418).
func NewTimeTrackingShifts(shifts workforce.ShiftQuery) *timetracking.TimeTrackingShifts {
	return timetracking.NewTimeTrackingShifts(shifts)
}
