package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The attendance mirror (WP-B10) behind timetable.AttendanceMirror. Student
// Presence triggers it on every visit write, inside its own tenant
// transaction, to
//
//  1. mirror check-in writes into schedule.instance_students (status flips
//     'expected' → 'present', checked_in_at stamped), and
//  2. resolve the current attendance row so the presence events carry the
//     attendance fields.
//
// Missing timetable assignments are valid no-ops. Read and write failures
// return errors to the caller, which owns the shared tenant transaction and
// rolls back presence and timetable writes together. The Timetable owner is
// the only runtime writer of these rows.

// AttendanceMirrorDependencies wires the mirror; both repositories are
// required, the logger falls back to slog.Default.
type AttendanceMirrorDependencies struct {
	Instances    scheduleModel.ActivityInstanceRepository
	Participants scheduleModel.InstanceStudentRepository
	Logger       *slog.Logger
}

type attendanceMirror struct {
	instanceRepo        scheduleModel.ActivityInstanceRepository
	instanceStudentRepo scheduleModel.InstanceStudentRepository
	logger              *slog.Logger
}

var _ timetable.AttendanceMirror = (*attendanceMirror)(nil)

// NewAttendanceMirror composes the Timetable owner's attendance mirror.
func NewAttendanceMirror(deps AttendanceMirrorDependencies) (timetable.AttendanceMirror, error) {
	if deps.Instances == nil || deps.Participants == nil {
		return nil, errors.New("timetable attendance mirror: required repository is nil")
	}
	return &attendanceMirror{
		instanceRepo:        deps.Instances,
		instanceStudentRepo: deps.Participants,
		logger:              deps.Logger,
	}, nil
}

func (s *attendanceMirror) getLogger() *slog.Logger {
	return orDefaultLogger(s.logger)
}

