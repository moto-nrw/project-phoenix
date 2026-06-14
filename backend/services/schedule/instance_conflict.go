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
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

// germanDateLayout renders calendar dates as dd.mm.yyyy in user-facing copy.
const germanDateLayout = "02.01.2006"

// PlannedConflictQuery is the input of DetectPlannedConflicts: a hypothetical
// slot (date + wall-clock window) plus the resources the caller wants checked.
// At least one of RoomID / StaffIDs / StudentIDs must be set — the HTTP
// handler enforces that before calling.
type PlannedConflictQuery struct {
	Date              timezone.Date
	StartTime         time.Time // wall-clock, any year anchor
	EndTime           time.Time // wall-clock, any year anchor
	RoomID            *int64
	StaffIDs          []int64
	StudentIDs        []int64
	ExcludeInstanceID *int64 // instance being edited — never conflicts with itself
}

// PlannedConflictWarning is one advisory hit of the planning-time conflict
// probe (GET /api/timetable/conflicts). Unlike InstanceConflictWarning it
// names the conflicting instance so the planner UI can link to it; there is
// no can_override flag because the probe never blocks anything.
type PlannedConflictWarning struct {
	Kind                  string `json:"kind"` // "room" | "staff" | "student"
	ResourceID            int64  `json:"resource_id"`
	Message               string `json:"message"` // German user-facing copy
	ConflictingInstanceID int64  `json:"conflicting_instance_id"`
	ConflictingTitle      string `json:"conflicting_title"`
}

// PlannedConflictDependencies groups the repos DetectPlannedConflicts needs.
// All fields are required.
type PlannedConflictDependencies struct {
	InstanceRepo      scheduleModel.ActivityInstanceRepository
	InstanceStaffRepo scheduleModel.InstanceStaffRepository
	InstanceStudents  scheduleModel.InstanceStudentRepository
}

// DetectPlannedConflicts checks a hypothetical slot against the day's already
// planned/active instances and returns advisory warnings. Sibling of
// DetectStartConflicts, but planning-time: it compares against the timetable
// (schedule.activity_instances), not the live layer (active.*).
//
// Error handling mirrors DetectStartConflicts: a failing sub-check degrades
// to zero warnings of that kind plus a slog warning — the probe must never
// turn into a 500 for the planner UI.
//
// Ordering is deterministic: room warnings first (by conflicting instance
// ID), then staff (by instance ID, then assignment row ID), then student (by
// instance ID, then student ID).
func DetectPlannedConflicts(
	ctx context.Context,
	deps PlannedConflictDependencies,
	q PlannedConflictQuery,
	logger *slog.Logger,
) []PlannedConflictWarning {
	if logger == nil {
		logger = slog.Default()
	}

	overlapping := loadOverlappingInstances(ctx, deps, q, logger)
	if len(overlapping) == 0 {
		return []PlannedConflictWarning{}
	}

	byID := make(map[int64]*scheduleModel.ActivityInstance, len(overlapping))
	ids := make([]int64, 0, len(overlapping))
	for _, inst := range overlapping {
		byID[inst.ID] = inst
		ids = append(ids, inst.ID)
	}

	warnings := make([]PlannedConflictWarning, 0)
	warnings = append(warnings, plannedRoomConflicts(q, overlapping)...)
	warnings = append(warnings, plannedStaffConflicts(ctx, deps, q, ids, byID, logger)...)
	warnings = append(warnings, plannedStudentConflicts(ctx, deps, q, ids, byID, logger)...)
	return warnings
}

