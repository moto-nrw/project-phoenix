package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
)

// ConfirmOrder records the caterer's confirmation of a fixed order once and
// reports whether the order has a day from firstDay, the first day of the
// current supply, on. A second confirmation keeps the first; one from before
// since, the predecessor's, is replaced.
func (s *Store) ConfirmOrder(ctx context.Context, orderID, accountID int64, firstDay domain.Date, since, confirmedAt time.Time) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	var found bool
	if err := timedQuery(&stats, func() error {
		return db.NewRaw(`
			WITH confirmed AS (
				UPDATE schedule.meal_orders
				SET confirmed_at = ?, confirmed_by_account_id = ?
				WHERE tenant_id = ? AND id = ? AND period_end >= ? AND (confirmed_at IS NULL OR confirmed_at < ?)
				RETURNING id
			)
			SELECT EXISTS (SELECT 1 FROM confirmed)
				OR EXISTS (SELECT 1 FROM schedule.meal_orders WHERE tenant_id = ? AND id = ? AND period_end >= ?)`,
			confirmedAt, optionalAccount(accountID), tenantID, orderID, firstDay, since, tenantID, orderID, firstDay).Scan(ctx, &found)
	}); err != nil {
		return false, stats, fmt.Errorf("meal plan postgres: confirm order: %w", err)
	}
	return found, stats, nil
}

func optionalAccount(accountID int64) *int64 {
	if accountID <= 0 {
		return nil
	}
	return &accountID
}