// recoverMirrorPanic returns unexpected failures to the transaction owner and
// keeps the stack in the logs for diagnosis without publishing it to
// clients. The panic itself goes to Sentry (#3640): the error that replaces
// it no longer carries its stack. It must be deferred directly so recover
// sees the panic.
func (s *attendanceMirror) recoverMirrorPanic(ctx context.Context, logMessage, errPrefix string, err *error) {
	if r := recover(); r != nil {
		s.getLogger().Error(logMessage,
			slog.Any("panic", r),
			slog.String("stack", string(debug.Stack())),
		)
		reportMirrorPanic(ctx, r)
		*err = fmt.Errorf("%s: %v", errPrefix, r)
	}
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

// MirrorCheckInForVisit mirrors a visit check-in onto its slot.
//
// Branches (all logged with student_id / instance_id only — no names at
// Info level per GDPR):
//
//	B1 zero ActiveGroupID            → Debug, return nil
//	B2 instance lookup error         → return error
//	B3 no instance bridged           → Debug, return nil (walk-in)
//	B4 instance_student lookup error → return error
//	B5 no instance_student row       → persist unplanned presence
//	B6 row is manual/observably open → Debug, return current snapshot
//	B7 UPDATE error                  → return error
//	B8 UPDATE rowsAffected=0 (race)  → Debug, return snapshot of row we read
//	B9 happy path                    → Info, return new snapshot
func (s *attendanceMirror) MirrorCheckInForVisit(
	ctx context.Context, visit timetable.AttendanceVisit,
) (snapshot *timetable.AttendanceSnapshot, err error) {
	// A panic leaves the snapshot nil: it is only ever set by a return.
	defer s.recoverMirrorPanic(ctx, "attendance mirror panic", "visit check-in sync panic", &err)

	if visit.ActiveGroupID <= 0 {
		s.getLogger().Debug("attendance mirror: visit has no active_group_id, skipping")
		return nil, nil
	}
	instance, row, err := s.loadVisitSlot(ctx, visit, "attendance mirror", "visit check-in sync")
	if err != nil || instance == nil {
		if err == nil {
			s.getLogger().Debug("attendance mirror: no instance bridged to active_group, walk-in",
				slog.Int64("active_group_id", visit.ActiveGroupID),
			)
		}
		return nil, err
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
	return s.openVisitPresence(ctx, instance.ID, visit, row)
}

// loadVisitSlot resolves the block bridged to the visit's session and the
// child's slot on it. A nil instance means a walk-in; a nil row means the
// child has no slot there.
func (s *attendanceMirror) loadVisitSlot(
	ctx context.Context, visit timetable.AttendanceVisit, logPrefix, errPrefix string,
) (*scheduleModel.ActivityInstance, *scheduleModel.InstanceStudent, error) {
	instance, err := s.instanceRepo.FindByActiveGroupID(ctx, visit.ActiveGroupID)
	if err != nil {
		s.getLogger().Warn(logPrefix+": find instance by active_group_id failed",
			slog.Int64("active_group_id", visit.ActiveGroupID),
			slog.String("error", err.Error()),
		)
		return nil, nil, fmt.Errorf("%s: find instance: %w", errPrefix, err)
	}
	if instance == nil {
		return nil, nil, nil
	}
	row, err := s.instanceStudentRepo.FindByInstanceAndStudent(ctx, instance.ID, visit.StudentID)
	if err != nil {
		s.getLogger().Warn(logPrefix+": find instance_student failed",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("error", err.Error()),
		)
		return nil, nil, fmt.Errorf("%s: find slot: %w", errPrefix, err)
	}
	return instance, row, nil
}

// openVisitPresence flips an expected (or projected-absent, or checked-out)
// slot to present from the visit's entry.
func (s *attendanceMirror) openVisitPresence(
	ctx context.Context, instanceID int64, visit timetable.AttendanceVisit, row *scheduleModel.InstanceStudent,
) (*timetable.AttendanceSnapshot, error) {
	// checked_in_at is stamped from visit.EntryTime — the same instant
	// active.attendance.check_in_time gets (createAttendanceRecord). History
	// and export session-to-slot matching relies on the two timestamps being
	// identical; never replace either side with an independent time.Now().
	updated, err := s.instanceStudentRepo.UpdateAttendanceFromCheckin(ctx, instanceID, visit.StudentID, visit.EntryTime)
	if err != nil {
		// Propagate failures even when PostgreSQL has not aborted the transaction.
		s.getLogger().Error("attendance mirror UPDATE failed",
			slog.Int64("tenant_id", tenant.FromContext(ctx)),
			slog.Int64("instance_id", instanceID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("visit check-in sync: update slot: %w", err)
	}

	if !updated {
		// B8: Row existed with status='expected' at the read in B6 but the
		// UPDATE's WHERE failed to match. Concurrent write (another tab,
		// admin PATCH) moved it out of expected between the read and write.
		// We still return the snapshot we read so SSE fires with *something*;
		// the persisted state may differ, but that's acceptable for a
		// fire-and-forget notification.
		s.getLogger().Debug("attendance mirror: race — row moved out of expected between read and UPDATE",
			slog.Int64("instance_id", instanceID),
			slog.Int64("student_id", visit.StudentID),
		)
		return snapshotFromRow(row), nil
	}

	// B9: happy path. Row flipped to present. Build the snapshot from the
	// new state (status=present) and the unchanged substatus/note fields.
	// IDs-only at Info level per GDPR.
	s.getLogger().Info("attendance mirror synced on check-in",
		slog.Int64("instance_id", instanceID),
		slog.Int64("student_id", visit.StudentID),
	)
	markOpenedPresence(row, visit.EntryTime)
	return s.finishVisitInterval(ctx, instanceID, visit, row)
}

// markOpenedPresence mirrors the repository's check-in UPDATE on the row
// already read: present, the plan provenance released, and a reopen of a
// checked-out row re-stamps checked_in_at with the re-entry time.
func markOpenedPresence(row *scheduleModel.InstanceStudent, at time.Time) {
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

func (s *attendanceMirror) createUnplannedAttendance(
	ctx context.Context,
	instanceID int64,
	visit timetable.AttendanceVisit,
) (*timetable.AttendanceSnapshot, error) {
	row, err := s.instanceStudentRepo.CreateUnplannedPresentIfAbsent(
		ctx, instanceID, visit.StudentID, visit.EntryTime,
	)
	if err != nil {
		s.getLogger().Error("attendance mirror: persist unplanned slot attendance failed",
			slog.Int64("instance_id", instanceID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("visit check-in sync: create unplanned slot: %w", err)
	}
	s.getLogger().Info("attendance mirror: persisted unplanned slot attendance",
		slog.Int64("instance_id", instanceID),
		slog.Int64("student_id", visit.StudentID),
	)
	return s.finishVisitInterval(ctx, instanceID, visit, row)
}

func (s *attendanceMirror) finishVisitInterval(
	ctx context.Context,
	instanceID int64,
	visit timetable.AttendanceVisit,
	row *scheduleModel.InstanceStudent,
) (*timetable.AttendanceSnapshot, error) {
	if !closesPresentInterval(visit.ExitTime, row) {
		return snapshotFromRow(row), nil
	}
	if err := s.instanceStudentRepo.UpdateAttendanceCheckout(
		ctx, instanceID, visit.StudentID, *visit.ExitTime,
	); err != nil {
		s.getLogger().Error("attendance mirror: persist completed visit checkout failed",
			slog.Int64("instance_id", instanceID),
			slog.Int64("student_id", visit.StudentID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("visit check-in sync: close completed interval: %w", err)
	}
	row.CheckedOutAt = visit.ExitTime
	return snapshotFromRow(row), nil
}

// closesPresentInterval reports whether an exit closes the row's open
// presence: the row is present with a check-in no later than the exit.
func closesPresentInterval(exit *time.Time, row *scheduleModel.InstanceStudent) bool {
	return exit != nil && row != nil &&
		row.Status == scheduleModel.AttendanceStatusPresent && row.CheckedInAt != nil &&
		!exit.Before(*row.CheckedInAt)
}

// MirrorCheckInAt resolves a roomless check-in only when exactly one booked
// slot currently matches. Ambiguous and unbooked check-ins deliberately stay
// unassigned; assigning either would invent business data.
func (s *attendanceMirror) MirrorCheckInAt(
	ctx context.Context, studentID int64, at time.Time,
) (snapshot *timetable.AttendanceSnapshot, err error) {
	// A panic leaves the snapshot nil: it is only ever set by a return.
	defer s.recoverMirrorPanic(ctx, "roomless attendance mirror panic", "roomless check-in sync panic", &err)

	rows, err := s.instanceStudentRepo.FindCurrentCandidates(
		ctx, studentID, scheduleModel.DateFromTime(at), at,
	)
	if err != nil {
		s.getLogger().Warn("roomless attendance mirror: candidate lookup failed",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("roomless check-in sync: read candidates: %w", err)
	}
	if len(rows) != 1 {
		s.getLogger().Debug("roomless attendance mirror: slot assignment is not unique",
			slog.Int64("student_id", studentID),
			slog.Int("candidate_count", len(rows)),
		)
		return nil, nil
	}

	row := rows[0]
	if shouldPreserveAttendanceOnCheckin(row) {
		return snapshotFromRow(row), nil
	}
	updated, err := s.instanceStudentRepo.UpdateAttendanceFromCheckin(ctx, row.InstanceID, studentID, at)
	if err != nil {
		s.getLogger().Error("roomless attendance mirror UPDATE failed",
			slog.Int64("instance_id", row.InstanceID),
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("roomless check-in sync: update slot: %w", err)
	}
	if updated {
		markOpenedPresence(row, at)
	}
	return snapshotFromRow(row), nil
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

// MirrorCheckOutForVisit preserves the slot status while recording the
// observed checkout for history/export.
func (s *attendanceMirror) MirrorCheckOutForVisit(
	ctx context.Context, visit timetable.AttendanceVisit,
) (snapshot *timetable.AttendanceSnapshot, err error) {
	// A panic leaves the snapshot nil: it is only ever set by a return.
	defer s.recoverMirrorPanic(ctx, "attendance load panic", "visit checkout sync panic", &err)

	if visit.ActiveGroupID <= 0 {
		return nil, nil
	}
	instance, row, err := s.loadVisitSlot(ctx, visit, "attendance load", "visit checkout sync")
	if err != nil || instance == nil || row == nil {
		return nil, err
	}
	if closesPresentInterval(visit.ExitTime, row) {
		if err := s.instanceStudentRepo.UpdateAttendanceCheckout(
			ctx, instance.ID, visit.StudentID, *visit.ExitTime,
		); err != nil {
			s.getLogger().Error("attendance mirror: persist slot checkout failed",
				slog.Int64("instance_id", instance.ID),
				slog.Int64("student_id", visit.StudentID),
				slog.String("error", err.Error()),
			)
			return nil, fmt.Errorf("visit checkout sync: update slot: %w", err)
		}
		row.CheckedOutAt = visit.ExitTime
	}

	return snapshotFromRow(row), nil
}

// MirrorVisitRevision updates the interval represented by a slot after staff
// edit or reopen a visit. The repository compares the previous check-in first,
// so an older visit cannot overwrite a later re-entry in the same slot. It may
// also repair a missing checkout left by an older completed-visit write.
func (s *attendanceMirror) MirrorVisitRevision(
	ctx context.Context, previous, updated timetable.AttendanceVisit,
) (err error) {
	defer s.recoverMirrorPanic(ctx, "attendance visit revision mirror panic", "attendance visit revision panic", &err)

	if previous.StudentID != updated.StudentID ||
		previous.ActiveGroupID <= 0 ||
		previous.ActiveGroupID != updated.ActiveGroupID {
		return nil
	}
	instance, err := s.instanceRepo.FindByActiveGroupID(ctx, previous.ActiveGroupID)
	if err != nil {
		s.getLogger().Warn("attendance visit revision: find instance failed",
			slog.Int64("active_group_id", previous.ActiveGroupID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("attendance visit revision: find instance: %w", err)
	}
	if instance == nil {
		return nil
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
		return fmt.Errorf("attendance visit revision: reconcile interval: %w", err)
	}
	if changed {
		s.getLogger().Info("attendance visit revision synced",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("student_id", previous.StudentID),
		)
	}
	return nil
}

// snapshotFromRow is the common projection. Substatus and Note already
// pointer-typed in the model, so we reuse the same pointers — no need to
// dereference + re-address.
func snapshotFromRow(row *scheduleModel.InstanceStudent) *timetable.AttendanceSnapshot {
	if row == nil {
		return nil
	}
	return &timetable.AttendanceSnapshot{
		Status:      row.Status,
		Substatus:   row.Substatus,
		Note:        row.Note,
		InstanceID:  row.InstanceID,
		IsUnplanned: row.IsUnplanned,
	}
}
