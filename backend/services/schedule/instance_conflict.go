// Package schedule — instance conflict detection (WP-B9, matrix reworked in
// #2139: only PERSON double-bookings warn; rooms and pure time overlaps are
// sanctioned).
//
// Soft-warning detection fired on POST /instances/{id}/start. Both sub-checks
// are read-only and NEVER block the transition: they run inside the caller's
// tenant transaction so a roll-back discards them cleanly, and the transition
// proceeds regardless of what they find. Every warning carries
// can_override=true in v1; a future WP may introduce hard blocks.
package schedule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// Conflict kinds — stable string values exported to clients so the frontend
// can switch on kind rather than parsing German messages.
//
// There is deliberately no "room" kind anymore (#2139): parallel groups may
// share a room, so a pure room overlap is not a conflict. Only a person
// (child or staff) planned twice at the same time can conflict.
const (
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
	Kind        string `json:"kind"`         // "staff" | "student"
	ResourceID  int64  `json:"resource_id"`  // staff_id / student_id
	Message     string `json:"message"`      // German user-facing copy
	CanOverride bool   `json:"can_override"` // always true in v1

	// The fields below are set by the planning-window detection (#2139) so
	// the calendar can link the two involved blocks and persist a per-user
	// acknowledgement. Start-time warnings (planned vs. live layer) leave
	// them empty — they are transient toast content, not acknowledgeable
	// calendar state.
	Fingerprint           string `json:"fingerprint,omitempty"`
	ConflictingInstanceID int64  `json:"conflicting_instance_id,omitempty"`
	ConflictingTitle      string `json:"conflicting_title,omitempty"`
	OverlapStart          string `json:"overlap_start,omitempty"` // "HH:MM"
	OverlapEnd            string `json:"overlap_end,omitempty"`   // "HH:MM"
}

// ConflictDependencies groups the repos needed by DetectStartConflicts so
// the service wiring is explicit. All fields are required; passing nil for
// any of them is a programmer error and will panic on first call — there is
// no graceful-degradation path here because the planner cannot reliably
// decide whether to warn without all sub-checks.
type ConflictDependencies struct {
	GroupRepo         active.GroupRepository
	SupervisorRepo    active.GroupSupervisorRepository
	VisitRepo         active.VisitRepository
	InstanceRepo      scheduleModel.ActivityInstanceRepository
	InstanceStaffRepo scheduleModel.InstanceStaffRepository
	InstanceStudents  scheduleModel.InstanceStudentRepository
}

