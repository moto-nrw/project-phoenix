package studentpresencehttp

import (
	realtime "github.com/moto-nrw/project-phoenix/realtime"
)

// broadcastStudentUpdated emits an SSE student_updated event to the
// AFFECTED tenant's connected clients only. use-global-sse.ts treats
// that event as "invalidate every student / room / dashboard cache in
// this tab", so a global fan-out (BroadcastToAll) would force tabs in
// schools B to N to refetch unrelated data whenever school A edits one
// student or one avatar. Routing via BroadcastToTenant, the helper
// already added in this branch for tenant_settings_changed, keeps the
// invalidation scoped to the school that actually changed.
//
// Callers MUST pass tenantID from request context (tenant.FromContext)
// or from a captured value inside an after-commit hook. Passing zero
// no-ops the broadcast rather than fanning out, since
// BroadcastToTenant rejects zero tenant IDs by definition (no Client
// has TenantID == 0).
func (rs *Resource) broadcastStudentUpdated(tenantID, studentID int64) {
	if rs.Broadcaster == nil {
		return
	}
	if tenantID <= 0 {
		// Defensive: a missing tenant context means we don't know which
		// school's clients should invalidate. Logging the case so a
		// future caller that forgets to thread tenantID through gets a
		// breadcrumb instead of silent loss.
		if rs.Logger != nil {
			rs.Logger.Warn(
				"skipping student_updated broadcast, no tenant context",
				"student_id", studentID,
			)
		}
		return
	}

	source := "manual"
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{
		Source: &source,
	})

	if err := rs.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && rs.Logger != nil {
		rs.Logger.Warn(
			"failed to broadcast student update",
			"tenant_id", tenantID,
			"student_id", studentID,
			"error", err.Error(),
		)
	}
}

// wakeChildGuardians fans a message-INDEPENDENT parent_child_updated SSE event
// out to every guardian of the child, so an open parents-app tab refetches the
// child's care state live after a STAFF-side write (status day, pickup/arrival
// override, or weekly-plan change). Without it the parents portal — which only
// receives guardian-targeted events on its own SSE stream, never the tenant-wide
// student_updated / arrival_schedule_changed staff broadcasts — keeps showing a
// stale pickup time or presence until the parent refocuses or reloads (#1725).
//
// Schedule this from an after-commit hook (or otherwise only after the write has
// committed) so a woken client never reads the pre-commit snapshot. tenantID must
// come from tenant.FromContext (captured before the hook runs); a nil emitter or a
// non-positive tenant/student id is a safe no-op — the fan-out is best-effort,
// mirroring the existing broadcastStudentUpdated / excused-request pattern.
func (rs *Resource) wakeChildGuardians(tenantID, studentID int64) {
	if rs.ParentEventEmitter == nil {
		return
	}
	if tenantID <= 0 {
		if rs.Logger != nil {
			rs.Logger.Warn(
				"skipping guardian child-update wake, no tenant context",
				"student_id", studentID,
			)
		}
		return
	}
	rs.ParentEventEmitter.BroadcastChildUpdateToGuardians(tenantID, studentID)
}
