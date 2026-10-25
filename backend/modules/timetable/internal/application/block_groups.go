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
	candidates, candidateStats, err := s.store.ListBlockGroupCandidates(ctx, instanceIDs)
	stats.Add(candidateStats)
	if err != nil {
		return nil, err
	}
	childrenByBlock := make(map[int64][]int64, len(candidates))
	for _, candidate := range candidates {
		if candidate.TemplateEducationGroupID != nil {
			groups.add(candidate.InstanceID, *candidate.TemplateEducationGroupID)
		}
		if candidate.TargetGroupType == timetable.TargetGroupTypeEducationGroup && candidate.TargetEducationGroupID != nil {
			groups.add(candidate.InstanceID, *candidate.TargetEducationGroupID)
		}
		if candidate.StudentID != nil {
			childrenByBlock[candidate.InstanceID] = append(childrenByBlock[candidate.InstanceID], *candidate.StudentID)
		}
	}
	childGroups, err := s.plannedChildEducationGroups(ctx, childrenByBlock, stats)
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
func (s *Service) plannedChildEducationGroups(ctx context.Context, childrenByBlock map[int64][]int64, stats *domain.OperationStats) (map[int64][]int64, error) {
	if len(childrenByBlock) == 0 {
		return map[int64][]int64{}, nil
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
	for instanceID, studentIDs := range childrenByBlock {
		for _, studentID := range studentIDs {
			if groupID, ok := groupOfStudent[studentID]; ok {
				result[instanceID] = append(result[instanceID], groupID)
			}
		}
	}
	return result, nil
}
