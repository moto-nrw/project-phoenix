package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The children of a family (#5918, 1.15.812): one row per family of the
// Träger with the number the parents stated and the one the Abrechnung
// confirmed, and the trail of every read and change per contract. Every
// statement names the Träger in its own filter, also below row level
// security.

// familyChildCountColumns are the columns a row reads back.
const familyChildCountColumns = "id, organization_id, family_email, care_contract_id, stated_children, stated_at, " +
	"confirmed_children, valid_from, valid_until, confirmed_at, updated_at"

// familyChildContractKey keeps a number to a contract of its Träger.
const familyChildContractKey = "fk_family_child_counts_contract"

type familyChildCountRow struct {
	bun.BaseModel `bun:"table:parent_billing.family_child_counts"`

	ID                int64          `bun:"id,pk,autoincrement"`
	OrganizationID    int64          `bun:"organization_id"`
	FamilyEmail       string         `bun:"family_email"`
	ContractID        int64          `bun:"care_contract_id"`
	StatedChildren    *int           `bun:"stated_children"`
	StatedAt          *time.Time     `bun:"stated_at"`
	ConfirmedChildren *int           `bun:"confirmed_children"`
	ValidFrom         *calendar.Date `bun:"valid_from,type:date"`
	ValidUntil        *calendar.Date `bun:"valid_until,type:date"`
	ConfirmedAt       *time.Time     `bun:"confirmed_at"`
	UpdatedAt         time.Time      `bun:"updated_at"`
}

func (r familyChildCountRow) count() parentbilling.FamilyChildCount {
	return parentbilling.FamilyChildCount{
		ID: r.ID, ContractID: r.ContractID, StatedChildren: r.StatedChildren, StatedAt: r.StatedAt,
		ConfirmedChildren: r.ConfirmedChildren, ValidFrom: r.ValidFrom, ValidUntil: r.ValidUntil,
		ConfirmedAt: r.ConfirmedAt, UpdatedAt: r.UpdatedAt, FamilyEmail: r.FamilyEmail,
	}
}

// FamilyChildCounts reads the numbers of the Träger's families by family.
func (s *Store) FamilyChildCounts(ctx context.Context, organizationID int64) ([]parentbilling.FamilyChildCount, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []familyChildCountRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		OrderExpr("family_email").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read family child counts: %w", err)
	}
	counts := make([]parentbilling.FamilyChildCount, len(rows))
	for i, row := range rows {
		counts[i] = row.count()
	}
	return counts, nil
}

// FamilyChildCount reads the number of one family; found is false without
// one.
func (s *Store) FamilyChildCount(ctx context.Context, organizationID int64, family string) (parentbilling.FamilyChildCount, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.FamilyChildCount{}, false, err
	}
	var row familyChildCountRow
	err = db.NewSelect().Model(&row).Where("organization_id = ?", organizationID).Where("family_email = ?", family).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return parentbilling.FamilyChildCount{}, false, nil
	}
	if err != nil {
		return parentbilling.FamilyChildCount{}, false, fmt.Errorf("read family child count: %w", err)
	}
	return row.count(), true, nil
}

// StateFamilyChildren stores the number the parents stated for their
// family on a contract; a confirmed number stays as it is.
func (s *Store) StateFamilyChildren(ctx context.Context, organizationID, accountID int64, family string, contractID int64, children int) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.NewRaw(`INSERT INTO parent_billing.family_child_counts
			(organization_id, family_email, care_contract_id, stated_children, stated_at, stated_by_account_id)
		VALUES (?0, ?1, ?2, ?3, CURRENT_TIMESTAMP, ?4)
		ON CONFLICT (organization_id, family_email) DO UPDATE SET care_contract_id = EXCLUDED.care_contract_id,
			stated_children = EXCLUDED.stated_children, stated_at = EXCLUDED.stated_at,
			stated_by_account_id = EXCLUDED.stated_by_account_id, updated_at = CURRENT_TIMESTAMP
		RETURNING id`, organizationID, family, contractID, children, accountOrNull(accountID)).Scan(ctx, &id)
	return id, familyChildWriteError(err)
}