// DetectStartConflicts runs the three sub-checks for the given planned
// instance and returns a (possibly empty) list of warnings. It mutates
// nothing and never returns an error that should block the transition —
// a DB error on one sub-check is logged and surfaces as zero warnings for
// that kind; the caller continues.
//
// Ordering is deterministic: staff first (sorted by instance_staff row ID),
// then student (sorted by instance_students row ID). Tests rely on that
// ordering; do not reorder without updating expectations.
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

	// A room shared by two groups is not a conflict (#2139) — the old
	// CheckRoomConflict sub-check is gone. Only people can be double-booked.

	// 1. Staff — each assigned staff member must not already be supervising
	// an active.group IN A DIFFERENT ROOM. Supervising a parallel group in
	// the SAME room is a sanctioned pattern (one Betreuungskraft, several
	// parallel groups, one physical room — #2139), so those supervisions are
	// skipped. The staff member's room in the RUNNING session is resolved via
	// activeSupervisionRoom (the bridged instance's per-row multi-room
	// override, not just the group's primary room — see #2151 review). When
	// that room cannot be resolved the warning stays: an undetermined room
	// counts as "not certainly the same room".
	// FindActiveByStaffID filters on end_date IS NULL / end_date > NOW() at
	// the repo level, so a result means "still supervising".
	staffRows, err := deps.InstanceStaffRepo.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		logger.Warn("conflict detection: load instance_staff failed",
			slog.Int64("instance_id", instance.ID),
			slog.String("error", err.Error()),
		)
	}
	staffConflicts := loadActiveStaffConflicts(ctx, deps, staffRows, logger)
	for _, row := range staffRows {
		// Absent rows are not candidates for supervision on this instance;
		// flagging them as "supervising elsewhere" would be misleading. Start()
		// also skips them when copying to active.group_supervisors, so the two
		// paths stay consistent.
		if row.IsAbsent {
			continue
		}
		supervisions := staffConflicts.supervisions[row.StaffID]
		// The staff member's effective room on THIS instance: the per-row
		// multi-room override wins over the instance's primary room.
		effectiveRoom := instance.RoomID
		if row.RoomID != nil {
			effectiveRoom = *row.RoomID
		}
		// Flag the first supervision that is not certainly in the same room —
		// listing them all gets noisy on a staff member with multiple
		// overlapping roles, and the frontend only needs to know "there is at
		// least one conflict" per staff_id.
		for _, sup := range supervisions {
			group := staffConflicts.groups[sup.GroupID]
			if group == nil {
				// Room not determinable → not certainly the same room → warn.
				warnings = append(warnings, InstanceConflictWarning{
					Kind:        ConflictKindStaff,
					ResourceID:  row.StaffID,
					Message:     fmt.Sprintf("Mitarbeiter betreut bereits aktive Gruppe #%d (Raum nicht eindeutig bestimmbar)", sup.GroupID),
					CanOverride: true,
				})
				break
			}
			supervisionRoom, roomKnown := staffConflicts.room(group, row.StaffID)
			if roomKnown && supervisionRoom == effectiveRoom {
				continue // same concrete room — sanctioned parallel supervision
			}
			message := fmt.Sprintf("Mitarbeiter betreut bereits aktive Gruppe #%d in einem anderen Raum", group.ID)
			if !roomKnown {
				message = fmt.Sprintf("Mitarbeiter betreut bereits aktive Gruppe #%d (Raum nicht eindeutig bestimmbar)", group.ID)
			}
			warnings = append(warnings, InstanceConflictWarning{
				Kind:        ConflictKindStaff,
				ResourceID:  row.StaffID,
				Message:     message,
				CanOverride: true,
			})
			break
		}
	}

	// 2. Student — each expected student must not already have an open visit.
	// A student mid-visit elsewhere would show up in two groups simultaneously
	// — always a conflict, regardless of rooms (double attendance, double
	// Betreuungsschlüssel). Informational only, never blocking. The check
	// ignores students already
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

type activeStaffConflicts struct {
	supervisions map[int64][]*active.GroupSupervisor
	groups       map[int64]*active.Group
	instances    map[int64]*scheduleModel.ActivityInstance
	staffRows    map[int64][]*scheduleModel.InstanceStaff
	roomsLoaded  bool
}

func loadActiveStaffConflicts(
	ctx context.Context,
	deps ConflictDependencies,
	assigned []*scheduleModel.InstanceStaff,
	logger *slog.Logger,
) activeStaffConflicts {
	result := newActiveStaffConflicts()
	staffIDs := distinctPresentStaffIDs(assigned)
	if len(staffIDs) == 0 {
		return result
	}
	options := modelBase.NewQueryOptions()
	options.Filter = modelBase.NewFilter().Equal("active_only", true).In("staff_id", int64FilterArgs(staffIDs)...)
	supervisions, err := deps.SupervisorRepo.List(ctx, options)
	if err != nil {
		logger.Warn("conflict detection: staff supervision batch lookup failed", slog.String("error", err.Error()))
		return result
	}
	groupIDs := indexSupervisions(result.supervisions, supervisions)
	if len(groupIDs) == 0 {
		return result
	}
	return loadActiveStaffConflictRooms(ctx, deps, result, groupIDs, logger)
}

func newActiveStaffConflicts() activeStaffConflicts {
	return activeStaffConflicts{
		supervisions: make(map[int64][]*active.GroupSupervisor),
		groups:       make(map[int64]*active.Group),
		instances:    make(map[int64]*scheduleModel.ActivityInstance),
		staffRows:    make(map[int64][]*scheduleModel.InstanceStaff),
	}
}

