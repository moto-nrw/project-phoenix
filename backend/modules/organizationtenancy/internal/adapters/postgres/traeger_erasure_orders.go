package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// Träger erasure orders (#4657, ADR 0092). platform.organization_erasure_orders
// has no tenant: only the operator's administrative transaction reaches it.
// The erasure CLI reads and completes it through the SECURITY DEFINER
// functions of migration 1.15.533, never through this adapter.

type traegerErasureOrderRow struct {
	bun.BaseModel  `bun:"table:platform.organization_erasure_orders,alias:erasure_order"`
	ID             int64         `bun:"id"`
	OrganizationID int64         `bun:"organization_id"`
	OrderReference string        `bun:"order_reference"`
	ConfirmedOn    calendar.Date `bun:"confirmed_on,type:date"`
	EraseNotBefore calendar.Date `bun:"erase_not_before,type:date"`
	RecordedBy     int64         `bun:"recorded_by"`
	RecordedAt     time.Time     `bun:"recorded_at"`
	StartedAt      *time.Time    `bun:"started_at"`
}

func (r traegerErasureOrderRow) toDomain() organizationtenancy.TraegerErasureOrder {
	return organizationtenancy.TraegerErasureOrder{
		ID: r.ID, OrganizationID: r.OrganizationID, OrderReference: r.OrderReference,
		ConfirmedOn: r.ConfirmedOn, EraseNotBefore: r.EraseNotBefore,
		RecordedBy: r.RecordedBy, RecordedAt: r.RecordedAt, StartedAt: r.StartedAt,
	}
}

const traegerErasureOrderColumns = `id, organization_id, order_reference, confirmed_on, erase_not_before, recorded_by, recorded_at, started_at`

const findOpenTraegerErasureOrderQuery = `SELECT ` + traegerErasureOrderColumns + `
	FROM platform.organization_erasure_orders
	WHERE organization_id = ? AND withdrawn_at IS NULL`

const lockOpenTraegerErasureOrderQuery = findOpenTraegerErasureOrderQuery + ` FOR UPDATE`

// FindOpenTraegerErasureOrder returns the open (not withdrawn) order of the
// Träger; lock "UPDATE" locks it until the caller's transaction ends.
func (s *Store) FindOpenTraegerErasureOrder(ctx context.Context, organizationID int64, lock string) (organizationtenancy.TraegerErasureOrder, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return organizationtenancy.TraegerErasureOrder{}, false, domain.OperationStats{}, err
	}
	var rows []traegerErasureOrderRow
	started := time.Now()
	if lock == "UPDATE" {
		err = db.NewRaw(lockOpenTraegerErasureOrderQuery, organizationID).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(findOpenTraegerErasureOrderQuery, organizationID).Scan(ctx, &rows)
	}
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return organizationtenancy.TraegerErasureOrder{}, false, stats, fmt.Errorf("organization tenancy postgres: find Träger erasure order: %w", err)
	}
	if len(rows) == 0 {
		return organizationtenancy.TraegerErasureOrder{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}

const insertTraegerErasureOrderQuery = `INSERT INTO platform.organization_erasure_orders
		(organization_id, order_reference, confirmed_on, erase_not_before, recorded_by)
	VALUES (?, ?, ?, ?, ?)
	RETURNING ` + traegerErasureOrderColumns

// InsertTraegerErasureOrder records the order. A second open order of the
// same Träger violates organization_erasure_orders_one_open.
func (s *Store) InsertTraegerErasureOrder(ctx context.Context, order organizationtenancy.NewTraegerErasureOrder) (organizationtenancy.TraegerErasureOrder, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return organizationtenancy.TraegerErasureOrder{}, domain.OperationStats{}, err
	}
	var row traegerErasureOrderRow
	started := time.Now()
	err = db.NewRaw(insertTraegerErasureOrderQuery, order.OrganizationID, order.OrderReference,
		order.ConfirmedOn, order.EraseNotBefore, order.RecordedBy).Scan(ctx, &row)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: 1}
	if err != nil {
		if isIntegrityViolation(err) {
			return organizationtenancy.TraegerErasureOrder{}, stats, organizationtenancy.ErrTraegerErasureOrderOpen
		}
		return organizationtenancy.TraegerErasureOrder{}, stats, fmt.Errorf("organization tenancy postgres: insert Träger erasure order: %w", err)
	}
	return row.toDomain(), stats, nil
}

const withdrawTraegerErasureOrderQuery = `UPDATE platform.organization_erasure_orders
	SET withdrawn_by = ?, withdrawn_at = clock_timestamp()
	WHERE id = ? AND withdrawn_at IS NULL AND started_at IS NULL AND completed_at IS NULL`

// WithdrawTraegerErasureOrder withdraws an open order whose erasure has not
// started.
func (s *Store) WithdrawTraegerErasureOrder(ctx context.Context, orderID, operatorID int64) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(withdrawTraegerErasureOrderQuery, operatorID, orderID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("organization tenancy postgres: withdraw Träger erasure order: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}
