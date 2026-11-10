package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The revenue accounts and cost centers of a Träger (#4821). Every query
// names the Träger, also below row level security.

type revenueAccountRow struct {
	bun.BaseModel `bun:"table:parent_billing.revenue_accounts"`

	ID                 int64     `bun:"id,pk,autoincrement"`
	OrganizationID     int64     `bun:"organization_id"`
	Scope              string    `bun:"scope"`
	ScopeID            int64     `bun:"scope_id"`
	Account            string    `bun:"account"`
	UpdatedByAccountID *int64    `bun:"updated_by_account_id"`
	UpdatedAt          time.Time `bun:"updated_at,nullzero,default:current_timestamp"`
}

type costCenterRow struct {
	bun.BaseModel `bun:"table:parent_billing.cost_centers"`

	ID                 int64         `bun:"id,pk,autoincrement"`
	OrganizationID     int64         `bun:"organization_id"`
	Scope              string        `bun:"scope"`
	ScopeID            int64         `bun:"scope_id"`
	ValidFrom          calendar.Date `bun:"valid_from,type:date"`
	Code               string        `bun:"code"`
	CreatedByAccountID *int64        `bun:"created_by_account_id"`
	CreatedAt          time.Time     `bun:"created_at,nullzero,default:current_timestamp"`
}

// costCenterScopeConstraint is the unique key of a scope and its day.
const costCenterScopeConstraint = "uq_cost_centers_scope"

// BookingAssignments reads every revenue account and cost center of the
// Träger.
func (s *Store) BookingAssignments(ctx context.Context, organizationID int64) (parentbilling.BookingAssignments, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.BookingAssignments{}, err
	}
	var accounts []revenueAccountRow
	if err := db.NewSelect().Model(&accounts).Where("organization_id = ?", organizationID).
		OrderExpr("scope, scope_id").Scan(ctx); err != nil {
		return parentbilling.BookingAssignments{}, fmt.Errorf("read revenue accounts: %w", err)
	}
	var centers []costCenterRow
	if err := db.NewSelect().Model(&centers).Where("organization_id = ?", organizationID).
		OrderExpr("scope, scope_id, valid_from").Scan(ctx); err != nil {
		return parentbilling.BookingAssignments{}, fmt.Errorf("read cost centers: %w", err)
	}
	out := parentbilling.BookingAssignments{
		RevenueAccounts: make([]parentbilling.RevenueAccount, len(accounts)),
		CostCenters:     make([]parentbilling.CostCenter, len(centers)),
	}
	for i, row := range accounts {
		out.RevenueAccounts[i] = parentbilling.RevenueAccount{Scope: row.Scope, ScopeID: row.ScopeID, Account: row.Account, UpdatedAt: row.UpdatedAt}
	}
	for i, row := range centers {
		out.CostCenters[i] = parentbilling.CostCenter{ID: row.ID, Scope: row.Scope, ScopeID: row.ScopeID, ValidFrom: row.ValidFrom,
			Code: row.Code, CreatedAt: row.CreatedAt}
	}
	return out, nil
}

type bookingOfferingRow struct {
	SchoolID       int64    `bun:"school_id"`
	CareOfferingID int64    `bun:"care_offering_id"`
	ModuleNames    []string `bun:"module_names,array"`
}

// BookingOfferings lists the Angebote the price lists of the Träger price,
// with their module names.
func (s *Store) BookingOfferings(ctx context.Context, organizationID int64) ([]parentbilling.BookingOffering, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []bookingOfferingRow
	if err := db.NewRaw(`SELECT tariff.school_id, tariff.care_offering_id,
			array_agg(DISTINCT tariff.module_name ORDER BY tariff.module_name) AS module_names
		FROM parent_billing.care_tariffs AS tariff
		WHERE tariff.organization_id = ?
		GROUP BY tariff.school_id, tariff.care_offering_id
		ORDER BY tariff.school_id, MIN(tariff.module_name), tariff.care_offering_id`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read the Angebote of the prices: %w", err)
	}
	offerings := make([]parentbilling.BookingOffering, len(rows))
	for i, row := range rows {
		offerings[i] = parentbilling.BookingOffering{SchoolID: row.SchoolID, CareOfferingID: row.CareOfferingID, ModuleNames: row.ModuleNames}
	}
	return offerings, nil
}

// SetRevenueAccount sets the account of a scope.
func (s *Store) SetRevenueAccount(ctx context.Context, organizationID, accountID int64, scope string, scopeID int64, account string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	row := &revenueAccountRow{OrganizationID: organizationID, Scope: scope, ScopeID: scopeID, Account: account, UpdatedByAccountID: &accountID}
	if _, err := db.NewInsert().Model(row).
		On("CONFLICT (organization_id, scope, scope_id) DO UPDATE").
		Set("account = EXCLUDED.account, updated_by_account_id = EXCLUDED.updated_by_account_id, updated_at = CURRENT_TIMESTAMP").
		Exec(ctx); err != nil {
		return fmt.Errorf("set revenue account: %w", err)
	}
	return nil
}

// DeleteRevenueAccount removes the account of a scope; none is no error.
func (s *Store) DeleteRevenueAccount(ctx context.Context, organizationID int64, scope string, scopeID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewDelete().Model((*revenueAccountRow)(nil)).Where("organization_id = ?", organizationID).
		Where("scope = ?", scope).Where("scope_id = ?", scopeID).Exec(ctx); err != nil {
		return fmt.Errorf("delete revenue account: %w", err)
	}
	return nil
}

// InsertCostCenter adds a cost center of a scope from its day on.
func (s *Store) InsertCostCenter(ctx context.Context, organizationID, accountID int64, center parentbilling.CostCenter) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	row := &costCenterRow{OrganizationID: organizationID, Scope: center.Scope, ScopeID: center.ScopeID, ValidFrom: center.ValidFrom,
		Code: center.Code, CreatedByAccountID: &accountID}
	if _, err := db.NewInsert().Model(row).Exec(ctx); err != nil {
		if constraintOf(err) == costCenterScopeConstraint {
			return parentbilling.ErrCostCenterDateTaken
		}
		return fmt.Errorf("insert cost center: %w", err)
	}
	return nil
}

// DeleteCostCenter removes a cost center of the Träger.
func (s *Store) DeleteCostCenter(ctx context.Context, organizationID, id int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.NewDelete().Model((*costCenterRow)(nil)).Where("organization_id = ?", organizationID).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete cost center: %w", err)
	}
	if deleted, err := result.RowsAffected(); err != nil {
		return err
	} else if deleted == 0 {
		return parentbilling.ErrCostCenterNotFound
	}
	return nil
}
