package usercontext

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/active"
)

// GetNavigationContext returns the identity projection with every available
// navigation section. Group-derived sections are optional: omitting one on a
// repository failure is safer than failing unrelated navigation entirely.
func (s *userContextService) GetNavigationContext(ctx context.Context) (*NavigationContext, error) {
	groups, err := s.GetMyGroups(ctx)
	if err != nil {
		s.getLogger().Warn("navigation context groups unavailable",
			slog.String("error", err.Error()),
		)
	}
	substituted, err := s.GetSubstitutedGroupIDs(ctx)
	if err != nil {
		s.getLogger().Warn("navigation context substitutions unavailable",
			slog.String("error", err.Error()),
		)
		substituted = make(map[int64]bool)
	}
	supervised, err := s.GetMySupervisedGroups(ctx)
	if err != nil {
		s.getLogger().Warn("navigation context supervision unavailable",
			slog.String("error", err.Error()),
		)
		supervised = []*active.Group{}
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
