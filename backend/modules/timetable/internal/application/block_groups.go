package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// BlockEducationGroups names the OGS groups of the given blocks for the
// Betreuungsplan's group filter (#4766): the template's own group, its
// "gruppe" participant sources and the current group of every planned child.
func (s *Service) BlockEducationGroups(ctx context.Context, instanceIDs []int64) (result map[int64][]int64, err error) {
	if hasInvalidID(instanceIDs) {
		return nil, s.reject("block_education_groups", timetable.ErrInvalidGroupQuery)
	}
	result = map[int64][]int64{}
	if len(instanceIDs) == 0 {
		return result, nil
	}
	err = s.run(ctx, "block_education_groups", func(ctx context.Context, stats *domain.OperationStats) error {
		groups, loadErr := s.loadBlockEducationGroups(ctx, instanceIDs, stats)
		result = groups
		return loadErr
	})
	return result, err
}

func (s *Service) loadBlockEducationGroups(ctx context.Context, instanceIDs []int64, stats *domain.OperationStats) (map[int64][]int64, error) {
	groups := blockGroupSets{}
	templateGroups, err := s.templateEducationGroups(ctx, instanceIDs, stats)
	if err != nil {
		return nil, err
	}
	for instanceID, groupIDs := range templateGroups {
		groups.add(instanceID, groupIDs...)
	}
	childGroups, err := s.plannedChildEducationGroups(ctx, instanceIDs, stats)
	if err != nil {
		return nil, err
	}
	for instanceID, groupIDs := range childGroups {
		groups.add(instanceID, groupIDs...)
	}
	return groups.sorted(), nil
}

// blockGroupSets collects the distinct groups of each block.
type blockGroupSets map[int64]map[int64]struct{}

func (g blockGroupSets) add(instanceID int64, groupIDs ...int64) {
	for _, groupID := range groupIDs {
		if g[instanceID] == nil {
			g[instanceID] = map[int64]struct{}{}
		}
		g[instanceID][groupID] = struct{}{}
	}
}

func (g blockGroupSets) sorted() map[int64][]int64 {
	result := make(map[int64][]int64, len(g))
	for instanceID, set := range g {
		groupIDs := make([]int64, 0, len(set))
		for groupID := range set {
			groupIDs = append(groupIDs, groupID)
		}
		slices.Sort(groupIDs)
		result[instanceID] = groupIDs
	}
	return result
}

// plannedChildEducationGroups maps each block to the current groups of the
// children planned on it.
func (s *Service) plannedChildEducationGroups(ctx context.Context, instanceIDs []int64, stats *domain.OperationStats) (map[int64][]int64, error) {
	slots, slotStats, err := s.store.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: instanceIDs})
	stats.Add(slotStats)
	if err != nil || len(slots) == 0 {
		return nil, err
	}
	students, studentStats, err := s.students.ListEnrolledStudents(ctx)
	stats.Add(studentStats)
	if err != nil {
		return nil, err
	}
	groupOfStudent := make(map[int64]int64, len(students))
	for _, student := range students {
		if student.EducationGroupID != nil {
			groupOfStudent[student.ID] = *student.EducationGroupID
		}
	}
	result := map[int64][]int64{}
	for _, slot := range slots {
		if groupID, ok := groupOfStudent[slot.StudentID]; ok {
			result[slot.InstanceID] = append(result[slot.InstanceID], groupID)
		}
	}
	return result, nil
}

// templateEducationGroups maps each template-backed block to the group its
// template targets and the groups of its "gruppe" participant sources.
func (s *Service) templateEducationGroups(ctx context.Context, instanceIDs []int64, stats *domain.OperationStats) (map[int64][]int64, error) {
	blocksOfTemplate, err := s.blocksOfTemplates(ctx, instanceIDs, stats)
	if err != nil || len(blocksOfTemplate) == 0 {
		return nil, err
	}
	templateIDs := make([]int64, 0, len(blocksOfTemplate))
	for templateID := range blocksOfTemplate {
		templateIDs = append(templateIDs, templateID)
	}
	templates, groupStats, err := s.store.ListGroups(ctx, timetable.GroupFilter{IDs: templateIDs})
	stats.Add(groupStats)
	if err != nil {
		return nil, err
	}
	targets, targetStats, err := s.store.ListGroupTargets(ctx, templateIDs)
	stats.Add(targetStats)
	if err != nil {
		return nil, err
	}
	result := map[int64][]int64{}
	for _, template := range templates {
		groupIDs := templateGroupIDs(template, targets[template.ID])
		for _, instanceID := range blocksOfTemplate[template.ID] {
			result[instanceID] = append(result[instanceID], groupIDs...)
		}
	}
	return result, nil
}

// blocksOfTemplates groups the template-backed blocks by their template.
func (s *Service) blocksOfTemplates(ctx context.Context, instanceIDs []int64, stats *domain.OperationStats) (map[int64][]int64, error) {
	instances, instanceStats, err := s.store.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{IDs: instanceIDs})
	stats.Add(instanceStats)
	if err != nil {
		return nil, err
	}
	blocks := map[int64][]int64{}
	for _, instance := range instances {
		if instance.ActivityGroupID != nil {
			blocks[*instance.ActivityGroupID] = append(blocks[*instance.ActivityGroupID], instance.ID)
		}
	}
	return blocks, nil
}

// templateGroupIDs is the template's own group and its "gruppe" sources.
func templateGroupIDs(template timetable.Group, targets []timetable.GroupTarget) []int64 {
	groupIDs := make([]int64, 0, 1+len(targets))
	if template.EducationGroupID != nil {
		groupIDs = append(groupIDs, *template.EducationGroupID)
	}
	for _, target := range targets {
		if target.TargetGroupType == timetable.TargetGroupTypeEducationGroup && target.EducationGroupID != nil {
			groupIDs = append(groupIDs, *target.EducationGroupID)
		}
	}
	return groupIDs
}
