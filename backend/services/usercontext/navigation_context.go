package usercontext

import (
	"context"
	"errors"
)

// GetNavigationContext returns a complete projection or an error. A partial
// identity payload would make access-sensitive navigation lie to the caller.
func (s *userContextService) GetNavigationContext(ctx context.Context) (*NavigationContext, error) {
	groups, err := s.GetMyGroups(ctx)
	if err != nil {
		return nil, err
	}
	substituted, err := s.GetSubstitutedGroupIDs(ctx)
	if err != nil {
		return nil, err
	}
	supervised, err := s.GetMySupervisedGroups(ctx)
	if err != nil {
		return nil, err
	}
	staff, err := s.GetCurrentStaff(ctx)
	if err != nil && !errors.Is(err, ErrUserNotLinkedToStaff) && !errors.Is(err, ErrUserNotLinkedToPerson) {
		return nil, err
	}

	educational := make([]*NavigationContextGroup, 0, len(groups))
	for _, group := range groups {
		educational = append(educational, &NavigationContextGroup{
			Group:           group,
			ViaSubstitution: substituted[group.ID],
		})
	}
	return &NavigationContext{
		EducationalGroups: educational,
		SupervisedGroups:  supervised,
		CurrentStaff:      staff,
	}, nil
}
