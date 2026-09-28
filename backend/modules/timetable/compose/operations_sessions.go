package compose

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// ActiveSessions lists the day's running blocks with their plan windows.
// Purely descriptive metadata, so it carries no per-caller filter; the
// route's schedules:read gates access.
func (s *operations) ActiveSessions(ctx context.Context, date timezone.Date) ([]timetable.OperationActiveSession, error) {
	instances, err := s.deps.Instances.FindByTenantAndDate(ctx, scheduleModels.Date(date))
	if err != nil {
		return nil, err
	}
	out := make([]timetable.OperationActiveSession, 0, len(instances))
	for _, inst := range instances {
		if inst.Status != scheduleModels.InstanceStatusActive || inst.ActiveGroupID == nil {
			continue
		}
		out = append(out, timetable.OperationActiveSession{
			ActiveGroupID: *inst.ActiveGroupID,
			InstanceID:    inst.ID,
			Title:         inst.Title,
			StartTime:     inst.StartTime.Format("15:04"),
			EndTime:       inst.EndTime.Format("15:04"),
		})
	}
	return out, nil
}

// SessionBlocks resolves the running blocks behind the given live sessions
// of the day in bulk (#3281). supervisorStaffIDs maps each session to its
// current supervisors as the caller already read them; the rule that turns
// plan and supervision into CanOperate is requireCanOperate's. Sessions
// without a running block are left out. The read costs a fixed number of
// statements, whatever the number of sessions.
func (s *operations) SessionBlocks(ctx context.Context, accountID int64, isAdmin bool, date timezone.Date, supervisorStaffIDs map[int64][]int64) ([]timetable.OperationSessionBlock, error) {
	result := []timetable.OperationSessionBlock{}
	if len(supervisorStaffIDs) == 0 {
		return result, nil
	}
	running, err := s.runningBlocksOf(ctx, date, supervisorStaffIDs)
	if err != nil || len(running) == 0 {
		return result, err
	}
	staffRows, err := s.deps.InstanceStaff.FindByInstanceIDs(ctx, retainedInstanceIDs(running))
	if err != nil {
		return nil, err
	}
	staffByInstance := indexInstanceStaffRows(staffRows)
	// An account requireCanOperate cannot resolve at all operates no block,
	// admins included; it has no staff profile either.
	staffID, hasStaff, err := s.resolveStaffID(ctx, accountID)
	if err != nil && !errors.Is(err, timetable.ErrTimetableOperationForbidden) {
		return nil, err
	}
	adminActions := err == nil && s.hasAdministrativeActionAccess(ctx, isAdmin)
	for _, inst := range running {
		activeGroupID := *inst.ActiveGroupID
		rows := staffByInstance[inst.ID]
		block := timetable.OperationSessionBlock{
			ActiveGroupID: activeGroupID,
			InstanceID:    inst.ID,
			Title:         inst.Title,
			StartTime:     inst.StartTime.Format("15:04"),
			EndTime:       inst.EndTime.Format("15:04"),
			IsAssigned:    hasStaff && staffAssigned(rows, staffID),
			CanOperate:    adminActions,
		}
		if !adminActions && hasStaff {
			block.CanOperate, err = s.operatesLoadedBlock(ctx, inst, rows, staffID, func() (bool, error) {
				return slices.Contains(supervisorStaffIDs[activeGroupID], staffID), nil
			})
			if err != nil {
				return nil, err
			}
		}
		result = append(result, block)
	}
	return result, nil
}

// runningBlocksOf keeps the day's running blocks behind the asked sessions.
func (s *operations) runningBlocksOf(ctx context.Context, date timezone.Date, supervisorStaffIDs map[int64][]int64) ([]*scheduleModels.ActivityInstance, error) {
	instances, err := s.deps.Instances.FindByTenantAndDate(ctx, scheduleModels.Date(date))
	if err != nil {
		return nil, err
	}
	running := make([]*scheduleModels.ActivityInstance, 0, len(supervisorStaffIDs))
	for _, inst := range instances {
		if inst.Status != scheduleModels.InstanceStatusActive || inst.ActiveGroupID == nil {
			continue
		}
		if _, asked := supervisorStaffIDs[*inst.ActiveGroupID]; asked {
			running = append(running, inst)
		}
	}
	return running, nil
}

