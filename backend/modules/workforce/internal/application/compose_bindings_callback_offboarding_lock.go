package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func NewOffboardingLock(service *Service) func(ctx context.Context, staffID int64) error {
	return func(ctx context.Context, staffID int64) error {
		if _, ok := tenant.TransactionFromContext(ctx); !ok {
			return errors.New("workforce: offboarding lock requires a transaction")
		}
		return service.LockStaffOffboarding(ctx, staffID)
	}
}
