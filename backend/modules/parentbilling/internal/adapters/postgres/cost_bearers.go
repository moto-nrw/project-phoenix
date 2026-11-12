package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The cost bearers of a Träger (#5627). A line of a frozen contribution,
// a position of an invoice and a share of a monthly contribution (#4687) name a cost bearer by its ID without a foreign
// key, so the erasure of a Träger deletes both in one cascade; the delete
// here refuses a cost bearer they name. The row lock of LockCostBearer
// orders a confirmation and a deletion of the same cost bearer.

// deleteUnusedCostBearerQuery deletes the cost bearer only when no line and
// no position names it.
const deleteUnusedCostBearerQuery = `DELETE FROM parent_billing.cost_bearers AS bearer
	WHERE bearer.id = ? AND bearer.organization_id = ?
		AND NOT EXISTS (SELECT 1 FROM parent_billing.holiday_contribution_lines AS line
			WHERE line.organization_id = bearer.organization_id AND line.cost_bearer_id = bearer.id)
		AND NOT EXISTS (SELECT 1 FROM parent_billing.document_positions AS position
			WHERE position.organization_id = bearer.organization_id AND position.cost_bearer_id = bearer.id)
		AND NOT EXISTS (SELECT 1 FROM parent_billing.care_cost_shares AS share
			WHERE share.organization_id = bearer.organization_id AND share.cost_bearer_id = bearer.id)
		AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoice_cost_shares AS issued
			WHERE issued.organization_id = bearer.organization_id AND issued.cost_bearer_id = bearer.id)`

// costBearersInUseQuery names the cost bearers lines or positions name.
const costBearersInUseQuery = `SELECT cost_bearer_id FROM parent_billing.holiday_contribution_lines
		WHERE organization_id = ? AND cost_bearer_id IS NOT NULL
	UNION
	SELECT cost_bearer_id FROM parent_billing.document_positions
		WHERE organization_id = ? AND cost_bearer_id IS NOT NULL
	UNION
	SELECT cost_bearer_id FROM parent_billing.care_cost_shares WHERE organization_id = ?
	UNION
	SELECT cost_bearer_id FROM parent_billing.care_invoice_cost_shares WHERE organization_id = ?`

type costBearerRow struct {
	bun.BaseModel `bun:"table:parent_billing.cost_bearers"`

	ID             int64     `bun:"id,pk,autoincrement"`
	OrganizationID int64     `bun:"organization_id"`
	Name           string    `bun:"name"`
	Address        string    `bun:"address"`
	FileReference  string    `bun:"file_reference"`
	Active         bool      `bun:"active"`
	UpdatedAt      time.Time `bun:"updated_at,nullzero,default:current_timestamp"`
}

func (r costBearerRow) costBearer() parentbilling.CostBearer {
	return parentbilling.CostBearer{
		ID: r.ID, Name: r.Name, Address: r.Address, FileReference: r.FileReference, Active: r.Active, UpdatedAt: r.UpdatedAt,
	}
}

func newCostBearerRow(organizationID int64, input parentbilling.CostBearerInput) *costBearerRow {
	return &costBearerRow{
		OrganizationID: organizationID, Name: input.Name, Address: input.Address, FileReference: input.FileReference, Active: input.Active,
	}
}

// ListCostBearers lists the cost bearers of the Träger by name.
func (s *Store) ListCostBearers(ctx context.Context, organizationID int64) ([]parentbilling.CostBearer, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []costBearerRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).OrderExpr("name, id").Scan(ctx); err != nil {
		return nil, err
	}
	bearers := make([]parentbilling.CostBearer, len(rows))
	for i, row := range rows {
		bearers[i] = row.costBearer()
	}
	return bearers, nil
}

// CostBearer reads one cost bearer of the Träger.
func (s *Store) CostBearer(ctx context.Context, organizationID, costBearerID int64) (parentbilling.CostBearer, error) {
	return s.selectCostBearer(ctx, organizationID, costBearerID, "")
}

