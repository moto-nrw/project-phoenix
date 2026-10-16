package compose

import (
	"context"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// AttendanceMirrorDependencies wire the attendance mirror; the composition
// binds its panic report to Sentry.
type AttendanceMirrorDependencies = application.AttendanceMirrorDependencies

// NewAttendanceMirror composes the Timetable owner's attendance mirror and
// reports its recovered panics to Sentry.
func NewAttendanceMirror(deps AttendanceMirrorDependencies) (timetable.AttendanceMirror, error) {
	deps.ReportPanic = reportMirrorPanic
	return application.NewAttendanceMirror(deps)
}

// reportMirrorPanic sends the panic to Sentry on the hub of the request or
// job that ran the mirror, tagged with the school whose transaction it ran
// in. The event is flushed, since a panic may precede the end of the process.
func reportMirrorPanic(ctx context.Context, recovered any) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	hub.WithScope(func(scope *sentry.Scope) {
		if schoolID := mirrorSchoolID(ctx); schoolID > 0 {
			scope.SetTag("school_id", strconv.FormatInt(schoolID, 10))
		}
		hub.RecoverWithContext(ctx, recovered)
	})
	hub.Flush(2 * time.Second)
}

// mirrorSchoolID is the school whose tenant transaction the mirror ran in,
// or 0 outside one.
func mirrorSchoolID(ctx context.Context) int64 {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0
	}
	return tenantID.Int64()
}
