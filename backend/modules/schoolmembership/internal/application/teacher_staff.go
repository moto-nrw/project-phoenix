package application

import (
	"context"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/schoolmembership"
	"github.com/moto-ogs/project-phoenix/modules/schoolmembership/internal/domain"
)

// Preserve ListStaff's distinct, ascending membership IDs and the same
// Workforce read before the caller resolves persons, even for a narrow DTO.
func (s *Service) composeTeacherEmployment(ctx context.Context, teachers []schoolmembership.Teacher, stats *domain.OperationStats) error {
	byID := make(map[int64]schoolmembership.Staff, len(teachers))
	for _, teacher := range teachers {
		if teacher.Staff != nil {
			byID[teacher.Staff.ID] = *teacher.Staff
		}
	}
	members := make([]schoolmembership.Staff, 0, len(byID))
	for _, member := range byID {
		members = append(members, member)
	}
	slices.SortFunc(members, func(a, b schoolmembership.Staff) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	if err := s.composeEmployment(ctx, members, stats); err != nil {
		return err
	}
	for _, member := range members {
		byID[member.ID] = member
	}
	for i, teacher := range teachers {
		if teacher.Staff != nil {
			member := byID[teacher.Staff.ID]
			teachers[i].Staff = &member
		}
	}
	return nil
}
