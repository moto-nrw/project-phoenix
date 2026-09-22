package presence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// broadcastVisitCheckout broadcasts SSE event for visit checkout.
// snapshot (WP-B10) may be nil — when present, it enriches the event
// with attendance_status/substatus/note so the frontend can display
// the current attendance state alongside the checkout line.
func (s *service) broadcastVisitCheckout(ctx context.Context, endedVisit *studentpresence.Visit, snapshot *AttendanceSnapshot) {
	if s.Broadcaster == nil || endedVisit == nil {
		return
	}

	educationGroupID := s.getEducationGroupForSSE(ctx, endedVisit.StudentID)
	broadcastCtx := tenant.ContextWithoutTransaction(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		s.emitVisitCheckout(broadcastCtx, endedVisit, snapshot, educationGroupID, "")
	})
}

// emitVisitCheckout is broadcastVisitCheckout with the student routing data
// already resolved. Split out so the attendance checkout paths can read the
// student inside their request transaction and defer only the emission to a
// tenant.RegisterAfterCommit hook (#2113): by the time hooks run, the tx stored
// in ctx is closed, so a repository call from inside the hook would fail — and
// running it outside the tenant tx would be blocked by RLS.
func (s *service) emitVisitCheckout(
	ctx context.Context,
	endedVisit *studentpresence.Visit,
	snapshot *AttendanceSnapshot,
	educationGroupID *int64,
	source string,
) {
	if s.Broadcaster == nil || endedVisit == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", endedVisit.ActiveGroupID)
	eduGroupIDs := eduGroupIDsOf(educationGroupID)

	realtimeevents.PublishVisitCheckOut(ctx, s.Broadcaster, s.getLogger(), realtimeevents.VisitChange{
		ActiveGroupID:    activeGroupID,
		StudentID:        fmt.Sprintf("%d", endedVisit.StudentID),
		EducationGroupID: educationGroupID,
		Source:           source,
		Attendance:       attendanceDetail(snapshot),
	})

	// Notify every client of the tenant so dashboard counts refresh, scoped to
	// the affected educational group when known (#2057).
	s.broadcastSupervisionRefresh(ctx, activeGroupID, activeSupervisionReasonStudentMoved, eduGroupIDs)
}

// broadcastVisitMoved publishes a checkout from the source active group and a
// checkin into the target active group. Attendance mutation happens before this
// helper so synchronization does not depend on a broadcaster being configured.
func (s *service) broadcastVisitMoved(
	ctx context.Context,
	previousVisit, movedVisit *studentpresence.Visit,
	sourceSnapshot, targetSnapshot *AttendanceSnapshot,
) {
	if previousVisit == nil || movedVisit == nil {
		return
	}

	// Moving a visit used to delegate to broadcastVisitCreated, which also
	// wakes guardians. Keep that side effect while reusing the resolved student.
	s.wakeGuardiansAfterCommit(ctx, movedVisit.StudentID)
	if s.Broadcaster == nil {
		return
	}
	educationGroupID := s.getEducationGroupForSSE(ctx, movedVisit.StudentID)
	s.emitVisitCheckout(ctx, previousVisit, sourceSnapshot, educationGroupID, "")
	s.emitVisitCreated(ctx, movedVisit, targetSnapshot, educationGroupID)
}

// broadcastStudentCheckoutEvents sends a batched checkout SSE notification when
// a whole session ends. All visits in the batch share one active_group_id (the
// session). Instead of one student_checkout per student — which produced 2N+
// events on a single client channel and overflowed its 10-slot buffer
// (issue #848) — this emits one bulk_student_checkout per affected topic:
// one to the active-group topic carrying every student, and one per distinct
// educational group carrying that group's students. SSE events are triggers,
// not payloads (the client refetches via bulk endpoints), so the per-student
// attendance snapshot is not carried in this bulk event; only the student IDs
// drive the client's per-student detail-cache invalidation. Slot-attendance
// checkout persistence still runs before this helper for every ended visit.
func (s *service) broadcastStudentCheckoutEvents(ctx context.Context, sessionIDStr string, visitsToNotify []visitSSEData) {
	if len(visitsToNotify) == 0 {
		return
	}

	// Collect every student ID for the active-group topic, and bucket them by
	// educational group for the per-edu-group topics.
	allStudentIDs := make([]string, 0, len(visitsToNotify))
	eduGroups := make(map[int64][]string)
	for _, visitData := range visitsToNotify {
		idStr := strconv.FormatInt(visitData.StudentID, 10)
		allStudentIDs = append(allStudentIDs, idStr)
		if visitData.EducationGroupID != nil {
			gid := *visitData.EducationGroupID
			eduGroups[gid] = append(eduGroups[gid], idStr)
		}
	}

	// Every affected educational group id, for scoped client-side invalidation
	// (#2057). Students without an OGS group contribute no id; they appear in
	// no ogs-students-{gid} list, so the scope stays correct.
	allEduGroupIDs := make([]string, 0, len(eduGroups))
	for gid := range eduGroups {
		allEduGroupIDs = append(allEduGroupIDs, strconv.FormatInt(gid, 10))
	}

	// One event to the active-group topic carrying every checked-out student
	// and every affected educational group.
	realtimeevents.PublishBulkStudentChange(ctx, s.Broadcaster, s.getLogger(), false, sessionIDStr, allStudentIDs, allEduGroupIDs)

	// One event per distinct educational group, carrying only that group's
	// students so each subscribed client invalidates the right detail caches.
	for gid, ids := range eduGroups {
		realtimeevents.PublishBulkStudentChangeToEducationGroup(ctx, s.Broadcaster, s.getLogger(), false, sessionIDStr, gid, ids)
	}

	// Single tenant-wide broadcast for the entire batch, scoped to the
	// affected educational groups (#2057).
	s.broadcastSupervisionRefresh(ctx, sessionIDStr, activeSupervisionReasonStudentMoved, allEduGroupIDs)
}

