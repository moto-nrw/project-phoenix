package compose

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// DetectPlannedConflicts checks a hypothetical slot against the day's already
// planned or running blocks and returns advisory warnings. Sibling of
// DetectStartConflicts, but planning-time: it compares against the timetable,
// not the live layer.
//
// The probe is best-effort: a failing sub-check degrades to zero warnings of
// that kind plus a log line — it must never turn into a 500 for the planner.
//
// Ordering is deterministic: staff warnings first (by instance ID, then
// assignment row), then students (by instance ID, then student).
func (d *conflictDetection) DetectPlannedConflicts(ctx context.Context, probe timetable.PlannedConflictProbe) []timetable.PlannedConflictWarning {
	overlapping := d.loadOverlappingInstances(ctx, probe)
	if len(overlapping) == 0 {
		return []timetable.PlannedConflictWarning{}
	}
	byID := make(map[int64]*scheduleModels.ActivityInstance, len(overlapping))
	ids := make([]int64, 0, len(overlapping))
	for _, instance := range overlapping {
		byID[instance.ID] = instance
		ids = append(ids, instance.ID)
	}
	warnings := make([]timetable.PlannedConflictWarning, 0)
	warnings = append(warnings, d.plannedStaffConflicts(ctx, probe, ids, byID)...)
	warnings = append(warnings, d.plannedStudentConflicts(ctx, probe, ids, byID)...)
	return warnings
}

// loadOverlappingInstances fetches the day's blocks and keeps the planned or
// running ones whose wall-clock window overlaps the probed slot (touching
// edges are not a conflict), sorted by ID. Both sides pass through
// timezone.NormalizeWallClock because TIME columns scan with driver-chosen
// date anchors.
func (d *conflictDetection) loadOverlappingInstances(ctx context.Context, probe timetable.PlannedConflictProbe) []*scheduleModels.ActivityInstance {
	day := scheduleModels.Date(probe.Date)
	instances, err := d.deps.Instances.FindByTenantAndDateRange(ctx, day, day)
	if err != nil {
		d.logger.Warn("planned conflict detection: load day instances failed",
			slog.String("date", probe.Date.String()),
			slog.String("error", err.Error()),
		)
		return nil
	}
	out := make([]*scheduleModels.ActivityInstance, 0, len(instances))
	for _, instance := range instances {
		if !isPlannableInstance(instance) {
			continue // completed/cancelled are history, not conflicts
		}
		if probe.ExcludeInstanceID != nil && instance.ID == *probe.ExcludeInstanceID {
			continue
		}
		if clockWindowsOverlap(probe.StartTime, probe.EndTime, instance.StartTime, instance.EndTime) {
			out = append(out, instance)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// plannedStaffConflicts intersects the overlapping blocks' staff rows with the
// probed staff. Absent rows are skipped. An overlap in the same concrete room
// is skipped too (#2139): one Betreuungskraft supervising several parallel
// groups in one room is sanctioned. A nil probe room is undetermined, so the
// warning stays. Staff names are not cheaply loadable here, so the copy uses
// the generic "Personal" subject and names the conflicting block instead.
func (d *conflictDetection) plannedStaffConflicts(
	ctx context.Context,
	probe timetable.PlannedConflictProbe,
	overlappingIDs []int64,
	byID map[int64]*scheduleModels.ActivityInstance,
) []timetable.PlannedConflictWarning {
	if len(probe.StaffIDs) == 0 {
		return nil
	}
	rows, err := d.deps.InstanceStaff.FindByInstanceIDs(ctx, overlappingIDs)
	if err != nil {
		d.logger.Warn("planned conflict detection: load instance_staff failed",
			slog.String("date", probe.Date.String()),
			slog.String("error", err.Error()),
		)
		return nil
	}
	requested := idSet(probe.StaffIDs)
	var warnings []timetable.PlannedConflictWarning
	for _, row := range rows {
		if _, ok := requested[row.StaffID]; !ok || row.IsAbsent {
			continue
		}
		instance, ok := byID[row.InstanceID]
		if !ok {
			continue
		}
		if note, conflict := plannedStaffRoomNote(probe.RoomID, instance, row); conflict {
			warnings = append(warnings, plannedConflictWarning(timetable.ConflictKindStaff, row.StaffID, instance,
				fmt.Sprintf("„Personal“ ist am %s von %s–%s bereits bei „%s“ eingeplant (%s).",
					probe.Date.Format(germanDateLayout), clockLabel(instance.StartTime), clockLabel(instance.EndTime), instance.Title, note)))
		}
	}
	return warnings
}

// plannedStaffRoomNote decides whether a staff overlap conflicts and names
// the room situation for the message.
func plannedStaffRoomNote(probeRoom *int64, instance *scheduleModels.ActivityInstance, row *scheduleModels.InstanceStaff) (string, bool) {
	if probeRoom == nil {
		return "Raum noch nicht festgelegt", true
	}
	if effectiveRoom(instance.RoomID, row.RoomID) == *probeRoom {
		return "", false // same concrete room — sanctioned parallel supervision
	}
	return "anderer Raum", true
}

// plannedStudentConflicts intersects the probed students with the students
// still expected on overlapping blocks (checked-in or absent students belong
// to that other block's history, not the plan).
func (d *conflictDetection) plannedStudentConflicts(
	ctx context.Context,
	probe timetable.PlannedConflictProbe,
	overlappingIDs []int64,
	byID map[int64]*scheduleModels.ActivityInstance,
) []timetable.PlannedConflictWarning {
	if len(probe.StudentIDs) == 0 {
		return nil
	}
	rows, err := d.deps.InstanceStudents.FindExpectedByInstanceIDs(ctx, overlappingIDs)
	if err != nil {
		d.logger.Warn("planned conflict detection: load instance_students failed",
			slog.String("date", probe.Date.String()),
			slog.String("error", err.Error()),
		)
		return nil
	}
	requested := idSet(probe.StudentIDs)
	var warnings []timetable.PlannedConflictWarning
	for _, row := range rows {
		if _, ok := requested[row.StudentID]; !ok {
			continue
		}
		instance, ok := byID[row.InstanceID]
		if !ok {
			continue
		}
		warnings = append(warnings, plannedConflictWarning(timetable.ConflictKindStudent, row.StudentID, instance,
			fmt.Sprintf("Kind ist am %s von %s–%s bereits bei „%s“ eingeplant.",
				probe.Date.Format(germanDateLayout), clockLabel(instance.StartTime), clockLabel(instance.EndTime), instance.Title)))
	}
	return warnings
}

func plannedConflictWarning(kind string, resourceID int64, instance *scheduleModels.ActivityInstance, message string) timetable.PlannedConflictWarning {
	return timetable.PlannedConflictWarning{
		Kind:                  kind,
		ResourceID:            resourceID,
		Message:               message,
		ConflictingInstanceID: instance.ID,
		ConflictingTitle:      instance.Title,
	}
}

// clockLabel renders a TIME value as HH:MM independent of its date anchor.
func clockLabel(value time.Time) string {
	return timezone.NormalizeWallClock(value).Format("15:04")
}

func idSet(ids []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}
