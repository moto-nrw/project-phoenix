package compose

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Start conflicts (WP-B9, matrix reworked in #2139: only person
// double-bookings warn; rooms and pure time overlaps are sanctioned). The
// check fires on POST /instances/{id}/start and on the auto-start tick. It is
// read-only and never blocks the transition by itself: it runs inside the
// caller's tenant transaction, so a roll-back discards it cleanly. Every
// warning carries can_override=true in v1.
//
// Ordering is deterministic: staff first (in instance_staff row order), then
// students (in instance_students row order). Tests rely on that ordering.

// DetectStartConflicts checks the block's staff and expected students against
// the live layer. Failures to load the expected students or their current
// presence are returned; staff lookups degrade to fewer warnings.
func (d *conflictDetection) DetectStartConflicts(ctx context.Context, subject timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
	// A room shared by two groups is not a conflict (#2139); only people can
	// be double-booked.
	warnings := d.staffStartConflicts(ctx, subject)
	studentWarnings, err := d.studentStartConflicts(ctx, subject.InstanceID)
	if err != nil {
		return nil, err
	}
	return append(warnings, studentWarnings...), nil
}

// staffStartConflicts flags each assigned staff member who already
// supervises an active group in a different room. Supervising a parallel
// group in the same room is a sanctioned pattern (one Betreuungskraft,
// several parallel groups, one physical room — #2139), so those supervisions
// are skipped. The staff member's room in the running session is resolved via
// activeStaffConflicts.room (the bridged instance's per-row multi-room
// override, not only the group's primary room — #2151 review). When that room
// cannot be resolved the warning stays: an undetermined room counts as "not
// certainly the same room".
func (d *conflictDetection) staffStartConflicts(ctx context.Context, subject timetable.StartConflictSubject) []timetable.InstanceConflictWarning {
	staffRows, err := d.deps.InstanceStaff.FindByInstanceID(ctx, subject.InstanceID)
	if err != nil {
		d.logger.Warn("conflict detection: load instance_staff failed",
			slog.Int64("instance_id", subject.InstanceID),
			slog.String("error", err.Error()),
		)
	}
	conflicts := d.loadActiveStaffConflicts(ctx, staffRows)
	var warnings []timetable.InstanceConflictWarning
	for _, row := range staffRows {
		// Absent rows are not candidates for supervision on this block;
		// flagging them as "supervising elsewhere" would be misleading. Start
		// also skips them when copying to the session supervisors.
		if row.IsAbsent {
			continue
		}
		if warning, ok := conflicts.staffWarning(row, effectiveRoom(subject.RoomID, row.RoomID)); ok {
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

// staffWarning flags the first supervision that is not certainly in the same
// room — listing them all gets noisy on a staff member with several
// overlapping roles, and the planner only needs "there is at least one
// conflict" per staff_id.
func (c activeStaffConflicts) staffWarning(row *scheduleModels.InstanceStaff, room int64) (timetable.InstanceConflictWarning, bool) {
	for _, supervision := range c.supervisions[row.StaffID] {
		group := c.groups[supervision.GroupID]
		if group == nil {
			// Room not determinable → not certainly the same room → warn.
			return staffStartWarning(row.StaffID, fmt.Sprintf("Mitarbeiter betreut bereits aktive Gruppe #%d (Raum nicht eindeutig bestimmbar)", supervision.GroupID)), true
		}
		supervisionRoom, roomKnown := c.room(group, row.StaffID)
		if roomKnown && supervisionRoom == room {
			continue // same concrete room — sanctioned parallel supervision
		}
		message := fmt.Sprintf("Mitarbeiter betreut bereits aktive Gruppe #%d in einem anderen Raum", group.ID)
		if !roomKnown {
			message = fmt.Sprintf("Mitarbeiter betreut bereits aktive Gruppe #%d (Raum nicht eindeutig bestimmbar)", group.ID)
		}
		return staffStartWarning(row.StaffID, message), true
	}
	return timetable.InstanceConflictWarning{}, false
}

func staffStartWarning(staffID int64, message string) timetable.InstanceConflictWarning {
	return timetable.InstanceConflictWarning{
		Kind:        timetable.ConflictKindStaff,
		ResourceID:  staffID,
		Message:     message,
		CanOverride: true,
	}
}

// studentStartConflicts flags each expected student with an open visit. A
// student mid-visit elsewhere would show up in two groups at once — always a
// conflict, regardless of rooms (double attendance, double
// Betreuungsschlüssel). Students in other statuses belong to other blocks'
// history. The visits are read in one batch: per-student lookups would issue
// N queries inside the tenant transaction.
func (d *conflictDetection) studentStartConflicts(ctx context.Context, instanceID int64) ([]timetable.InstanceConflictWarning, error) {
	studentRows, err := d.deps.InstanceStudents.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}
	expectedIDs := make([]int64, 0, len(studentRows))
	for _, row := range studentRows {
		if row.Status == scheduleModels.AttendanceStatusExpected {
			expectedIDs = append(expectedIDs, row.StudentID)
		}
	}
	if len(expectedIDs) == 0 {
		return nil, nil
	}
	rows, err := d.deps.Presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: expectedIDs, OpenOnly: true, NewestFirst: true, StudentOrder: true})
	if err != nil {
		return nil, fmt.Errorf("load student presence: %w", err)
	}
	visits := make(map[int64]studentpresence.Visit, len(rows))
	for _, row := range rows {
		if _, found := visits[row.StudentID]; !found {
			visits[row.StudentID] = row
		}
	}
	// Iterate expectedIDs (not the map) so the order mirrors the
	// instance_students insertion order from materialization.
	var warnings []timetable.InstanceConflictWarning
	for _, studentID := range expectedIDs {
		visit, ok := visits[studentID]
		if !ok {
			continue
		}
		warnings = append(warnings, timetable.InstanceConflictWarning{
			Kind:        timetable.ConflictKindStudent,
			ResourceID:  studentID,
			Message:     fmt.Sprintf("Kind hat bereits einen aktiven Aufenthalt in Gruppe #%d", visit.ActiveGroupID),
			CanOverride: true,
		})
	}
	return warnings, nil
}

// effectiveRoom is the room a staff assignment binds: the per-row multi-room
// override when set, else the block's primary room.
func effectiveRoom(blockRoom int64, override *int64) int64 {
	if override != nil {
		return *override
	}
	return blockRoom
}

type activeStaffConflicts struct {
	supervisions map[int64][]studentpresence.GroupSupervision
	groups       map[int64]*studentpresence.SessionDetail
	instances    map[int64]*scheduleModels.ActivityInstance
	staffRows    map[int64][]*scheduleModels.InstanceStaff
	roomsLoaded  bool
}

func newActiveStaffConflicts() activeStaffConflicts {
	return activeStaffConflicts{
		supervisions: make(map[int64][]studentpresence.GroupSupervision),
		groups:       make(map[int64]*studentpresence.SessionDetail),
		instances:    make(map[int64]*scheduleModels.ActivityInstance),
		staffRows:    make(map[int64][]*scheduleModels.InstanceStaff),
	}
}

// loadActiveStaffConflicts reads today's supervisions of the present staff
// and the rooms of the groups they supervise. The supervision filter keeps
// only supervisions still running today, so a result means "still
// supervising".
func (d *conflictDetection) loadActiveStaffConflicts(ctx context.Context, assigned []*scheduleModels.InstanceStaff) activeStaffConflicts {
	result := newActiveStaffConflicts()
	staffIDs := distinctPresentStaffIDs(assigned)
	if len(staffIDs) == 0 {
		return result
	}
	day := timezone.TodayDate().String()
	supervisions, err := d.deps.Presence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{ActiveOn: &day, StaffIDs: staffIDs})
	if err != nil {
		d.logger.Warn("conflict detection: staff supervision batch lookup failed", slog.String("error", err.Error()))
		return result
	}
	groupIDs := indexSupervisions(result.supervisions, supervisions)
	if len(groupIDs) == 0 {
		return result
	}
	return d.loadActiveStaffConflictRooms(ctx, result, groupIDs)
}

