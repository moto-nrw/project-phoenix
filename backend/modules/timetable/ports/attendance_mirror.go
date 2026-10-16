package ports

import (
	"context"
	"log/slog"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// AttendanceMirrorDependencies wires the mirror; both repositories are
// required, the logger falls back to slog.Default.
type AttendanceMirrorDependencies struct {
	Instances    scheduleModels.ActivityInstanceRepository
	Participants scheduleModels.InstanceStudentRepository
	Logger       *slog.Logger
	// ReportPanic reports a recovered panic to error tracking; the
	// composition binds Sentry.
	ReportPanic func(ctx context.Context, recovered any)
}