func loadActiveStaffConflictRooms(
	ctx context.Context,
	deps ConflictDependencies,
	result activeStaffConflicts,
	groupIDs []int64,
	logger *slog.Logger,
) activeStaffConflicts {
	groups, err := deps.GroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		logger.Warn("conflict detection: supervised group batch lookup failed", slog.String("error", err.Error()))
		return result
	}
	result.groups = groups
	if len(groupIDs) == 0 {
		result.roomsLoaded = true
		return result
	}
	instances, err := legacyList[*scheduleModel.ActivityInstance](ctx, deps.InstanceRepo, &modelBase.QueryOptions{
		Filter: modelBase.NewFilter().In("active_group_id", int64FilterArgs(groupIDs)...),
	})
	if err != nil {
		logger.Warn("conflict detection: bridged instance batch lookup failed", slog.String("error", err.Error()))
		return result
	}
	result.roomsLoaded = true
	for _, instance := range instances {
		if instance.ActiveGroupID != nil {
			result.instances[*instance.ActiveGroupID] = instance
		}
	}
	rows, err := deps.InstanceStaffRepo.FindByInstanceIDs(ctx, activityInstanceIDs(instances))
	if err != nil {
		logger.Warn("conflict detection: bridged instance_staff batch lookup failed", slog.String("error", err.Error()))
		return result
	}
	result.staffRows = indexInstanceStaffRows(rows)
	return result
}

func distinctPresentStaffIDs(rows []*scheduleModel.InstanceStaff) []int64 {
	ids := make([]int64, 0, len(rows))
	seen := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if row.IsAbsent || seen[row.StaffID] {
			continue
		}
		seen[row.StaffID] = true
		ids = append(ids, row.StaffID)
	}
	return ids
}

func indexSupervisions(byStaff map[int64][]*active.GroupSupervisor, rows []*active.GroupSupervisor) []int64 {
	groupIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]bool, len(rows))
	for _, row := range rows {
		byStaff[row.StaffID] = append(byStaff[row.StaffID], row)
		if !seen[row.GroupID] {
			seen[row.GroupID] = true
			groupIDs = append(groupIDs, row.GroupID)
		}
	}
	return groupIDs
}

// room resolves the room a staff member is actually bound to
// in the given RUNNING group. active.groups stores only the session's primary
// room, so for a group bridged to a timetable instance the staff member's
// per-row multi-room override on that instance's instance_staff rows wins —
// comparing against the group's primary room alone would suppress a real
// double-booking (staff actually in a secondary room) or warn although the
// staff member sits in the same room (#2151 review).
//
// A spontaneous group without a bridged instance has exactly one room, so the
// group's room is authoritative. ok=false means the room could not be
// determined (lookup failure, or a bridged instance whose roster does not
// contain the staff member) — callers must then KEEP the warning, because an
// undetermined room is "not certainly the same room".
func (c activeStaffConflicts) room(group *active.Group, staffID int64) (roomID int64, ok bool) {
	if !c.roomsLoaded {
		return 0, false
	}
	instance := c.instances[group.ID]
	if instance == nil {
		return group.RoomID, true
	}
	for _, row := range c.staffRows[instance.ID] {
		if row.StaffID == staffID {
			return effectiveStaffRoom(instance, row), true
		}
	}
	// Supervisor without a matching roster row on the bridged instance —
	// anomalous (Start and the deviation flows keep both in sync), so the
	// effective room is unknown.
	return 0, false
}

// germanDateLayout renders calendar dates as dd.mm.yyyy in user-facing copy.
const germanDateLayout = "02.01.2006"

