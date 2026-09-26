package students

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
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

// broadcastStudentCompanionsChanged tells the tenant's clients that a child's
// Laufgemeinschaft may have changed, so every mounted "läuft mit" view refetches
// (and an in-progress edit stops before it overwrites the change).
//
// Separate from student_updated on purpose: the links are symmetric, so a save
// on one child changes another child's card, and an editing form has to react by
// discarding or blocking its draft. Reacting that way to every student write —
// a photo, a name, a sick flag — would cost users their work for changes that
// never touched the links. Callers pass tenantID like broadcastStudentUpdated,
// and the fan-out is best-effort: a lost event costs a stale card, never data.
func (rs *Resource) broadcastStudentCompanionsChanged(tenantID, studentID int64) {
	if rs.Broadcaster == nil || tenantID <= 0 {
		return
	}

	source := "manual"
	event := realtime.NewEvent(realtime.EventStudentCompanionsChanged, "", realtime.EventData{
		Source: &source,
	})

	if err := rs.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && rs.Logger != nil {
		rs.Logger.Warn(
			"failed to broadcast student companions change",
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

// scheduleStudentUpdateWakes enqueues any durable absence notification and
// registers the after-commit SSE fan-out for a student update. It always
// broadcasts the tenant-wide student_updated staff
// event; it additionally wakes the child's guardians when the request actually
// touched a status field (sick/excused), because a sick/excused edit writes
// TODAY's status day — the exact signal the parent pickup tile resolves
// today_absent from — while student_updated never reaches the parents stream. A
// plain name/notes edit changes nothing parent-visible, so it wakes no one
// (#1725). Only the ephemeral broadcasts run after the OUTER tx commits, so a
// woken client never reads the pre-commit snapshot; tenantID is captured before
// the hook fires.
func (rs *Resource) scheduleStudentUpdateWakes(ctx context.Context, tenantID, studentID int64, req *UpdateStudentRequest, companionsChanged bool, reportedStatus string, reportDate timezone.Date) error {
	statusChanged := req.Sick != nil || req.Excused != nil
	actorAccountID := int64(jwt.ClaimsFromCtx(ctx).ID)
	if reportedStatus != "" {
		if err := rs.notifyAbsenceReported(ctx, tenantID, []int64{studentID}, reportedStatus, []timezone.Date{reportDate}, false, actorAccountID); err != nil {
			return err
		}
	}
	tenant.RegisterAfterCommit(ctx, func() {
		rs.broadcastStudentUpdated(tenantID, studentID)
		// Only when the write actually changed the links (or a linked child's
		// departure plan) — see applyCompanionUpdate. An open Laufgemeinschaft
		// form reacts to this event by discarding or blocking the user's draft,
		// so firing it for a resubmitted, unchanged plan would cost somebody
		// their unsaved work for a change that never touched them.
		if companionsChanged {
			rs.broadcastStudentCompanionsChanged(tenantID, studentID)
		}
		if statusChanged {
			rs.wakeChildGuardians(tenantID, studentID)
		}
	})
	return nil
}