// LockCostBearer reads one cost bearer of the Träger and holds its row until
// the transaction ends: a confirmation shares the lock while it writes the
// lines that name the cost bearer, a deletion takes it exclusively before
// it checks whether anything names it. So a deletion waits for a running
// confirmation and then sees its lines, and a confirmation waits for a
// running deletion and then finds no cost bearer.
func (s *Store) LockCostBearer(ctx context.Context, organizationID, costBearerID int64, exclusive bool) (parentbilling.CostBearer, error) {
	if exclusive {
		return s.selectCostBearer(ctx, organizationID, costBearerID, "UPDATE")
	}
	return s.selectCostBearer(ctx, organizationID, costBearerID, "SHARE")
}

func (s *Store) selectCostBearer(ctx context.Context, organizationID, costBearerID int64, lock string) (parentbilling.CostBearer, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CostBearer{}, err
	}
	var row costBearerRow
	query := db.NewSelect().Model(&row).Where("id = ?", costBearerID).Where("organization_id = ?", organizationID)
	if lock != "" {
		query = query.For(lock)
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.CostBearer{}, parentbilling.ErrCostBearerNotFound
		}
		return parentbilling.CostBearer{}, err
	}
	return row.costBearer(), nil
}

// InsertCostBearer stores a new cost bearer.
func (s *Store) InsertCostBearer(ctx context.Context, organizationID int64, input parentbilling.CostBearerInput) (parentbilling.CostBearer, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CostBearer{}, err
	}
	row := newCostBearerRow(organizationID, input)
	if _, err := db.NewInsert().Model(row).Returning("id, updated_at").Exec(ctx); err != nil {
		return parentbilling.CostBearer{}, err
	}
	return row.costBearer(), nil
}

// UpdateCostBearer replaces the fields of a cost bearer.
func (s *Store) UpdateCostBearer(ctx context.Context, organizationID, costBearerID int64, input parentbilling.CostBearerInput) (parentbilling.CostBearer, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CostBearer{}, err
	}
	row := newCostBearerRow(organizationID, input)
	row.ID = costBearerID
	result, err := db.NewUpdate().Model(row).Column("name", "address", "file_reference", "active").
		Where("id = ?", costBearerID).Where("organization_id = ?", organizationID).
		Returning("updated_at").Exec(ctx)
	if err != nil {
		return parentbilling.CostBearer{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return parentbilling.CostBearer{}, err
	}
	if affected == 0 {
		return parentbilling.CostBearer{}, parentbilling.ErrCostBearerNotFound
	}
	return row.costBearer(), nil
}

// DeleteCostBearer deletes a cost bearer no line and no position names.
// When nothing is deleted it tells a missing cost bearer from one in use.
func (s *Store) DeleteCostBearer(ctx context.Context, organizationID, costBearerID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.NewRaw(deleteUnusedCostBearerQuery, costBearerID, organizationID).Exec(ctx)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	exists, err := db.NewSelect().Model((*costBearerRow)(nil)).
		Where("id = ?", costBearerID).Where("organization_id = ?", organizationID).Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return parentbilling.ErrCostBearerInUse
	}
	return parentbilling.ErrCostBearerNotFound
}

// CostBearersInUse names the cost bearers of the Träger a frozen
// contribution line or an invoice position names.
func (s *Store) CostBearersInUse(ctx context.Context, organizationID int64) (map[int64]bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var ids []int64
	if err := db.NewRaw(costBearersInUseQuery, organizationID, organizationID, organizationID, organizationID).Scan(ctx, &ids); err != nil {
		return nil, err
	}
	inUse := make(map[int64]bool, len(ids))
	for _, id := range ids {
		inUse[id] = true
	}
	return inUse, nil
}

// RegistrationConfirmations names when each registration of the period with
// a frozen contribution was confirmed (#5627).
func (s *Store) RegistrationConfirmations(ctx context.Context, organizationID, periodID int64) (map[int64]time.Time, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		RegistrationID int64     `bun:"holiday_registration_id"`
		ConfirmedAt    time.Time `bun:"confirmed_at"`
	}
	if err := db.NewSelect().TableExpr("parent_billing.holiday_contributions").
		ColumnExpr("holiday_registration_id, min(confirmed_at) AS confirmed_at").
		Where("organization_id = ?", organizationID).Where("holiday_period_id = ?", periodID).
		GroupExpr("holiday_registration_id").Scan(ctx, &rows); err != nil {
		return nil, err
	}
	confirmed := make(map[int64]time.Time, len(rows))
	for _, row := range rows {
		confirmed[row.RegistrationID] = row.ConfirmedAt
	}
	return confirmed, nil
}
