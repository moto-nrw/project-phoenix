package presence

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// registerCheckoutBroadcast queues the SSE fan-out of an attendance checkout
// that closed a row or healed an orphaned visit. It covers every entry point
// into performCheckOut: the staff web checkout
// (POST /active/visits/student/{id}/checkout), the school check-in/out endpoint
// used by the binary-mode "Kinder an- und abmelden" flow, the kiosk toggle, and
// the kiosk daily checkout. None of them emitted anything between 976b32f and
// #2113 — ending the visit moved from service.EndVisit (which broadcasts) to
// the repository call above (which does not), so other staff tabs kept showing
// the child as present indefinitely: the room, search and detail views
// revalidate on neither focus nor interval.
//
// Two invariants:
//
//  1. The student display data is read HERE, inside the request transaction.
//     Only the emission is deferred to tenant.RegisterAfterCommit — by the time
//     hooks run, the tx carried in ctx is closed, so a repository read from
//     inside the hook would fail, and reading outside the tenant tx would be
//     blocked by RLS. Deferring matters because TenantTxMiddleware commits at
//     the end of the request: a client woken before the commit refetches the
//     pre-checkout state, and nothing corrects it afterwards.
//  2. Callers pass endedVisit == nil when no room visit was closed, which
//     selects the roomless event shape. Broadcaster identity rules follow the
//     existing helpers: the child id rides only the group-scoped topics, never
//     the tenant-wide invalidation events (#2085).
func (s *service) registerCheckoutBroadcast(
	ctx context.Context,
	studentID int64,
	endedVisit *studentpresence.Visit,
	snapshot *AttendanceSnapshot,
	checkoutType string,
) {
	// Siehe registerCheckinBroadcast: die Eltern-Weckung haengt an der
	// Anwesenheit, nicht am Personal-Broadcaster.
	s.wakeGuardiansAfterCommit(ctx, studentID)

	if s.Broadcaster == nil {
		return
	}

	educationGroupID := s.getEducationGroupForSSE(ctx, studentID)

	tenant.RegisterAfterCommit(ctx, func() {
		if endedVisit != nil {
			// Ordinary visit checkouts historically had no source. Only carry the
			// daily checkout's stable wire value onto the visit-shaped heal path.
			source := ""
			if checkoutType == checkoutTypeDaily {
				source = dailyCheckoutSource
			}
			s.emitVisitCheckout(ctx, endedVisit, snapshot, educationGroupID, source)
			return
		}
		s.emitRoomlessCheckout(ctx, studentID, educationGroupID, checkoutSourceLabel(checkoutType))
	})
}

// registerCheckinBroadcast queues the SSE fan-out of an attendance check-in
// that actually opened a row. Mirror of registerCheckoutBroadcast, same two
// invariants — the display data is read HERE, inside the request transaction,
// and only the emission is deferred past the commit.
//
// Covers the check-in paths that write attendance without a room visit: the
// school check-in/out endpoint used by the "Kinder an- und abmelden" flow, the
// binary-mode kiosk scan, and the door kiosk's attendance toggle in either mode.
// All three were silent, so a colleague's open room, search or detail view kept
// showing the child as absent — none of them revalidates on focus or interval.
//
// Deliberately not gated on presence mode: the detailed-mode ROOM check-in
// takes the CreateVisit path, which writes its own attendance row and emits
// broadcastVisitCreated. The two call sets are disjoint, so no request can emit
// both.
func (s *service) registerCheckinBroadcast(ctx context.Context, studentID int64, checkinType string) {
	// Die Sorgeberechtigten werden unabhaengig vom Broadcaster geweckt: ihr
	// Tagesstatus haengt an der Anwesenheit, nicht an den Personal-Topics.
	s.wakeGuardiansAfterCommit(ctx, studentID)

	if s.Broadcaster == nil {
		return
	}

	educationGroupID := s.getEducationGroupForSSE(ctx, studentID)

	tenant.RegisterAfterCommit(ctx, func() {
		s.emitRoomlessCheckin(ctx, studentID, educationGroupID, checkinType)
	})
}

// emitRoomlessCheckout publishes a checkout that has no room context. Used when
// no visit ended — a binary-mode tenant (which keeps no visit rows at all) or a
// kiosk daily checkout, where the visit was already closed when the child left
// the room.
func (s *service) emitRoomlessCheckout(
	ctx context.Context,
	studentID int64,
	educationGroupID *int64,
	source string,
) {
	s.emitRoomlessAttendanceChange(ctx, false, studentID, educationGroupID, source)
}

// emitRoomlessCheckin publishes a check-in that has no room context: attendance
// opened without the child entering a room. That is every binary-mode check-in
// (the tenant keeps no visit rows) and the door kiosk's attendance toggle in
// either mode. The detailed-mode room check-in does NOT come through here — it
// runs through CreateVisit, which writes its own attendance row and emits
// broadcastVisitCreated, so the two can never fire for the same request.
func (s *service) emitRoomlessCheckin(
	ctx context.Context,
	studentID int64,
	educationGroupID *int64,
	source string,
) {
	s.emitRoomlessAttendanceChange(ctx, true, studentID, educationGroupID, source)
}

// emitRoomlessAttendanceChange publishes an attendance change with no room
// context: the student's educational (OGS) group topic plus the tenant-wide
// dashboard refresh. There is no active group to scope to, hence no active-group
// topic and no active_supervision_changed — no room roster changed.
//
// Takes the display data pre-resolved for the same reason as
// emitVisitCheckout — see its doc comment.
func (s *service) emitRoomlessAttendanceChange(
	ctx context.Context,
	checkIn bool,
	studentID int64,
	educationGroupID *int64,
	source string,
) {
	if s.Broadcaster == nil {
		return
	}

	eduGroupIDs := eduGroupIDsOf(educationGroupID)

	// Broadcast to educational (OGS) group topic so the "Meine Gruppe" page
	// updates. The source is always carried, even when empty.
	realtimeevents.PublishRoomlessAttendanceChange(ctx, s.Broadcaster, s.getLogger(), checkIn, realtimeevents.VisitChange{
		StudentID:        fmt.Sprintf("%d", studentID),
		EducationGroupID: educationGroupID,
		Source:           source,
	})

	// Notify every client of the tenant so dashboard counts and the search
	// page refresh — the educational group broadcast only reaches staff in
	// that group, but the search page is used by all staff. Scoped to the
	// student's educational group when known (#2057).
	s.broadcastDashboardCountsChanged(ctx, eduGroupIDs)
}