// activeGroupInstanceLister is the retained repository's options read the
// bridged-instance lookup needs; the instance repository serves it beside
// ConflictInstances.
type activeGroupInstanceLister interface {
	List(ctx context.Context, options *modelBase.QueryOptions) ([]*scheduleModels.ActivityInstance, error)
}

func (d *conflictDetection) loadActiveStaffConflictRooms(ctx context.Context, result activeStaffConflicts, groupIDs []int64) activeStaffConflicts {
	groups, err := d.deps.Sessions.FindByIDs(ctx, groupIDs)
	if err != nil {
		d.logger.Warn("conflict detection: supervised group batch lookup failed", slog.String("error", err.Error()))
		return result
	}
	result.groups = groups
	instances, err := d.bridgedInstances(ctx, groupIDs)
	if err != nil {
		d.logger.Warn("conflict detection: bridged instance batch lookup failed", slog.String("error", err.Error()))
		return result
	}
	result.roomsLoaded = true
	instanceIDs := make([]int64, 0, len(instances))
	for _, instance := range instances {
		instanceIDs = append(instanceIDs, instance.ID)
		if instance.ActiveGroupID != nil {
			result.instances[*instance.ActiveGroupID] = instance
		}
	}
	rows, err := d.deps.InstanceStaff.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		d.logger.Warn("conflict detection: bridged instance_staff batch lookup failed", slog.String("error", err.Error()))
		return result
	}
	for _, row := range rows {
		result.staffRows[row.InstanceID] = append(result.staffRows[row.InstanceID], row)
	}
	return result
}

