package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The cost bearer shares of the monthly contribution (#4687, 1.15.816):
// the shares of the contracts with their trail, and the shares of an issued
// invoice. Every statement names the Träger in its own filter, also below
// row level security.

const (
	// careCostShareContractKey keeps a share to a contract of its Träger.
	careCostShareContractKey = "fk_care_cost_shares_contract"
	// careCostShareBearerKey keeps a share to a cost bearer of its Träger.
	careCostShareBearerKey = "fk_care_cost_shares_bearer"
	// careCostShareOverlap keeps one share of a kind per contract and day.
	careCostShareOverlap = "excl_care_cost_shares_overlap"
)

type careCostShareRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_cost_shares,alias:share"`

	ID                  int64         `bun:"id,pk,autoincrement"`
	OrganizationID      int64         `bun:"organization_id"`
	CareContractID      int64         `bun:"care_contract_id"`
	Kind                string        `bun:"kind"`
	CostBearerID        int64         `bun:"cost_bearer_id"`
	ShareBasisPoints    *int          `bun:"share_basis_points"`
	ShareCents          *int64        `bun:"share_cents"`
	ValidFrom           calendar.Date `bun:"valid_from,type:date"`
	ValidUntil          calendar.Date `bun:"valid_until,type:date"`
	Reference           string        `bun:"reference"`
	CheckedOn           calendar.Date `bun:"checked_on,type:date"`
	RecordedByAccountID *int64        `bun:"recorded_by_account_id"`
	UpdatedAt           time.Time     `bun:"updated_at,nullzero,default:current_timestamp"`
	CostBearerName      string        `bun:"cost_bearer_name,scanonly"`
}

func newCareCostShareRow(organizationID, accountID int64, input parentbilling.CareCostShareInput) *careCostShareRow {
	return &careCostShareRow{
		OrganizationID: organizationID, CareContractID: input.ContractID, Kind: input.Kind, CostBearerID: input.CostBearerID,
		ShareBasisPoints: input.ShareBasisPoints, ShareCents: input.ShareCents, ValidFrom: input.ValidFrom,
		ValidUntil: input.ValidUntil, Reference: input.Reference, CheckedOn: input.CheckedOn, RecordedByAccountID: &accountID,
	}
}

// CareCostShares reads the shares of the contracts of the Träger with the
// name of their cost bearer, all of them when contractIDs is nil, by
// contract, kind and start.
func (s *Store) CareCostShares(ctx context.Context, organizationID int64, contractIDs []int64) ([]parentbilling.CareCostShare, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if contractIDs != nil && len(contractIDs) == 0 {
		return []parentbilling.CareCostShare{}, nil
	}
	var rows []careCostShareRow
	query := db.NewSelect().Model(&rows).ColumnExpr("share.*").ColumnExpr("bearer.name AS cost_bearer_name").
		Join("JOIN parent_billing.cost_bearers AS bearer ON bearer.organization_id = share.organization_id AND bearer.id = share.cost_bearer_id").
		Where("share.organization_id = ?", organizationID)
	if contractIDs != nil {
		query = query.Where("share.care_contract_id = ANY(?)", pgdialect.Array(contractIDs))
	}
	if err := query.OrderExpr("share.care_contract_id, share.kind, share.valid_from, share.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care cost shares: %w", err)
	}
	shares := make([]parentbilling.CareCostShare, len(rows))
	for i, row := range rows {
		shares[i] = parentbilling.CareCostShare{
			ID: row.ID, ContractID: row.CareContractID, Kind: row.Kind, CostBearerID: row.CostBearerID,
			CostBearerName: row.CostBearerName, ShareBasisPoints: row.ShareBasisPoints, ShareCents: row.ShareCents,
			ValidFrom: row.ValidFrom, ValidUntil: row.ValidUntil, Reference: row.Reference, CheckedOn: row.CheckedOn,
			UpdatedAt: row.UpdatedAt,
		}
	}
	return shares, nil
}

// InsertCareCostShare stores a new share.
func (s *Store) InsertCareCostShare(ctx context.Context, organizationID, accountID int64, input parentbilling.CareCostShareInput) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	row := newCareCostShareRow(organizationID, accountID, input)
	if _, err := db.NewInsert().Model(row).ExcludeColumn("cost_bearer_name").Returning("id").Exec(ctx); err != nil {
		return 0, costShareWriteError(err)
	}
	return row.ID, nil
}

// UpdateCareCostShare replaces a share of the Träger and answers its
// contract. The contract of a share never changes.
func (s *Store) UpdateCareCostShare(ctx context.Context, organizationID, accountID, shareID int64, input parentbilling.CareCostShareInput) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	row := newCareCostShareRow(organizationID, accountID, input)
	if err := db.NewUpdate().Model(row).
		Column("kind", "cost_bearer_id", "share_basis_points", "share_cents", "valid_from", "valid_until", "reference",
			"checked_on", "recorded_by_account_id").
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("id = ?", shareID).Where("organization_id = ?", organizationID).Where("care_contract_id = ?", input.ContractID).
		Returning("care_contract_id").Scan(ctx, &row.CareContractID); err != nil {
		if isNoRows(err) {
			return 0, parentbilling.ErrCareCostShareNotFound
		}
		return 0, costShareWriteError(err)
	}
	return row.CareContractID, nil
}

