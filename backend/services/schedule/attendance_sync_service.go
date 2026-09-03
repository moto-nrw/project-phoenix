// Package schedule — attendance sync service (WP-B10).
//
// Implements active.AttendanceSyncer. Called from active.service.CreateVisit /
// EndVisit to:
//
//  1. Mirror check-in writes into schedule.instance_students (status flips
//     'expected' → 'present', checked_in_at stamped).
//  2. Resolve the current attendance row so the broadcast helper can
//     populate EventData.Attendance* fields on the student_checkin /
//     student_checkout SSE events.
//
// Contract: every method is graceful-degradation-by-design. Errors are
// swallowed and nil is returned. The caller (active.service) proceeds as
// if no instance was attached to the visit — which is always a valid
// outcome (walk-ins, schulhof/WC sessions, pre-start race windows).
//
// Shared-tenant-tx caveat: these methods run inside the IoT handler's
// TenantTxMiddleware transaction. If the UPDATE in MirrorCheckInForVisit
// fails with a Postgres-level error (not just sql.ErrNoRows), the tx is
// tainted and a subsequent commit failure will roll back the visit. That
// is unavoidable under the shared tx and is the one failure mode this
// service cannot gracefully degrade. Branch 7 below logs at Error level
// so the existing "backend error rate" alert surfaces it in ops.
package schedule

