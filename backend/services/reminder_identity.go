package services

import (
	"context"
	"errors"

	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
)

// reminderStaffIdentity adapts the request-memoized caller identity without
// exposing its persistence models to reminder evaluation.
func reminderStaffIdentity(identity CareRequestStaff) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		if identity == nil {
			return 0, errors.New("user context is not configured")
		}
		staffID, found, err := identity.CurrentStaffID(ctx)
		switch {
		case err != nil:
			return 0, err
		case !found:
			return 0, reminder.ErrNotLinkedToStaff
		}
		return staffID, nil
	}
}