// DeleteCareCostShare removes a share of the Träger and answers its
// contract.
func (s *Store) DeleteCareCostShare(ctx context.Context, organizationID, shareID int64) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	var contractID int64
	if err := db.NewDelete().TableExpr("parent_billing.care_cost_shares").
		Where("id = ?", shareID).Where("organization_id = ?", organizationID).
		Returning("care_contract_id").Scan(ctx, &contractID); err != nil {
		if isNoRows(err) {
			return 0, parentbilling.ErrCareCostShareNotFound
		}
		return 0, fmt.Errorf("delete care cost share: %w", err)
	}
	return contractID, nil
}

// costShareWriteError names a contract or a cost bearer of another Träger
// and an overlap.
func costShareWriteError(err error) error {
	switch constraintOf(err) {
	case careCostShareContractKey:
		return validation.Errors{"contract_id": errors.New("is no contract of the Träger")}
	case careCostShareBearerKey:
		return validation.Errors{"cost_bearer_id": errors.New("must name a cost bearer of the Träger")}
	case careCostShareOverlap:
		return parentbilling.ErrCareCostShareOverlap
	}
	return fmt.Errorf("write care cost share: %w", err)
}

type careCostShareEventRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_cost_share_events"`

	ID             int64  `bun:"id,pk,autoincrement"`
	OrganizationID int64  `bun:"organization_id"`
	CareContractID int64  `bun:"care_contract_id"`
	ShareID        *int64 `bun:"share_id"`
	Action         string `bun:"action"`
	ActorAccountID *int64 `bun:"actor_account_id"`
}

// InsertCareCostShareEvents records one action per contract in one
// statement.
func (s *Store) InsertCareCostShareEvents(ctx context.Context, organizationID, accountID int64, action string, shareID *int64, contractIDs []int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(contractIDs) == 0 {
		return nil
	}
	rows := make([]careCostShareEventRow, len(contractIDs))
	for i, contractID := range contractIDs {
		rows[i] = careCostShareEventRow{OrganizationID: organizationID, CareContractID: contractID, ShareID: shareID,
			Action: action, ActorAccountID: &accountID}
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("insert care cost share events: %w", err)
	}
	return nil
}

type careInvoiceCostShareRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_invoice_cost_shares"`

	ID               int64  `bun:"id,pk,autoincrement"`
	OrganizationID   int64  `bun:"organization_id"`
	InvoiceID        int64  `bun:"invoice_id"`
	Position         int    `bun:"position"`
	Kind             string `bun:"kind"`
	CostBearerID     int64  `bun:"cost_bearer_id"`
	CostBearerName   string `bun:"cost_bearer_name"`
	ShareBasisPoints *int   `bun:"share_basis_points"`
	ShareCents       *int64 `bun:"share_cents"`
	AmountCents      int64  `bun:"amount_cents"`
}

// insertCareCostShares stores the shares of the invoices just issued; rows
// run parallel to invoices.
func insertCareCostShares(ctx context.Context, db bun.IDB, organizationID int64, rows []careInvoiceRow, invoices []parentbilling.CareInvoice) error {
	var shares []careInvoiceCostShareRow
	for i, invoice := range invoices {
		for j, share := range invoice.CostShares {
			shares = append(shares, careInvoiceCostShareRow{
				OrganizationID: organizationID, InvoiceID: rows[i].ID, Position: j + 1, Kind: share.Kind,
				CostBearerID: share.CostBearerID, CostBearerName: share.CostBearerName,
				ShareBasisPoints: share.ShareBasisPoints, ShareCents: share.ShareCents, AmountCents: share.AmountCents,
			})
		}
	}
	if len(shares) == 0 {
		return nil
	}
	if _, err := db.NewInsert().Model(&shares).Exec(ctx); err != nil {
		return fmt.Errorf("insert care invoice cost shares: %w", err)
	}
	return nil
}

// careCostSharesOf reads the shares of the invoices just read under the
// Träger's filter, by invoice and position.
func careCostSharesOf(ctx context.Context, db bun.IDB, organizationID int64, invoiceIDs []int64) (map[int64][]parentbilling.CareInvoiceCostShare, error) {
	var rows []careInvoiceCostShareRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		Where("invoice_id = ANY(?)", pgdialect.Array(invoiceIDs)).OrderExpr("invoice_id, position").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care invoice cost shares: %w", err)
	}
	byInvoice := map[int64][]parentbilling.CareInvoiceCostShare{}
	for _, row := range rows {
		byInvoice[row.InvoiceID] = append(byInvoice[row.InvoiceID], parentbilling.CareInvoiceCostShare{
			Kind: row.Kind, CostBearerID: row.CostBearerID, CostBearerName: row.CostBearerName,
			ShareBasisPoints: row.ShareBasisPoints, ShareCents: row.ShareCents, AmountCents: row.AmountCents,
		})
	}
	return byInvoice, nil
}