// PlannedConflictQuery is the input of DetectPlannedConflicts: a hypothetical
// slot (date + wall-clock window) plus the resources the caller wants checked.
// At least one of RoomID / StaffIDs / StudentIDs must be set — the HTTP
// handler enforces that before calling.
//
// RoomID no longer produces warnings of its own (#2139: shared rooms are
// sanctioned). It still matters as INPUT: staff double-planning is only a
// conflict when the two slots are not certainly in the same room, so the
// hypothetical slot's room feeds that comparison. A nil RoomID means the
// slot's room is undetermined — staff overlaps then always warn.
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
	Kind                  string `json:"kind"` // "staff" | "student"
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
// Ordering is deterministic: staff warnings first (by instance ID, then
// assignment row ID), then student (by instance ID, then student ID).
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
// normalising both sides through timezone.NormalizeWallClock pins the comparison to
// pure HH:MM:SS.
func loadOverlappingInstances(
	ctx context.Context,
	deps PlannedConflictDependencies,
	q PlannedConflictQuery,
	logger *slog.Logger,
) []*scheduleModel.ActivityInstance {
	queryDate := scheduleModel.Date(q.Date)
	instances, err := deps.InstanceRepo.FindByTenantAndDateRange(ctx, queryDate, queryDate)
	if err != nil {
		logger.Warn("planned conflict detection: load day instances failed",
			slog.String("date", q.Date.String()),
			slog.String("error", err.Error()),
		)
		return nil
	}

	qStart := timezone.NormalizeWallClock(q.StartTime)
	qEnd := timezone.NormalizeWallClock(q.EndTime)

	out := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusPlanned &&
			inst.Status != scheduleModel.InstanceStatusActive {
			continue // completed/cancelled are history, not conflicts
		}
		if q.ExcludeInstanceID != nil && inst.ID == *q.ExcludeInstanceID {
			continue
		}
		iStart := timezone.NormalizeWallClock(inst.StartTime)
		iEnd := timezone.NormalizeWallClock(inst.EndTime)
		if qStart.Before(iEnd) && iStart.Before(qEnd) {
			out = append(out, inst)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// plannedStaffConflicts bulk-loads the overlapping instances' staff
// assignments and intersects them with the queried staff. IsAbsent rows are
// skipped — staff marked out for an instance are not actually bound by it.
// An overlap in the SAME concrete room is skipped too (#2139): one
// Betreuungskraft supervising several parallel groups in one room is a
// sanctioned pattern. The comparison uses effective rooms (per-row multi-room
// override, else the instance's primary room) against q.RoomID; a nil q.RoomID
// means the hypothetical slot's room is undetermined, so the warning stays.
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
		roomNote := "Raum noch nicht festgelegt"
		if q.RoomID != nil {
			effectiveRoom := inst.RoomID
			if row.RoomID != nil {
				effectiveRoom = *row.RoomID
			}
			if effectiveRoom == *q.RoomID {
				continue // same concrete room — sanctioned parallel supervision
			}
			roomNote = "anderer Raum"
		}
		warnings = append(warnings, PlannedConflictWarning{
			Kind:       ConflictKindStaff,
			ResourceID: row.StaffID,
			Message: fmt.Sprintf("„Personal“ ist am %s von %s–%s bereits bei „%s“ eingeplant (%s).",
				q.Date.Format(germanDateLayout),
				timezone.NormalizeWallClock(inst.StartTime).Format("15:04"),
				timezone.NormalizeWallClock(inst.EndTime).Format("15:04"),
				inst.Title,
				roomNote,
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
				timezone.NormalizeWallClock(inst.StartTime).Format("15:04"),
				timezone.NormalizeWallClock(inst.EndTime).Format("15:04"),
				inst.Title,
			),
			ConflictingInstanceID: inst.ID,
			ConflictingTitle:      inst.Title,
		})
	}
	return warnings
}

// WindowConflictInput is one instance plus its assignment rows, the unit the
// calendar's window-wide conflict detection (#2139) works on. The caller loads
// the rows (the instance list already fetches them for enrichment); the
// detection itself is a pure function so every matrix row is unit-testable
// without a database.
type WindowConflictInput struct {
	Instance *scheduleModel.ActivityInstance
	Staff    []*scheduleModel.InstanceStaff
	Students []*scheduleModel.InstanceStudent
}

