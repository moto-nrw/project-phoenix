package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Substitute time-overlap advisories (WP-B12) behind
// timetable.SubstituteConflictQuery. The staffing saves ask once for every
// substitute they placed; the retained staff move asks for one person.

// SubstituteConflictDependencies wires the advisory read.
type SubstituteConflictDependencies struct {
	Instances     scheduleModel.ActivityInstanceRepository
	InstanceStaff scheduleModel.InstanceStaffRepository
}

// NewSubstituteConflicts composes the substitute time-overlap advisories.
func NewSubstituteConflicts(deps SubstituteConflictDependencies) (timetable.SubstituteConflictQuery, error) {
	if deps.Instances == nil || deps.InstanceStaff == nil {
		return nil, errors.New("timetable substitute conflicts: required repository is nil")
	}
	return substituteConflicts{instances: deps.Instances, staff: deps.InstanceStaff}, nil
}

type substituteConflicts struct {
	instances scheduleModel.ActivityInstanceRepository
	staff     scheduleModel.InstanceStaffRepository
}

// substitutionWarningProbe is one person whose other same-day assignments
// are compared against the targets the save placed them on.
type substitutionWarningProbe struct {
	staffID   int64
	targets   []timetable.SubstituteConflictInstance
	targetIDs map[int64]bool
}

// DetectSubstituteConflicts reads the advisories of one person. A probe
// without targets reports nothing.
func (c substituteConflicts) DetectSubstituteConflicts(ctx context.Context, in timetable.SubstituteConflictProbe) ([]timetable.SubstituteTimeConflict, error) {
	if len(in.TargetIDs) == 0 {
		return nil, nil
	}
	probe := substitutionWarningProbe{staffID: in.StaffID, targets: in.Targets, targetIDs: make(map[int64]bool, len(in.TargetIDs))}
	for _, id := range in.TargetIDs {
		probe.targetIDs[id] = true
	}
	return c.loadSubstitutionWarnings(ctx, []substitutionWarningProbe{probe}, in.Date)
}

// loadSubstitutionWarnings reads every probed person's same-day assignments
// and the foreign blocks behind them in one batch each. A lookup failure
// propagates: the probe runs inside the tenant tx, and a PostgreSQL error
// aborts that tx, so the eventual commit would fail after the client saw a
// success.
func (c substituteConflicts) loadSubstitutionWarnings(
	ctx context.Context,
	probes []substitutionWarningProbe,
	date timezone.Date,
) ([]timetable.SubstituteTimeConflict, error) {
	staffIDs := make([]int64, 0, len(probes))
	for _, probe := range probes {
		staffIDs = append(staffIDs, probe.staffID)
	}
	rows, err := c.staff.FindByStaffIDsAndDate(ctx, staffIDs, scheduleModel.Date(date))
	if err != nil {
		return nil, err
	}
	rowsByStaff := make(map[int64][]*scheduleModel.InstanceStaff, len(probes))
	for _, row := range rows {
		rowsByStaff[row.StaffID] = append(rowsByStaff[row.StaffID], row)
	}
	foreignIDs := warningForeignInstanceIDs(probes, rowsByStaff)
	foreigns, err := c.instances.FindByIDs(ctx, foreignIDs)
	if err != nil {
		return nil, err
	}
	foreignByID := make(map[int64]*scheduleModel.ActivityInstance, len(foreigns))
	for _, instance := range foreigns {
		foreignByID[instance.ID] = instance
	}
	for _, id := range foreignIDs {
		if foreignByID[id] == nil {
			return nil, fmt.Errorf("load substitute conflict instance %d: %w", id, modelBase.ErrNotFound)
		}
	}
	return mergeSubstitutionWarnings(probes, rowsByStaff, foreignByID), nil
}

func warningForeignInstanceIDs(probes []substitutionWarningProbe, rowsByStaff map[int64][]*scheduleModel.InstanceStaff) []int64 {
	seen := make(map[int64]bool)
	ids := make([]int64, 0)
	for _, probe := range probes {
		for _, row := range rowsByStaff[probe.staffID] {
			if row.IsAbsent || probe.targetIDs[row.InstanceID] || seen[row.InstanceID] {
				continue
			}
			seen[row.InstanceID] = true
			ids = append(ids, row.InstanceID)
		}
	}
	return ids
}

func mergeSubstitutionWarnings(
	probes []substitutionWarningProbe,
	rowsByStaff map[int64][]*scheduleModel.InstanceStaff,
	foreignByID map[int64]*scheduleModel.ActivityInstance,
) []timetable.SubstituteTimeConflict {
	warnings := make([]timetable.SubstituteTimeConflict, 0)
	seenConflict := make(map[[3]int64]bool)
	for _, probe := range probes {
		foreigns := make([]timetable.SubstituteConflictInstance, 0)
		for _, row := range rowsByStaff[probe.staffID] {
			if row.IsAbsent || probe.targetIDs[row.InstanceID] || foreignByID[row.InstanceID] == nil {
				continue
			}
			foreigns = append(foreigns, toConflictInstance(foreignByID[row.InstanceID]))
		}
		for _, conflict := range timetable.DetectSubstituteTimeConflicts(probe.targets, foreigns) {
			key := [3]int64{probe.staffID, conflict.InstanceID, conflict.OtherID}
			if seenConflict[key] {
				continue
			}
			seenConflict[key] = true
			warnings = append(warnings, conflict)
		}
	}
	return warnings
}

// toConflictInstance converts an ActivityInstance's TIME columns into the
// minutes-since-midnight form expected by the conflict helper.
func toConflictInstance(inst *scheduleModel.ActivityInstance) timetable.SubstituteConflictInstance {
	return timetable.SubstituteConflictInstance{
		ID:        inst.ID,
		StartMin:  timetable.MinutesOfTime(inst.StartTime.Hour(), inst.StartTime.Minute()),
		EndMin:    timetable.MinutesOfTime(inst.EndTime.Hour(), inst.EndTime.Minute()),
		StartHHMM: inst.StartTime.Format("15:04"),
	}
}