// EarliestPlannedBlockStartForClass returns the wall-clock start ("HH:MM")
// of the day's first block that addresses the school class, "" when there
// is none. It is the preset behind "Unterricht fällt aus" (#2962/#2970): a
// class released early arrives when its first block begins.
//
// A template addresses a class through its own target, a dynamic target or
// its offering-source class filter, compared by the normalized class name.
// Cancelled blocks do not count. Jahrgang and Gruppe targets are not
// resolved: the OGS dialog reads the same three fields, and both presets
// must agree.
func (s *operations) EarliestPlannedBlockStartForClass(ctx context.Context, schoolClass string, date timezone.Date) (string, error) {
	key := s.deps.NormalizeSchoolClass(schoolClass)
	if key == "" {
		return "", nil
	}
	instances, err := s.deps.Instances.FindByTenantAndDate(ctx, scheduleModels.Date(date))
	if err != nil {
		return "", fmt.Errorf("earliest block start: load instances: %w", err)
	}
	candidates, groupIDs := templateBackedBlocks(instances)
	if len(candidates) == 0 {
		return "", nil
	}
	applies, err := s.templatesAddressingClass(ctx, groupIDs, key)
	if err != nil {
		return "", err
	}
	earliest := ""
	for _, inst := range candidates {
		if !applies[*inst.ActivityGroupID] {
			continue
		}
		if start := inst.StartTime.Format("15:04"); earliest == "" || start < earliest {
			earliest = start
		}
	}
	return earliest, nil
}

// templateBackedBlocks keeps the non-cancelled blocks with a template and
// the distinct templates they use.
func templateBackedBlocks(instances []*scheduleModels.ActivityInstance) ([]*scheduleModels.ActivityInstance, []int64) {
	candidates := make([]*scheduleModels.ActivityInstance, 0, len(instances))
	groupIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		if inst == nil || inst.Status == scheduleModels.InstanceStatusCancelled || inst.ActivityGroupID == nil || *inst.ActivityGroupID <= 0 {
			continue
		}
		candidates = append(candidates, inst)
		if !slices.Contains(groupIDs, *inst.ActivityGroupID) {
			groupIDs = append(groupIDs, *inst.ActivityGroupID)
		}
	}
	return candidates, groupIDs
}

// templatesAddressingClass reads the templates and their targets and keeps
// the ones that name the normalized class.
func (s *operations) templatesAddressingClass(ctx context.Context, groupIDs []int64, key string) (map[int64]bool, error) {
	groups, err := s.deps.Templates.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("earliest block start: load templates: %w", err)
	}
	targetsByGroup, err := s.deps.Templates.FindTargetsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("earliest block start: load template targets: %w", err)
	}
	applies := make(map[int64]bool, len(groups))
	for _, group := range groups {
		if group != nil && s.templateAddressesClass(group, targetsByGroup[group.ID], key) {
			applies[group.ID] = true
		}
	}
	return applies, nil
}

// templateAddressesClass reports whether the template names the normalized
// class in its own target, one of its dynamic targets, or its offering class
// filter.
func (s *operations) templateAddressesClass(group *activitiesModels.Group, targets []*activitiesModels.GroupTarget, key string) bool {
	if group.TargetSchoolClass != nil && s.deps.NormalizeSchoolClass(*group.TargetSchoolClass) == key {
		return true
	}
	for _, target := range targets {
		if target != nil && target.TargetSchoolClass != nil && s.deps.NormalizeSchoolClass(*target.TargetSchoolClass) == key {
			return true
		}
	}
	for _, class := range group.SourceSchoolClasses {
		if s.deps.NormalizeSchoolClass(class) == key {
			return true
		}
	}
	return false
}
