package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// NewAttendanceMirror composes the Timetable owner's attendance mirror and
// reports its recovered panics to Sentry.
func NewAttendanceMirror(deps ports.AttendanceMirrorDependencies) (timetable.AttendanceMirror, error) {
	deps.ReportPanic = application.ReportMirrorPanic
	return application.NewAttendanceMirror(deps)
}