import (
	"cmp"
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// AttendanceSyncService implements activeSvc.AttendanceSyncer.
type AttendanceSyncService struct {
	instanceRepo        scheduleModel.ActivityInstanceRepository
	instanceStudentRepo scheduleModel.InstanceStudentRepository
	logger              *slog.Logger
}

// NewAttendanceSyncService constructs the service. Both repos are required;
// the logger is optional (falls back to slog.Default()).
func NewAttendanceSyncService(
	instanceRepo scheduleModel.ActivityInstanceRepository,
	instanceStudentRepo scheduleModel.InstanceStudentRepository,
	logger *slog.Logger,
) *AttendanceSyncService {
	return &AttendanceSyncService{
		instanceRepo:        instanceRepo,
		instanceStudentRepo: instanceStudentRepo,
		logger:              logger,
	}
}

// Verify at compile-time that the concrete type satisfies the interface
// declared in the active package.
var _ activeSvc.AttendanceSyncer = (*AttendanceSyncService)(nil)

func (s *AttendanceSyncService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// MirrorCheckInForVisit implements activeSvc.AttendanceSyncer.
//
// Branches (all logged with student_id / instance_id only — no names at
// Info level per GDPR):
//
//	B1 nil or zero ActiveGroupID     → Debug, return nil
//	B2 instance lookup error         → Warn, return nil
//	B3 no instance bridged           → Debug, return nil (walk-in)
//	B4 instance_student lookup error → Warn, return nil
//	B5 no instance_student row       → persist unplanned presence
//	B6 row is manual/observably open → Debug, return current snapshot
//	B7 UPDATE error                  → Error (tx likely tainted), return nil
//	B8 UPDATE rowsAffected=0 (race)  → Debug, return snapshot of row we read
//	B9 happy path                    → Info, return new snapshot
func (s *AttendanceSyncService) MirrorCheckInForVisit(
	ctx context.Context, visit *activeModel.Visit,
) (snapshot *activeSvc.AttendanceSnapshot) {
	// Panic belt-and-braces — we promise no error and no panic to the caller.
	// The stack is captured at Error level so an after-the-fact root cause
	// is recoverable from Grafana without having to reproduce the panic.
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("attendance mirror panic",
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			)
			snapshot = nil
		}
	}()

	if visit == nil || visit.ActiveGroupID <= 0 {
		s.getLogger().Debug("attendance mirror: visit has no active_group_id, skipping")
		return nil
	}

	instance, err := s.instanceRepo.FindByActiveGroupID(ctx, visit.ActiveGroupID)
	if err != nil {
		s.getLogger().Warn("attendance mirror: find instance by active_group_id failed",
			slog.Int64("active_group_id", visit.ActiveGroupID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	if instance == nil {
		s.getLogger().Debug("attendance mirror: no instance bridged to active_group, walk-in",
			slog.Int64("active_group_id", visit.ActiveGroupID),
		)
		return nil
	}

	row, err := s.instanceStudentRepo.FindByInstanceAndStudent(ctx, instance.ID, visit.StudentID)
	if err != nil {
		s.getLogger().Warn("attendance mirror: find instance_student failed",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	if row == nil {
		return s.createUnplannedAttendance(ctx, instance.ID, visit)
	}

	// B6: respect manual states and already-open presence. A checked-out
	// present row is deliberately excluded: re-entry into the same care slot
	// must reopen it so history does not claim the child is still checked out.
	// This covers:
	//   * double-tap within a short window (present, checked_out_at=NULL)
	//   * admin marked absent before check-in via PATCH
	// Return the snapshot we already read so SSE reflects the true state.
	if shouldPreserveAttendanceOnCheckin(row) {
		s.getLogger().Debug("attendance mirror: row already past expected, not clobbering",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("current_status", row.Status),
		)
		return s.finishVisitInterval(ctx, instance.ID, visit, row)
	}

	// checked_in_at is stamped from visit.EntryTime — the same instant
	// active.attendance.check_in_time gets (createAttendanceRecord). History
	// and export session-to-slot matching relies on the two timestamps being
	// identical; never replace either side with an independent time.Now().
	updated, err := s.instanceStudentRepo.UpdateAttendanceFromCheckin(
		ctx, instance.ID, visit.StudentID, visit.EntryTime,
	)
	if err != nil {
		// B7: This is the loud one. If the UPDATE fails with a Postgres-level
		// error (FK violation, aborted-tx, constraint), the tenant tx is now
		// tainted — the TenantTxMiddleware will 5xx on commit, rolling back
		// the visit write too. Error level (not Warn) so the Grafana
		// "backend error rate" alert picks it up and we find out in ops
		// rather than via a customer-reported ghost-visit incident.
		tenantID := tenant.FromContext(ctx)
		s.getLogger().Error("attendance mirror UPDATE failed — tenant tx likely tainted, visit write at risk",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}

	if !updated {
		// B8: Row existed with status='expected' at the read in B6 but the
		// UPDATE's WHERE failed to match. Concurrent write (another tab,
		// admin PATCH) moved it out of expected between the read and write.
		// We still return the snapshot we read so SSE fires with *something*;
		// the persisted state may differ, but that's acceptable for a
		// fire-and-forget notification.
		s.getLogger().Debug("attendance mirror: race — row moved out of expected between read and UPDATE",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", visit.StudentID),
		)
		return snapshotFromRow(row)
	}

	// B9: happy path. Row flipped to present. Build the snapshot from the
	// new state (status=present) and the unchanged substatus/note fields.
	// IDs-only at Info level per GDPR.
	s.getLogger().Info("attendance mirror synced on check-in",
		slog.Int64("instance_id", instance.ID),
		slog.Int64("student_id", visit.StudentID),
	)
	row.Status = scheduleModel.AttendanceStatusPresent
	if row.StudentStatusDayID != nil || row.PickupExceptionID != nil {
		row.Substatus = nil
	}
	row.StudentStatusDayID = nil
	row.PickupExceptionID = nil
	// A reopen (checked-out row) re-stamps checked_in_at with the re-entry
	// time — mirrors the repo UPDATE's session boundary.
	if row.CheckedOutAt != nil || row.CheckedInAt == nil {
		row.CheckedInAt = &visit.EntryTime
	}
	row.CheckedOutAt = nil
	return s.finishVisitInterval(ctx, instance.ID, visit, row)
}

func (s *AttendanceSyncService) createUnplannedAttendance(
	ctx context.Context,
	instanceID int64,
	visit *activeModel.Visit,
) *activeSvc.AttendanceSnapshot {
	row, err := s.instanceStudentRepo.CreateUnplannedPresentIfAbsent(
		ctx, instanceID, visit.StudentID, visit.EntryTime,
	)
	if err != nil {
		s.getLogger().Error("attendance mirror: persist unplanned slot attendance failed",
			slog.Int64("instance_id", instanceID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	s.getLogger().Info("attendance mirror: persisted unplanned slot attendance",
		slog.Int64("instance_id", instanceID),
		slog.Int64("student_id", visit.StudentID),
	)
	return s.finishVisitInterval(ctx, instanceID, visit, row)
}

func (s *AttendanceSyncService) finishVisitInterval(
	ctx context.Context,
	instanceID int64,
	visit *activeModel.Visit,
	row *scheduleModel.InstanceStudent,
) *activeSvc.AttendanceSnapshot {
	if visit == nil || visit.ExitTime == nil || row == nil ||
		row.Status != scheduleModel.AttendanceStatusPresent || row.CheckedInAt == nil ||
		visit.ExitTime.Before(*row.CheckedInAt) {
		return snapshotFromRow(row)
	}
	if err := s.instanceStudentRepo.UpdateAttendanceCheckout(
		ctx, instanceID, visit.StudentID, *visit.ExitTime,
	); err != nil {
		s.getLogger().Error("attendance mirror: persist completed visit checkout failed",
			slog.Int64("instance_id", instanceID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	row.CheckedOutAt = visit.ExitTime
	return snapshotFromRow(row)
}

// MirrorCheckInAt resolves a roomless check-in only when exactly one booked
// slot currently matches. Ambiguous and unbooked check-ins deliberately stay
// unassigned; assigning either would invent business data.
func (s *AttendanceSyncService) MirrorCheckInAt(
	ctx context.Context, studentID int64, at time.Time,
) (snapshot *activeSvc.AttendanceSnapshot) {
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("roomless attendance mirror panic",
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			)
			snapshot = nil
		}
	}()

	rows, err := s.instanceStudentRepo.FindCurrentCandidates(
		ctx, studentID, timezone.DateFromTime(at), at,
	)
	if err != nil {
		s.getLogger().Warn("roomless attendance mirror: candidate lookup failed",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	if len(rows) != 1 {
		s.getLogger().Debug("roomless attendance mirror: slot assignment is not unique",
			slog.Int64("student_id", studentID),
			slog.Int("candidate_count", len(rows)),
		)
		return nil
	}

	row := rows[0]
	if shouldPreserveAttendanceOnCheckin(row) {
		return snapshotFromRow(row)
	}
	updated, err := s.instanceStudentRepo.UpdateAttendanceFromCheckin(ctx, row.InstanceID, studentID, at)
	if err != nil {
		s.getLogger().Error("roomless attendance mirror UPDATE failed",
			slog.Int64("instance_id", row.InstanceID),
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	if updated {
		row.Status = scheduleModel.AttendanceStatusPresent
		if row.StudentStatusDayID != nil || row.PickupExceptionID != nil {
			row.Substatus = nil
		}
		row.StudentStatusDayID = nil
		row.PickupExceptionID = nil
		if row.CheckedOutAt != nil || row.CheckedInAt == nil {
			row.CheckedInAt = &at
		}
		row.CheckedOutAt = nil
	}
	return snapshotFromRow(row)
}

func shouldPreserveAttendanceOnCheckin(row *scheduleModel.InstanceStudent) bool {
	// Expected rows, broad day-status absences, and partial-excusal absences
	// are all plan projections: a real check-in may replace them. Manual
	// absences (no provenance) and already-open presence stay put.
	if row == nil ||
		row.Status == scheduleModel.AttendanceStatusExpected ||
		row.StudentStatusDayID != nil ||
		row.PickupExceptionID != nil {
		return false
	}
	return row.Status != scheduleModel.AttendanceStatusPresent || row.CheckedOutAt == nil
}

// MirrorCheckOutForVisit implements activeSvc.AttendanceSyncer. It preserves
// the slot status while recording the observed checkout for history/export.
func (s *AttendanceSyncService) MirrorCheckOutForVisit(
	ctx context.Context, visit *activeModel.Visit,
) (snapshot *activeSvc.AttendanceSnapshot) {
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("attendance load panic",
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			)
			snapshot = nil
		}
	}()

	if visit == nil || visit.ActiveGroupID <= 0 {
		return nil
	}

	instance, err := s.instanceRepo.FindByActiveGroupID(ctx, visit.ActiveGroupID)
	if err != nil {
		s.getLogger().Warn("attendance load: find instance by active_group_id failed",
			slog.Int64("active_group_id", visit.ActiveGroupID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	if instance == nil {
		return nil
	}

	row, err := s.instanceStudentRepo.FindByInstanceAndStudent(ctx, instance.ID, visit.StudentID)
	if err != nil {
		s.getLogger().Warn("attendance load: find instance_student failed",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	if row == nil {
		return nil
	}
	if visit.ExitTime != nil &&
		row.Status == scheduleModel.AttendanceStatusPresent &&
		row.CheckedInAt != nil &&
		!visit.ExitTime.Before(*row.CheckedInAt) {
		if err := s.instanceStudentRepo.UpdateAttendanceCheckout(
			ctx, instance.ID, visit.StudentID, *visit.ExitTime,
		); err != nil {
			s.getLogger().Error("attendance mirror: persist slot checkout failed",
				slog.Int64("instance_id", instance.ID),
				slog.Int64("student_id", visit.StudentID),
				slog.String("error", err.Error()),
			)
			return nil
		}
		row.CheckedOutAt = visit.ExitTime
	}

	return snapshotFromRow(row)
}

// MirrorVisitRevision updates the interval represented by a slot after staff
// edit or reopen a visit. The repository compares the previous check-in first,
// so an older visit cannot overwrite a later re-entry in the same slot. It may
// also repair a missing checkout left by an older completed-visit write.
func (s *AttendanceSyncService) MirrorVisitRevision(
	ctx context.Context, previous, updated *activeModel.Visit,
) {
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("attendance visit revision mirror panic",
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			)
		}
	}()

	if previous == nil || updated == nil ||
		previous.StudentID != updated.StudentID ||
		previous.ActiveGroupID <= 0 ||
		previous.ActiveGroupID != updated.ActiveGroupID {
		return
	}
	instance, err := s.instanceRepo.FindByActiveGroupID(ctx, previous.ActiveGroupID)
	if err != nil {
		s.getLogger().Warn("attendance visit revision: find instance failed",
			slog.Int64("active_group_id", previous.ActiveGroupID),
			slog.String("error", err.Error()),
		)
		return
	}
	if instance == nil {
		return
	}
	changed, err := s.instanceStudentRepo.ReconcileAttendanceInterval(
		ctx,
		instance.ID,
		previous.StudentID,
		previous.EntryTime,
		previous.ExitTime,
		updated.EntryTime,
		updated.ExitTime,
	)
	if err != nil {
		s.getLogger().Error("attendance visit revision: reconcile slot interval failed",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", previous.StudentID),
			slog.String("error", err.Error()),
		)
		return
	}
	if changed {
		s.getLogger().Info("attendance visit revision synced",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", previous.StudentID),
		)
	}
}

// MirrorCheckOutAt closes the latest open slot attendance for roomless binary
// mode. It never changes another slot's status or history.
func (s *AttendanceSyncService) MirrorCheckOutAt(ctx context.Context, studentID int64, at time.Time) {
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("roomless attendance checkout mirror panic",
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			)
		}
	}()

	day := timezone.DateFromTime(at)
	rows, err := s.instanceStudentRepo.FindByStudentAndDateRange(ctx, studentID, day, day)
	if err != nil {
		s.getLogger().Warn("roomless attendance checkout mirror: slot lookup failed",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return
	}
	var latest *scheduleModel.InstanceStudent
	for _, row := range rows {
		if row.Status != scheduleModel.AttendanceStatusPresent || row.CheckedInAt == nil || row.CheckedOutAt != nil {
			continue
		}
		if latest == nil || row.CheckedInAt.After(*latest.CheckedInAt) {
			latest = row
		}
	}
	if latest == nil {
		return
	}
	if err := s.instanceStudentRepo.UpdateAttendanceCheckout(ctx, latest.InstanceID, studentID, at); err != nil {
		s.getLogger().Error("roomless attendance checkout mirror UPDATE failed",
			slog.Int64("instance_id", latest.InstanceID),
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
	}
}

// MirrorCheckInAtBatch resolves many roomless check-ins at one shared instant
// with one candidate query and one guarded UPDATE (review #2372). Per student
// the same rule as MirrorCheckInAt applies: exactly one currently-running
// booked slot, ambiguous or unbooked students stay unassigned, and rows in a
// manual or already-open state are preserved.
func (s *AttendanceSyncService) MirrorCheckInAtBatch(ctx context.Context, studentIDs []int64, at time.Time) {
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("roomless attendance batch mirror panic",
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			)
		}
	}()
	if len(studentIDs) == 0 {
		return
	}

	rows, err := s.instanceStudentRepo.FindCurrentCandidatesByStudentIDs(
		ctx, studentIDs, timezone.DateFromTime(at), at,
	)
	if err != nil {
		s.getLogger().Warn("roomless attendance batch mirror: candidate lookup failed",
			slog.String("error", err.Error()),
		)
		return
	}

	byStudent := make(map[int64][]*scheduleModel.InstanceStudent, len(studentIDs))
	for _, row := range rows {
		byStudent[row.StudentID] = append(byStudent[row.StudentID], row)
	}

	keys := make([]scheduleModel.InstanceStudentKey, 0, len(byStudent))
	for studentID, candidates := range byStudent {
		if len(candidates) != 1 {
			s.getLogger().Debug("roomless attendance batch mirror: slot assignment is not unique",
				slog.Int64("student_id", studentID),
				slog.Int("candidate_count", len(candidates)),
			)
			continue
		}
		if shouldPreserveAttendanceOnCheckin(candidates[0]) {
			continue
		}
		keys = append(keys, scheduleModel.InstanceStudentKey{
			InstanceID: candidates[0].InstanceID,
			StudentID:  studentID,
		})
	}
	if len(keys) == 0 {
		return
	}
	if err := s.instanceStudentRepo.UpdateAttendanceFromCheckinBatch(ctx, keys, at); err != nil {
		s.getLogger().Error("roomless attendance batch mirror UPDATE failed",
			slog.Int("row_count", len(keys)),
			slog.String("error", err.Error()),
		)
	}
}

// MirrorCheckOutAtBatch closes many students' latest open slot attendance at
// one shared instant: one slot query and one guarded UPDATE (review #2372).
// Per student the same rule as MirrorCheckOutAt applies — only the most
// recently checked-in open present row of the day closes.
func (s *AttendanceSyncService) MirrorCheckOutAtBatch(ctx context.Context, studentIDs []int64, at time.Time) {
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("roomless attendance batch checkout mirror panic",
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			)
		}
	}()
	if len(studentIDs) == 0 {
		return
	}

	day := timezone.DateFromTime(at)
	rows, err := s.instanceStudentRepo.FindByStudentIDsAndDate(ctx, studentIDs, day)
	if err != nil {
		s.getLogger().Warn("roomless attendance batch checkout mirror: slot lookup failed",
			slog.String("error", err.Error()),
		)
		return
	}

	latestByStudent := make(map[int64]*scheduleModel.InstanceStudent, len(studentIDs))
	for _, row := range rows {
		if row.Status != scheduleModel.AttendanceStatusPresent || row.CheckedInAt == nil || row.CheckedOutAt != nil {
			continue
		}
		latest := latestByStudent[row.StudentID]
		if latest == nil || row.CheckedInAt.After(*latest.CheckedInAt) {
			latestByStudent[row.StudentID] = row
		}
	}
	keys := make([]scheduleModel.InstanceStudentKey, 0, len(latestByStudent))
	for studentID, row := range latestByStudent {
		keys = append(keys, scheduleModel.InstanceStudentKey{InstanceID: row.InstanceID, StudentID: studentID})
	}
	if len(keys) == 0 {
		return
	}
	if err := s.instanceStudentRepo.UpdateAttendanceCheckoutBatch(ctx, keys, at); err != nil {
		s.getLogger().Error("roomless attendance batch checkout mirror UPDATE failed",
			slog.Int("row_count", len(keys)),
			slog.String("error", err.Error()),
		)
	}
}

// MirrorCheckOutForVisits stamps the slot checkout for many visits ended at
// one shared instant. Instances are resolved once per distinct active group
// and all slot checkouts land in one guarded UPDATE (review #2372) — the
// per-visit read of MirrorCheckOutForVisit exists only for its SSE snapshot,
// which bulk events do not carry (#848), so the batch skips it and relies on
// the UPDATE's own guards.
func (s *AttendanceSyncService) MirrorCheckOutForVisits(ctx context.Context, visits []*activeModel.Visit, at time.Time) {
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("attendance batch visit checkout mirror panic",
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			)
		}
	}()
	if len(visits) == 0 {
		return
	}

	groupIDs := make([]int64, 0, len(visits))
	seenGroups := make(map[int64]bool, len(visits))
	for _, visit := range visits {
		if visit != nil && visit.ActiveGroupID > 0 && !seenGroups[visit.ActiveGroupID] {
			seenGroups[visit.ActiveGroupID] = true
			groupIDs = append(groupIDs, visit.ActiveGroupID)
		}
	}
	instances, err := s.instanceRepo.FindByActiveGroupIDs(ctx, groupIDs)
	if err != nil {
		s.getLogger().Warn("attendance batch mirror: find instances by active_group_id failed", slog.String("error", err.Error()))
		return
	}
	instanceByGroup := make(map[int64]*scheduleModel.ActivityInstance, len(instances))
	for _, instance := range instances {
		if instance.ActiveGroupID != nil {
			instanceByGroup[*instance.ActiveGroupID] = instance
		}
	}
	keys := make([]scheduleModel.InstanceStudentKey, 0, len(visits))
	for _, visit := range visits {
		if visit == nil || visit.ActiveGroupID <= 0 {
			continue
		}
		instance := instanceByGroup[visit.ActiveGroupID]
		if instance == nil {
			continue // walk-in / no bridged instance — nothing to mirror
		}
		keys = append(keys, scheduleModel.InstanceStudentKey{InstanceID: instance.ID, StudentID: visit.StudentID})
	}
	if len(keys) == 0 {
		return
	}
	if err := s.instanceStudentRepo.UpdateAttendanceCheckoutBatch(ctx, keys, at); err != nil {
		s.getLogger().Error("attendance batch visit checkout mirror UPDATE failed",
			slog.Int("row_count", len(keys)),
			slog.String("error", err.Error()),
		)
	}
}

// snapshotFromRow is the common projection. Substatus and Note already
// pointer-typed in the model, so we reuse the same pointers — no need to
// dereference + re-address.
func snapshotFromRow(row *scheduleModel.InstanceStudent) *activeSvc.AttendanceSnapshot {
	if row == nil {
		return nil
	}
	return &activeSvc.AttendanceSnapshot{
		Status:      row.Status,
		Substatus:   row.Substatus,
		Note:        row.Note,
		InstanceID:  row.InstanceID,
		IsUnplanned: row.IsUnplanned,
	}
}