// broadcastActivityEndEvent sends the activity_end SSE event for a completed
// session. It performs no database access: data was resolved inside the
// ending transaction, because this runs from an after-commit hook where no
// tenant role is set (#2951).
func (s *service) broadcastActivityEndEvent(ctx context.Context, sessionIDStr string, data activityEndSSEData) {
	realtimeevents.PublishActivityEnd(ctx, s.Broadcaster, s.getLogger(), realtimeevents.ActivitySession{
		ActiveGroupID: sessionIDStr,
		ActivityName:  data.ActivityName,
		RoomID:        fmt.Sprintf("%d", data.RoomID),
		RoomName:      data.RoomName,
	})

	// Notify every client of the tenant (including zero-topic) so dashboards
	// refresh. No group scope: a session end affects room occupancy across
	// groups, so clients fall back to a broad refresh (#2057).
	s.broadcastSupervisionRefresh(ctx, sessionIDStr, activeSupervisionReasonActivityEnded, nil)
}

func (s *service) findActivityName(ctx context.Context, groupID *int64) (string, error) {
	if groupID == nil {
		return "", nil
	}
	activity, err := s.ActivityGroupRepo.FindByID(ctx, *groupID)
	if err != nil {
		return "", err
	}
	if activity == nil {
		return "", nil
	}
	return activity.Name, nil
}

func (s *service) findRoomName(ctx context.Context, roomID int64) (string, error) {
	room, err := s.RoomRepo.FindByID(ctx, roomID)
	if err != nil {
		return "", err
	}
	if room == nil {
		return "", nil
	}
	return room.Name, nil
}

// getActivityName retrieves an activity name for an SSE event. A nil groupID
// marks a spontaneous session (WP-B6), which has no template to look up.
func (s *service) getActivityName(ctx context.Context, groupID *int64) string {
	name, err := s.findActivityName(ctx, groupID)
	if err != nil {
		s.getLogger().Warn("SSE activity name lookup failed",
			slog.Int64("activity_group_id", *groupID),
			slog.String("error", err.Error()),
		)
	}
	return name
}

// getRoomName retrieves a room name for an SSE event.
func (s *service) getRoomName(ctx context.Context, roomID int64) string {
	name, err := s.findRoomName(ctx, roomID)
	if err != nil {
		s.getLogger().Warn("SSE room name lookup failed",
			slog.Int64("room_id", roomID),
			slog.String("error", err.Error()),
		)
	}
	return name
}

// getActivityEndActivityName keeps an optional lookup from aborting the
// session-ending transaction. PostgreSQL marks a transaction failed after a
// query error, so the lookup has to use a savepoint before it can be ignored.
func (s *service) getActivityEndActivityName(ctx context.Context, groupID *int64) (string, error) {
	if groupID == nil {
		return "", nil
	}
	return s.getActivityEndName(ctx, func(ctx context.Context) (string, error) {
		return s.findActivityName(ctx, groupID)
	}, "SSE activity name lookup failed", slog.Int64("activity_group_id", *groupID))
}

func (s *service) getActivityEndRoomName(ctx context.Context, roomID int64) (string, error) {
	return s.getActivityEndName(ctx, func(ctx context.Context) (string, error) {
		return s.findRoomName(ctx, roomID)
	}, "SSE room name lookup failed", slog.Int64("room_id", roomID))
}

func (s *service) getActivityEndName(
	ctx context.Context,
	lookup func(context.Context) (string, error),
	message string,
	attrs slog.Attr,
) (string, error) {
	var name string
	lookupFn := func(lookupCtx context.Context) error {
		var err error
		name, err = lookup(lookupCtx)
		return err
	}

	if _, inTx := tenant.TransactionFromContext(ctx); inTx {
		if err := tenant.WithSavepoint(ctx, lookupFn); err != nil {
			if errors.Is(err, tenant.ErrSavepointControl) {
				return "", err
			}
			s.getLogger().LogAttrs(ctx, slog.LevelWarn, message,
				attrs,
				slog.String("error", err.Error()),
			)
			return "", nil
		}
		return name, nil
	}

	if err := lookupFn(ctx); err != nil {
		s.getLogger().LogAttrs(ctx, slog.LevelWarn, message,
			attrs,
			slog.String("error", err.Error()),
		)
		return "", nil
	}
	return name, nil
}
