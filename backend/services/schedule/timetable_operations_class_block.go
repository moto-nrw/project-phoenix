package schedule

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// EarliestPlannedBlockStartForClass returns the wall-clock start ("HH:MM")
// of the first Betreuungsplan block of the date that addresses the school
// class, or "" when the day has none. It is the preset behind "Unterricht
// fällt aus" (#2962/#2970): a class released early arrives when its first
// block begins.
//
// A template addresses a class when its own target, one of its dynamic
// targets, or its offering-source class filter names the class (compared
// via schoolclass.Normalize). Cancelled blocks do not count: the class would
// arrive into nothing. Jahrgang and Gruppe targets are deliberately not
// resolved — the OGS dialog reads the same three fields, and both presets
// must agree.
func (s *timetableOperationsService) EarliestPlannedBlockStartForClass(
	ctx context.Context,
	schoolClass string,
	date timezone.Date,
) (string, error) {
	key := schoolclass.Normalize(schoolClass)
	if key == "" {
		return "", nil
	}
	instances, err := s.deps.InstanceRepo.FindByTenantAndDate(ctx, date)
	if err != nil {
		return "", fmt.Errorf("earliest block start: load instances: %w", err)
	}
	candidates := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	groupIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		if inst == nil || inst.Status == scheduleModel.InstanceStatusCancelled || inst.ActivityGroupID == nil || *inst.ActivityGroupID <= 0 {
			continue
		}
		candidates = append(candidates, inst)
		if !slices.Contains(groupIDs, *inst.ActivityGroupID) {
			groupIDs = append(groupIDs, *inst.ActivityGroupID)
		}
	}
	if len(candidates) == 0 {
		return "", nil
	}
	groups, err := s.deps.ActivityGroupRepo.FindByIDs(ctx, groupIDs)
	if err != nil {
		return "", fmt.Errorf("earliest block start: load templates: %w", err)
	}
	targetsByGroup := map[int64][]*activitiesModel.GroupTarget{}
	if targetRepo, ok := s.deps.ActivityGroupRepo.(interface {
		FindTargetsByGroupIDs(context.Context, []int64) (map[int64][]*activitiesModel.GroupTarget, error)
	}); ok {
		targetsByGroup, err = targetRepo.FindTargetsByGroupIDs(ctx, groupIDs)
		if err != nil {
			return "", fmt.Errorf("earliest block start: load template targets: %w", err)
		}
	}
	applies := make(map[int64]bool, len(groups))
	for _, group := range groups {
		if group != nil && templateAddressesClass(group, targetsByGroup[group.ID], key) {
			applies[group.ID] = true
		}
	}
	earliest := ""
	for _, inst := range candidates {
		if !applies[*inst.ActivityGroupID] {
			continue
		}
		start := inst.StartTime.Format("15:04")
		if earliest == "" || start < earliest {
			earliest = start
		}
	}
	return earliest, nil
}

// templateAddressesClass reports whether the template names the normalized
// class in its own target, one of its dynamic targets, or its offering
// class filter.
func templateAddressesClass(group *activitiesModel.Group, targets []*activitiesModel.GroupTarget, key string) bool {
	if group.TargetSchoolClass != nil && schoolclass.Normalize(*group.TargetSchoolClass) == key {
		return true
	}
	for _, target := range targets {
		if target != nil && target.TargetSchoolClass != nil && schoolclass.Normalize(*target.TargetSchoolClass) == key {
			return true
		}
	}
	for _, class := range group.SourceSchoolClasses {
		if schoolclass.Normalize(class) == key {
			return true
		}
	}
	return false
}
