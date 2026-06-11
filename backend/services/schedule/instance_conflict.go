// Package schedule — instance conflict detection (WP-B9).
//
// Soft-warning detection fired on POST /instances/{id}/start. All three
// sub-checks are read-only and NEVER block the transition: they run inside
// the caller's tenant transaction so a roll-back discards them cleanly, and
// the transition proceeds regardless of what they find. Every warning carries
// can_override=true in v1; a future WP may introduce hard blocks.
package schedule

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// Conflict kinds — stable string values exported to clients so the frontend
// can switch on kind rather than parsing German messages.
const (
	ConflictKindRoom    = "room"
	ConflictKindStaff   = "staff"
	ConflictKindStudent = "student"
)

// InstanceConflictWarning is a single soft warning surfaced on the start
// response. One entry per (kind, resource) pair. Multiple warnings per kind
// are possible (e.g. two staff already supervising elsewhere).
//
// This is a new type rather than a reuse of api/iot/sessions.ConflictInfoResponse
// because that type's ConflictingDevice field is meaningless for planner-driven
// transitions (no device is involved for staff/student conflicts). Keeping the
// IoT shape unchanged lets both flows evolve independently.
type InstanceConflictWarning struct {
	Kind        string `json:"kind"`         // "room" | "staff" | "student"
	ResourceID  int64  `json:"resource_id"`  // room_id / staff_id / student_id
	Message     string `json:"message"`      // German user-facing copy
	CanOverride bool   `json:"can_override"` // always true in v1
}

// ConflictDependencies groups the repos needed by DetectStartConflicts so
// the service wiring is explicit. All fields are required; passing nil for
// any of them is a programmer error and will panic on first call — there is
// no graceful-degradation path here because the planner cannot reliably
// decide whether to warn without all three checks.
type ConflictDependencies struct {
	GroupRepo         active.GroupRepository
	SupervisorRepo    active.GroupSupervisorRepository
	VisitRepo         active.VisitRepository
	InstanceStaffRepo scheduleModel.InstanceStaffRepository
	InstanceStudents  scheduleModel.InstanceStudentRepository
}

// DetectStartConflicts runs the three sub-checks for the given planned
// instance and returns a (possibly empty) list of warnings. It mutates
// nothing and never returns an error that should block the transition —
// a DB error on one sub-check is logged and surfaces as zero warnings for
// that kind; the caller continues.
//
// Ordering is deterministic: room first, then staff (sorted by instance_staff
// row ID), then student (sorted by instance_students row ID). Tests rely on
// that ordering; do not reorder without updating expectations.
func DetectStartConflicts(
	ctx context.Context,
	deps ConflictDependencies,
	instance *scheduleModel.ActivityInstance,
	logger *slog.Logger,
) []InstanceConflictWarning {
	if logger == nil {
		logger = slog.Default()
	}
	var warnings []InstanceConflictWarning

	// 1. Room — reuse existing CheckRoomConflict (models/active/repository.go:44).
	// excludeGroupID=0 means "no exclusion": an instance is not yet bridged to
	// an active.group at start time, so any hit is a real conflict.
	has, conflicting, err := deps.GroupRepo.CheckRoomConflict(ctx, instance.RoomID, 0)
	if err != nil {
		logger.Warn("conflict detection: room check failed",
			slog.Int64("instance_id", instance.ID),
			slog.Int64("room_id", instance.RoomID),
			slog.String("error", err.Error()),
		)
	} else if has && conflicting != nil {
		warnings = append(warnings, InstanceConflictWarning{
			Kind:        ConflictKindRoom,
			ResourceID:  instance.RoomID,
			Message:     fmt.Sprintf("Raum ist bereits durch aktive Gruppe #%d belegt", conflicting.ID),
			CanOverride: true,
		})
	}

	// 2. Staff — each assigned staff member must not already be supervising
	// an active.group. FindActiveByStaffID filters on end_date IS NULL /
	// end_date > NOW() at the repo level, so a result means "still supervising".
	staffRows, err := deps.InstanceStaffRepo.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		logger.Warn("conflict detection: load instance_staff failed",
			slog.Int64("instance_id", instance.ID),
			slog.String("error", err.Error()),
		)
	}
	for _, row := range staffRows {
		// Absent rows are not candidates for supervision on this instance;
		// flagging them as "supervising elsewhere" would be misleading. Start()
		// also skips them when copying to active.group_supervisors, so the two
		// paths stay consistent.
		if row.IsAbsent {
			continue
		}
		supervisions, err := deps.SupervisorRepo.FindActiveByStaffID(ctx, row.StaffID)
		if err != nil {
			logger.Warn("conflict detection: staff supervision lookup failed",
				slog.Int64("staff_id", row.StaffID),
				slog.String("error", err.Error()),
			)
			continue
		}
		if len(supervisions) == 0 {
			continue
		}
		// Flag the first active supervision — listing them all gets noisy on
		// a staff member with multiple overlapping roles, and the frontend
		// only needs to know "there is at least one conflict" per staff_id.
		sup := supervisions[0]
		warnings = append(warnings, InstanceConflictWarning{
			Kind:        ConflictKindStaff,
			ResourceID:  row.StaffID,
			Message:     fmt.Sprintf("Mitarbeiter betreut bereits aktive Gruppe #%d", sup.GroupID),
			CanOverride: true,
		})
	}

	// 3. Student — each expected student must not already have an open visit.
	// A student mid-visit elsewhere would show up in two groups simultaneously;
	// informational only, never blocking. The check ignores students already
	// in statuses other than expected (present/absent are post-check-in states
	// that belong to other instances in this tenant's day).
	//
	// Batched via GetCurrentByStudentIDs — per-student lookups would produce
	// N queries on an instance with N expected students (20+ for a typical
	// OGS group), measurable latency inside the tenant tx.
	studentRows, err := deps.InstanceStudents.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		logger.Warn("conflict detection: load instance_students failed",
			slog.Int64("instance_id", instance.ID),
			slog.String("error", err.Error()),
		)
	}
	expectedIDs := make([]int64, 0, len(studentRows))
	for _, row := range studentRows {
		if row.Status != scheduleModel.AttendanceStatusExpected {
			continue
		}
		expectedIDs = append(expectedIDs, row.StudentID)
	}
	if len(expectedIDs) > 0 {
		visits, err := deps.VisitRepo.GetCurrentByStudentIDs(ctx, expectedIDs)
		if err != nil {
			logger.Warn("conflict detection: student current visits lookup failed",
				slog.Int64("instance_id", instance.ID),
				slog.Int("student_count", len(expectedIDs)),
				slog.String("error", err.Error()),
			)
		} else {
			// Iterate expectedIDs (not the map) so warning order stays
			// deterministic — it mirrors the instance_students insertion
			// order from materialization, which tests rely on.
			for _, sid := range expectedIDs {
				visit, ok := visits[sid]
				if !ok || visit == nil {
					continue
				}
				warnings = append(warnings, InstanceConflictWarning{
					Kind:        ConflictKindStudent,
					ResourceID:  sid,
					Message:     fmt.Sprintf("Kind hat bereits einen aktiven Aufenthalt in Gruppe #%d", visit.ActiveGroupID),
					CanOverride: true,
				})
			}
		}
	}

	return warnings
}
