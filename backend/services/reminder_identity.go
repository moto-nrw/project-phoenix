package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/services/usercontext"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
)

// reminderStaffIdentity adapts the existing request-memoized identity lookup
// without exposing its persistence models to reminder evaluation.
func reminderStaffIdentity(identity usercontext.UserContextService) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		if identity == nil {
			return 0, errors.New("user context is not configured")
		}
		staff, err := identity.GetCurrentStaff(ctx)
		switch {
		case errors.Is(err, usercontext.ErrUserNotLinkedToStaff), errors.Is(err, usercontext.ErrUserNotLinkedToPerson):
			return 0, reminder.ErrNotLinkedToStaff
		case err != nil:
			return 0, err
		}
		return staff.ID, nil
	}
}