// loadOverlappingInstances fetches the day's instances and keeps the
// planned/active ones whose wall-clock window overlaps the queried slot
// (startA < endB && startB < endA — touching edges are NOT a conflict).
// Result is sorted by instance ID for deterministic warning order.
//
// Instances scan their TIME columns with arbitrary, driver-chosen year
// anchors (see the WallClock caveat in api/timetable/exception_conflicts.go);
// normalising both sides through timezone.WallClock pins the comparison to
// pure HH:MM:SS.
func loadOverlappingInstances(
	ctx context.Context,
	deps PlannedConflictDependencies,
	q PlannedConflictQuery,
	logger *slog.Logger,
) []*scheduleModel.ActivityInstance {
	instances, err := deps.InstanceRepo.FindByTenantAndDateRange(ctx, q.Date, q.Date)
	if err != nil {
		logger.Warn("planned conflict detection: load day instances failed",
			slog.String("date", q.Date.String()),
			slog.String("error", err.Error()),
		)
		return nil
	}

	qStart := timezone.WallClock(q.StartTime)
	qEnd := timezone.WallClock(q.EndTime)

	out := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusPlanned &&
			inst.Status != scheduleModel.InstanceStatusActive {
			continue // completed/cancelled are history, not conflicts
		}
		if q.ExcludeInstanceID != nil && inst.ID == *q.ExcludeInstanceID {
			continue
		}
		iStart := timezone.WallClock(inst.StartTime)
		iEnd := timezone.WallClock(inst.EndTime)
		if qStart.Before(iEnd) && iStart.Before(qEnd) {
			out = append(out, inst)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// plannedRoomConflicts flags every overlapping instance occupying the
// queried room.
func plannedRoomConflicts(q PlannedConflictQuery, overlapping []*scheduleModel.ActivityInstance) []PlannedConflictWarning {
	if q.RoomID == nil {
		return nil
	}
	var warnings []PlannedConflictWarning
	for _, inst := range overlapping {
		if inst.RoomID != *q.RoomID {
			continue
		}
		warnings = append(warnings, PlannedConflictWarning{
			Kind:       ConflictKindRoom,
			ResourceID: *q.RoomID,
			Message: fmt.Sprintf("Raum ist am %s von %s–%s bereits durch „%s“ belegt.",
				q.Date.Format(germanDateLayout),
				timezone.WallClock(inst.StartTime).Format("15:04"),
				timezone.WallClock(inst.EndTime).Format("15:04"),
				inst.Title,
			),
			ConflictingInstanceID: inst.ID,
			ConflictingTitle:      inst.Title,
		})
	}
	return warnings
}

// plannedStaffConflicts bulk-loads the overlapping instances' staff
// assignments and intersects them with the queried staff. IsAbsent rows are
// skipped — staff marked out for an instance are not actually bound by it.
// Staff names are not cheaply loadable here (users.staff carries only the
// person FK), so the copy uses the generic "Personal" subject and names the
// conflicting instance instead — no N+1 lookups.
func plannedStaffConflicts(
	ctx context.Context,
	deps PlannedConflictDependencies,
	q PlannedConflictQuery,
	overlappingIDs []int64,
	byID map[int64]*scheduleModel.ActivityInstance,
	logger *slog.Logger,
) []PlannedConflictWarning {
	if len(q.StaffIDs) == 0 {
		return nil
	}
	rows, err := deps.InstanceStaffRepo.FindByInstanceIDs(ctx, overlappingIDs)
	if err != nil {
		logger.Warn("planned conflict detection: load instance_staff failed",
			slog.String("date", q.Date.String()),
			slog.String("error", err.Error()),
		)
		return nil
	}
	requested := make(map[int64]struct{}, len(q.StaffIDs))
	for _, id := range q.StaffIDs {
		requested[id] = struct{}{}
	}
	var warnings []PlannedConflictWarning
	for _, row := range rows {
		if row.IsAbsent {
			continue
		}
		if _, ok := requested[row.StaffID]; !ok {
			continue
		}
		inst, ok := byID[row.InstanceID]
		if !ok {
			continue
		}
		warnings = append(warnings, PlannedConflictWarning{
			Kind:       ConflictKindStaff,
			ResourceID: row.StaffID,
			Message: fmt.Sprintf("„Personal“ ist am %s von %s–%s bereits bei „%s“ eingeplant.",
				q.Date.Format(germanDateLayout),
				timezone.WallClock(inst.StartTime).Format("15:04"),
				timezone.WallClock(inst.EndTime).Format("15:04"),
				inst.Title,
			),
			ConflictingInstanceID: inst.ID,
			ConflictingTitle:      inst.Title,
		})
	}
	return warnings
}

// plannedStudentConflicts intersects the queried students with the students
// still expected on overlapping instances (status='expected'; checked-in or
// absent students belong to that other instance's history, not the plan).
func plannedStudentConflicts(
	ctx context.Context,
	deps PlannedConflictDependencies,
	q PlannedConflictQuery,
	overlappingIDs []int64,
	byID map[int64]*scheduleModel.ActivityInstance,
	logger *slog.Logger,
) []PlannedConflictWarning {
	if len(q.StudentIDs) == 0 {
		return nil
	}
	rows, err := deps.InstanceStudents.FindExpectedByInstanceIDs(ctx, overlappingIDs)
	if err != nil {
		logger.Warn("planned conflict detection: load instance_students failed",
			slog.String("date", q.Date.String()),
			slog.String("error", err.Error()),
		)
		return nil
	}
	requested := make(map[int64]struct{}, len(q.StudentIDs))
	for _, id := range q.StudentIDs {
		requested[id] = struct{}{}
	}
	var warnings []PlannedConflictWarning
	for _, row := range rows {
		if _, ok := requested[row.StudentID]; !ok {
			continue
		}
		inst, ok := byID[row.InstanceID]
		if !ok {
			continue
		}
		warnings = append(warnings, PlannedConflictWarning{
			Kind:       ConflictKindStudent,
			ResourceID: row.StudentID,
			Message: fmt.Sprintf("Kind ist am %s von %s–%s bereits bei „%s“ eingeplant.",
				q.Date.Format(germanDateLayout),
				timezone.WallClock(inst.StartTime).Format("15:04"),
				timezone.WallClock(inst.EndTime).Format("15:04"),
				inst.Title,
			),
			ConflictingInstanceID: inst.ID,
			ConflictingTitle:      inst.Title,
		})
	}
	return warnings
}