// bridgedInstances lists the blocks bridged to the given running sessions.
func (d *conflictDetection) bridgedInstances(ctx context.Context, groupIDs []int64) ([]*scheduleModels.ActivityInstance, error) {
	lister, ok := d.deps.Instances.(activeGroupInstanceLister)
	if !ok {
		return nil, fmt.Errorf("legacy list capability is not configured for %T", d.deps.Instances)
	}
	args := make([]any, len(groupIDs))
	for i, id := range groupIDs {
		args[i] = id
	}
	return lister.List(ctx, &modelBase.QueryOptions{Filter: modelBase.NewFilter().In("active_group_id", args...)})
}

func distinctPresentStaffIDs(rows []*scheduleModels.InstanceStaff) []int64 {
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

func indexSupervisions(byStaff map[int64][]studentpresence.GroupSupervision, rows []studentpresence.GroupSupervision) []int64 {
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

// room resolves the room a staff member is actually bound to in the given
// running group. The session stores only its primary room, so for a group
// bridged to a timetable block the staff member's per-row multi-room override
// on that block's staff rows wins — comparing against the group's primary
// room alone would suppress a real double-booking or warn although the staff
// member sits in the same room (#2151 review).
//
// A spontaneous group without a bridged block has exactly one room, so the
// group's room is authoritative. ok=false means the room could not be
// determined (lookup failure, or a bridged block whose roster does not
// contain the staff member) — callers must then keep the warning.
func (c activeStaffConflicts) room(group *studentpresence.SessionDetail, staffID int64) (roomID int64, ok bool) {
	if !c.roomsLoaded {
		return 0, false
	}
	instance := c.instances[group.ID]
	if instance == nil {
		return group.RoomID, true
	}
	for _, row := range c.staffRows[instance.ID] {
		if row.StaffID == staffID {
			return effectiveRoom(instance.RoomID, row.RoomID), true
		}
	}
	// Supervisor without a matching roster row on the bridged block —
	// anomalous (Start and the deviation flows keep both in sync), so the
	// effective room is unknown.
	return 0, false
}
