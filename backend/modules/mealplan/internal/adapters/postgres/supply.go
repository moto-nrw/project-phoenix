package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

type supplyRow struct {
	bun.BaseModel   `bun:"table:meal_caterer_supplies,alias:meal_caterer_supply"`
	ID              int64      `bun:"id,pk,autoincrement"`
	TenantID        int64      `bun:"tenant_id"`
	CatererID       int64      `bun:"caterer_id"`
	DeliveryAddress string     `bun:"delivery_address"`
	StartedAt       time.Time  `bun:"started_at"`
	StartedBy       *int64     `bun:"started_by_account_id"`
	EndedAt         *time.Time `bun:"ended_at"`
	EndedBy         *int64     `bun:"ended_by_account_id"`
}

// FindActiveSupply reads the school's running supply. A start and a
// confirmation lock it for the rest of the transaction, so an end of the
// supply cannot cross them (ADR 0075).
func (s *Store) FindActiveSupply(ctx context.Context, forUpdate bool) (domain.Supply, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Supply{}, false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	var row supplyRow
	err = timedQuery(&stats, func() error {
		query := db.NewSelect().Model(&row).
			ModelTableExpr(`schedule.meal_caterer_supplies AS "meal_caterer_supply"`).
			Where(`"meal_caterer_supply".tenant_id = ?`, tenantID).
			Where(`"meal_caterer_supply".ended_at IS NULL`)
		if forUpdate {
			query = query.For("UPDATE")
		}
		return query.Scan(ctx)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Supply{}, false, stats, nil
	}
	if err != nil {
		return domain.Supply{}, false, stats, fmt.Errorf("meal plan postgres: find supply: %w", err)
	}
	stats.Rows++
	return domain.Supply{CatererID: row.CatererID, DeliveryAddress: row.DeliveryAddress, StartedAt: row.StartedAt}, true, stats, nil
}

// InsertSupply starts the school's supply by a caterer.
func (s *Store) InsertSupply(ctx context.Context, catererID int64, deliveryAddress string, accountID int64, startedAt time.Time) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	row := supplyRow{TenantID: tenantID, CatererID: catererID, DeliveryAddress: deliveryAddress, StartedAt: startedAt, StartedBy: optionalAccount(accountID)}
	if err := timedQuery(&stats, func() error {
		_, insertErr := db.NewInsert().Model(&row).ModelTableExpr(`schedule.meal_caterer_supplies`).
			ExcludeColumn("id", "ended_at", "ended_by_account_id").Exec(ctx)
		return insertErr
	}); err != nil {
		return stats, fmt.Errorf("meal plan postgres: insert supply: %w", err)
	}
	stats.Rows++
	return stats, nil
}

// EndSupply ends the school's running supply and reports whether one ran.
func (s *Store) EndSupply(ctx context.Context, accountID int64, endedAt time.Time) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	var ended int64
	if err := timedQuery(&stats, func() error {
		result, updateErr := db.NewUpdate().
			Table("schedule.meal_caterer_supplies").
			Set("ended_at = ?", endedAt).
			Set("ended_by_account_id = ?", optionalAccount(accountID)).
			Where("tenant_id = ?", tenantID).
			Where("ended_at IS NULL").
			Exec(ctx)
		if updateErr != nil {
			return updateErr
		}
		ended, updateErr = result.RowsAffected()
		return updateErr
	}); err != nil {
		return false, stats, fmt.Errorf("meal plan postgres: end supply: %w", err)
	}
	stats.Rows += ended
	return ended > 0, stats, nil
}

// ConfirmOrder records the caterer's confirmation of a fixed order once and
// reports whether the school has the order with a day from since on. A
// second confirmation keeps the first.
func (s *Store) ConfirmOrder(ctx context.Context, orderID, accountID int64, since calendar.Date, confirmedAt time.Time) (bool, domain.OperationStats, error) {
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
				WHERE tenant_id = ? AND id = ? AND period_end >= ? AND confirmed_at IS NULL
				RETURNING id
			)
			SELECT EXISTS (SELECT 1 FROM confirmed)
				OR EXISTS (SELECT 1 FROM schedule.meal_orders WHERE tenant_id = ? AND id = ? AND period_end >= ?)`,
			confirmedAt, optionalAccount(accountID), tenantID, orderID, since, tenantID, orderID, since).Scan(ctx, &found)
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