// ConfirmFamilyChildren stores the number the Abrechnung confirmed for the
// family of a contract; what the parents stated stays as it is.
func (s *Store) ConfirmFamilyChildren(ctx context.Context, organizationID, accountID int64, family string, input parentbilling.FamilyChildrenInput) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.NewRaw(`INSERT INTO parent_billing.family_child_counts
			(organization_id, family_email, care_contract_id, confirmed_children, valid_from, valid_until, confirmed_at, confirmed_by_account_id)
		VALUES (?0, ?1, ?2, ?3, ?4, ?5, CURRENT_TIMESTAMP, ?6)
		ON CONFLICT (organization_id, family_email) DO UPDATE SET care_contract_id = EXCLUDED.care_contract_id,
			confirmed_children = EXCLUDED.confirmed_children, valid_from = EXCLUDED.valid_from, valid_until = EXCLUDED.valid_until,
			confirmed_at = EXCLUDED.confirmed_at, confirmed_by_account_id = EXCLUDED.confirmed_by_account_id,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id`, organizationID, family, input.ContractID, input.Children, input.ValidFrom, input.ValidUntil, accountOrNull(accountID)).
		Scan(ctx, &id)
	return id, familyChildWriteError(err)
}

// accountOrNull binds no account as NULL.
func accountOrNull(accountID int64) *int64 {
	if accountID <= 0 {
		return nil
	}
	return &accountID
}

// familyChildWriteError reports a contract of another Träger as a missing
// family.
func familyChildWriteError(err error) error {
	switch {
	case err == nil:
		return nil
	case constraintOf(err) == familyChildContractKey:
		return parentbilling.ErrFamilyChildrenNotFound
	}
	return fmt.Errorf("store family child count: %w", err)
}

// DeleteFamilyChildren removes the number of a family and answers it.
func (s *Store) DeleteFamilyChildren(ctx context.Context, organizationID int64, family string) (parentbilling.FamilyChildCount, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.FamilyChildCount{}, err
	}
	var rows []familyChildCountRow
	if err := db.NewDelete().Model(&rows).Where("organization_id = ?", organizationID).Where("family_email = ?", family).
		Returning(familyChildCountColumns).Scan(ctx); err != nil {
		return parentbilling.FamilyChildCount{}, fmt.Errorf("delete family child count: %w", err)
	}
	if len(rows) == 0 {
		return parentbilling.FamilyChildCount{}, parentbilling.ErrFamilyChildrenNotFound
	}
	return rows[0].count(), nil
}

// InsertFamilyChildEvents records one action per target in one statement:
// a read names the number only by its row, a change also carries value and
// validity.
func (s *Store) InsertFamilyChildEvents(ctx context.Context, organizationID int64, event parentbilling.FamilyChildEvent, targets []parentbilling.FamilyChildTarget) error {
	if len(targets) == 0 {
		return nil
	}
	contractIDs := make([]int64, len(targets))
	// A target without a number travels as 0 and lands as NULL.
	countIDs := make([]int64, len(targets))
	for i, target := range targets {
		contractIDs[i] = target.ContractID
		if target.CountID != nil {
			countIDs[i] = *target.CountID
		}
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	account := accountOrNull(event.AccountID)
	if _, err := db.ExecContext(ctx, `INSERT INTO parent_billing.family_child_count_events
			(organization_id, care_contract_id, family_child_count_id, action, actor, actor_account_id, children, valid_from, valid_until)
		SELECT ?, target.contract_id, NULLIF(target.count_id, 0), ?, ?, ?, ?, ?, ?
		FROM unnest(?::bigint[], ?::bigint[]) AS target(contract_id, count_id)`,
		organizationID, event.Action, event.Actor, account, event.Children, event.ValidFrom, event.ValidUntil,
		pgdialect.Array(contractIDs), pgdialect.Array(countIDs)); err != nil {
		return fmt.Errorf("record family child count event: %w", err)
	}
	return nil
}
