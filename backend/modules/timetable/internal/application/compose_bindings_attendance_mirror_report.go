package application

import (
	context "context"
	strconv "strconv"
	time "time"

	sentry "github.com/getsentry/sentry-go"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// reportMirrorPanic sends the panic to Sentry on the hub of the request or
// job that ran the mirror, tagged with the school whose transaction it ran
// in. The event is flushed, since a panic may precede the end of the process.
func ReportMirrorPanic(ctx context.Context, recovered any) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	hub.WithScope(func(scope *sentry.Scope) {
		if schoolID := MirrorSchoolID(ctx); schoolID > 0 {
			scope.SetTag("school_id", strconv.FormatInt(schoolID, 10))
		}
		hub.RecoverWithContext(ctx, recovered)
	})
	hub.Flush(2 * time.Second)
}

// mirrorSchoolID is the school whose tenant transaction the mirror ran in,
// or 0 outside one.
func MirrorSchoolID(ctx context.Context) int64 {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0
	}
	return tenantID.Int64()
}
