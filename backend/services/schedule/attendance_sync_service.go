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
	"context"
	"log/slog"

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
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
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
//	B5 no instance_student row       → Debug, return nil (walk-in)
//	B6 row already non-expected      → Debug, return current snapshot
//	B7 UPDATE error                  → Error (tx likely tainted), return nil
//	B8 UPDATE rowsAffected=0 (race)  → Debug, return snapshot of row we read
//	B9 happy path                    → Info, return new snapshot
func (s *AttendanceSyncService) MirrorCheckInForVisit(
	ctx context.Context, visit *activeModel.Visit,
) (snapshot *activeSvc.AttendanceSnapshot) {
	// Panic belt-and-braces — we promise no error and no panic to the caller.
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("attendance mirror panic",
				slog.Any("panic", r),
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
		s.getLogger().Debug("attendance mirror: student not expected at instance, walk-in",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", visit.StudentID),
		)
		return nil
	}

	// B6: row is no longer 'expected' — respect existing state. This covers:
	//   * double-tap within a short window (already present)
	//   * admin marked absent before check-in via PATCH
	//   * status=present from a prior visit today
	// Return the snapshot we already read so SSE reflects the true state.
	if row.Status != scheduleModel.AttendanceStatusExpected {
		s.getLogger().Debug("attendance mirror: row already past expected, not clobbering",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("current_status", row.Status),
		)
		return snapshotFromRow(row)
	}

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
	return &activeSvc.AttendanceSnapshot{
		Status:    scheduleModel.AttendanceStatusPresent,
		Substatus: row.Substatus,
		Note:      row.Note,
	}
}

// LoadAttendanceForVisit implements activeSvc.AttendanceSyncer. Read-only
// companion to MirrorCheckInForVisit — used on the checkout SSE path.
// Same graceful-degradation branches 1–5, no write.
func (s *AttendanceSyncService) LoadAttendanceForVisit(
	ctx context.Context, visit *activeModel.Visit,
) (snapshot *activeSvc.AttendanceSnapshot) {
	defer func() {
		if r := recover(); r != nil {
			s.getLogger().Error("attendance load panic",
				slog.Any("panic", r),
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

	return snapshotFromRow(row)
}

// snapshotFromRow is the common projection. Substatus and Note already
// pointer-typed in the model, so we reuse the same pointers — no need to
// dereference + re-address.
func snapshotFromRow(row *scheduleModel.InstanceStudent) *activeSvc.AttendanceSnapshot {
	if row == nil {
		return nil
	}
	return &activeSvc.AttendanceSnapshot{
		Status:    row.Status,
		Substatus: row.Substatus,
		Note:      row.Note,
	}
}