// DetectWindowConflicts computes person double-bookings between the given
// planned/active instances, keyed by instance ID. It implements the #2139
// conflict matrix for the calendar view:
//
//   - pure time overlap or a shared room alone: NO warning
//   - same child in two overlapping instances: warning, regardless of rooms
//   - same (non-absent) staff in two overlapping instances: warning only when
//     the effective rooms differ (per-row multi-room override, else the
//     instance's primary room) — same concrete room_id is sanctioned
//   - touching edges (endA == startB): NO warning (half-open comparison)
//   - completed/cancelled instances: never conflict
//
// Every warning appears on BOTH involved instances with the SAME fingerprint,
// so the calendar can deduplicate by fingerprint and a per-user
// acknowledgement hides the pair at once.
func DetectWindowConflicts(inputs []WindowConflictInput) map[int64][]InstanceConflictWarning {
	relevant := make([]WindowConflictInput, 0, len(inputs))
	for _, in := range inputs {
		if in.Instance == nil {
			continue
		}
		if in.Instance.Status != scheduleModel.InstanceStatusPlanned &&
			in.Instance.Status != scheduleModel.InstanceStatusActive {
			continue
		}
		relevant = append(relevant, in)
	}
	sort.Slice(relevant, func(i, j int) bool {
		if relevant[i].Instance.Date != relevant[j].Instance.Date {
			return relevant[i].Instance.Date.Before(relevant[j].Instance.Date)
		}
		return relevant[i].Instance.ID < relevant[j].Instance.ID
	})

	out := make(map[int64][]InstanceConflictWarning)
	for i := 0; i < len(relevant); i++ {
		for j := i + 1; j < len(relevant); j++ {
			a, b := relevant[i], relevant[j]
			if a.Instance.Date != b.Instance.Date {
				break // sorted by date — no later same-date partner exists
			}
			aStart := timezone.NormalizeWallClock(a.Instance.StartTime)
			aEnd := timezone.NormalizeWallClock(a.Instance.EndTime)
			bStart := timezone.NormalizeWallClock(b.Instance.StartTime)
			bEnd := timezone.NormalizeWallClock(b.Instance.EndTime)
			// Half-open: touching edges are not a conflict.
			if !aStart.Before(bEnd) || !bStart.Before(aEnd) {
				continue
			}
			oStart, oEnd := aStart, aEnd
			if bStart.After(oStart) {
				oStart = bStart
			}
			if bEnd.Before(oEnd) {
				oEnd = bEnd
			}
			appendPairStaffConflicts(out, a, b, oStart, oEnd)
			appendPairStudentConflicts(out, a, b, oStart, oEnd)
		}
	}
	return out
}

// effectiveStaffRoom resolves the room a staff assignment actually binds: the
// per-row multi-room override when set, else the instance's primary room.
func effectiveStaffRoom(inst *scheduleModel.ActivityInstance, row *scheduleModel.InstanceStaff) int64 {
	if row.RoomID != nil {
		return *row.RoomID
	}
	return inst.RoomID
}

// appendPairStaffConflicts emits mirrored staff warnings for one overlapping
// instance pair. IsAbsent rows are skipped on both sides — staff marked out
// for an instance are not bound by it.
func appendPairStaffConflicts(out map[int64][]InstanceConflictWarning, a, b WindowConflictInput, oStart, oEnd time.Time) {
	bByStaff := make(map[int64]*scheduleModel.InstanceStaff, len(b.Staff))
	for _, row := range b.Staff {
		if row.IsAbsent {
			continue
		}
		bByStaff[row.StaffID] = row
	}
	for _, aRow := range a.Staff {
		if aRow.IsAbsent {
			continue
		}
		bRow, ok := bByStaff[aRow.StaffID]
		if !ok {
			continue
		}
		roomA := effectiveStaffRoom(a.Instance, aRow)
		roomB := effectiveStaffRoom(b.Instance, bRow)
		if roomA == roomB {
			continue // same concrete room — sanctioned parallel supervision
		}
		fp := conflictFingerprint(ConflictKindStaff, aRow.StaffID, timezone.Date(a.Instance.Date),
			a.Instance.ID, b.Instance.ID, oStart, oEnd, roomA, roomB)
		appendMirroredWarnings(out, a.Instance, b.Instance, InstanceConflictWarning{
			Kind:        ConflictKindStaff,
			ResourceID:  aRow.StaffID,
			CanOverride: true,
			Fingerprint: fp,
		}, oStart, oEnd, "Personal ist von %s–%s auch bei „%s“ eingeplant (anderer Raum).")
	}
}

