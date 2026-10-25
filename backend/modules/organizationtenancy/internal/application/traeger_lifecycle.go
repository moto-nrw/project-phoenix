package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Träger erasure orders (#4657). The order of a deactivated Träger records
// the written confirmation and opens the grace period; the erasure CLI checks
// and completes it (ADR 0092). The Träger row is locked before the order is
// written, so deactivation, reactivation and the order serialize on it.

// FindOpenTraegerErasureOrder returns the open order of the Träger, if any.
func (s *Service) FindOpenTraegerErasureOrder(ctx context.Context, organizationID int64) (order organizationtenancy.TraegerErasureOrder, found bool, err error) {
	err = s.runRead(ctx, "find_traeger_erasure_order", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, exists, findStats, findErr := s.store.FindByID(txCtx, organizationID, "")
		stats.Add(findStats)
		if findErr != nil {
			return findErr
		}
		if !exists {
			return organizationtenancy.ErrOrganizationNotFound
		}
		var orderStats domain.OperationStats
		order, found, orderStats, findErr = s.store.FindOpenTraegerErasureOrder(txCtx, organizationID, "")
		stats.Add(orderStats)
		return findErr
	})
	return order, found, err
}

// CreateTraegerErasureOrder records the order of a deactivated Träger. The
// written confirmation lies in the past or today; the erasure may run only
// after today, so every order has a grace period in which it can be
// withdrawn.
func (s *Service) CreateTraegerErasureOrder(ctx context.Context, input organizationtenancy.NewTraegerErasureOrder) (order organizationtenancy.TraegerErasureOrder, err error) {
	input.OrderReference = strings.TrimSpace(input.OrderReference)
	if err := validateTraegerErasureOrder(input, calendar.TodayDate()); err != nil {
		return organizationtenancy.TraegerErasureOrder{}, err
	}
	err = s.run(ctx, "create_traeger_erasure_order", func(txCtx context.Context, stats *domain.OperationStats) error {
		organization, found, findStats, findErr := s.store.FindByID(txCtx, input.OrganizationID, "UPDATE")
		stats.Add(findStats)
		switch {
		case findErr != nil:
			return findErr
		case !found:
			return organizationtenancy.ErrOrganizationNotFound
		case organization.Active:
			return organizationtenancy.ErrTraegerStillActive
		}
		var insertStats domain.OperationStats
		order, insertStats, findErr = s.store.InsertTraegerErasureOrder(txCtx, input)
		stats.Add(insertStats)
		return findErr
	})
	return order, err
}

// WithdrawTraegerErasureOrder withdraws the open order of the Träger. Once
// the erasure CLI started, the order stays: a failed run resumes with it.
func (s *Service) WithdrawTraegerErasureOrder(ctx context.Context, organizationID, operatorID int64) (order organizationtenancy.TraegerErasureOrder, err error) {
	if operatorID <= 0 {
		return organizationtenancy.TraegerErasureOrder{}, invalid("operator is required")
	}
	err = s.run(ctx, "withdraw_traeger_erasure_order", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var findStats domain.OperationStats
		var findErr error
		order, found, findStats, findErr = s.store.FindOpenTraegerErasureOrder(txCtx, organizationID, "UPDATE")
		stats.Add(findStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return organizationtenancy.ErrTraegerErasureOrderNotFound
		}
		if order.StartedAt != nil {
			return organizationtenancy.ErrTraegerErasureStarted
		}
		withdrawn, withdrawStats, withdrawErr := s.store.WithdrawTraegerErasureOrder(txCtx, order.ID, operatorID)
		stats.Add(withdrawStats)
		if withdrawErr == nil && !withdrawn {
			return organizationtenancy.ErrTraegerErasureOrderNotFound
		}
		return withdrawErr
	})
	return order, err
}

// validateTraegerErasureOrder checks the order against today's calendar day.
func validateTraegerErasureOrder(input organizationtenancy.NewTraegerErasureOrder, today calendar.Date) error {
	switch {
	case input.RecordedBy <= 0:
		return invalid("operator is required")
	case !orderReferencePattern.MatchString(input.OrderReference):
		return invalid("order reference must be a case number of 3-40 uppercase letters, digits or hyphens")
	case input.ConfirmedOn.IsZero() || input.EraseNotBefore.IsZero():
		return invalid("confirmation date and erasure date are required")
	case input.ConfirmedOn.After(today):
		return invalid("the written confirmation cannot lie in the future")
	case !input.EraseNotBefore.After(today):
		return invalid("the erasure date must lie after today")
	}
	return nil
}