// appendPairStudentConflicts emits mirrored student warnings for one
// overlapping instance pair. Rows in status expected or present count — both
// mean the plan claims the child for that window; absent rows do not.
func appendPairStudentConflicts(out map[int64][]InstanceConflictWarning, a, b WindowConflictInput, oStart, oEnd time.Time) {
	claims := func(status string) bool {
		return status == scheduleModel.AttendanceStatusExpected ||
			status == scheduleModel.AttendanceStatusPresent
	}
	bStudents := make(map[int64]bool, len(b.Students))
	for _, row := range b.Students {
		if claims(row.Status) {
			bStudents[row.StudentID] = true
		}
	}
	for _, aRow := range a.Students {
		if !claims(aRow.Status) || !bStudents[aRow.StudentID] {
			continue
		}
		// Rooms are irrelevant to a child double-booking (the conflict exists
		// either way), so the fingerprint pins them to zero: moving one block
		// to another room must NOT resurface an acknowledged child conflict.
		fp := conflictFingerprint(ConflictKindStudent, aRow.StudentID, timezone.Date(a.Instance.Date),
			a.Instance.ID, b.Instance.ID, oStart, oEnd, 0, 0)
		appendMirroredWarnings(out, a.Instance, b.Instance, InstanceConflictWarning{
			Kind:        ConflictKindStudent,
			ResourceID:  aRow.StudentID,
			CanOverride: true,
			Fingerprint: fp,
		}, oStart, oEnd, "Kind ist von %s–%s auch bei „%s“ eingeplant.")
	}
}

// appendMirroredWarnings attaches the warning to both involved instances,
// each naming the OTHER instance as the conflicting one. The message format
// takes (overlap start, overlap end, other title).
func appendMirroredWarnings(
	out map[int64][]InstanceConflictWarning,
	a, b *scheduleModel.ActivityInstance,
	warning InstanceConflictWarning,
	oStart, oEnd time.Time,
	messageFormat string,
) {
	startStr := oStart.Format("15:04")
	endStr := oEnd.Format("15:04")
	warning.OverlapStart = startStr
	warning.OverlapEnd = endStr

	forA := warning
	forA.ConflictingInstanceID = b.ID
	forA.ConflictingTitle = b.Title
	forA.Message = fmt.Sprintf(messageFormat, startStr, endStr, b.Title)
	out[a.ID] = append(out[a.ID], forA)

	forB := warning
	forB.ConflictingInstanceID = a.ID
	forB.ConflictingTitle = a.Title
	forB.Message = fmt.Sprintf(messageFormat, startStr, endStr, a.Title)
	out[b.ID] = append(out[b.ID], forB)
}

// conflictFingerprint derives the stable identity of one concrete conflict:
// kind, person, both instances (order-normalized), date, overlap window, and
// the two effective rooms (zero for student conflicts, where rooms do not
// define the conflict). Any change to person, instances, time, or a relevant
// room yields a new fingerprint, so an acknowledgement stops matching and the
// warning resurfaces (#2139).
func conflictFingerprint(
	kind string,
	resourceID int64,
	date timezone.Date,
	instA, instB int64,
	oStart, oEnd time.Time,
	roomA, roomB int64,
) string {
	if instB < instA {
		instA, instB = instB, instA
		roomA, roomB = roomB, roomA
	}
	payload := fmt.Sprintf("v1|%s|%d|%s|%d|%d|%s|%s|%d|%d",
		kind, resourceID, date.String(), instA, instB,
		oStart.Format("15:04"), oEnd.Format("15:04"), roomA, roomB)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16])
}
