// Package postgres persists the rules and the frozen holiday contributions of
// the Elternabrechnung. Its tables carry organization_id and a Träger policy
// on app.current_org_id (ADR 0086 section 5, ADR 0105): every statement here
// runs in the Träger transaction of the Träger it names, and names it once
// more in its own filter.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ruleOverlapConstraint is the exclusion constraint that keeps two rules of
// one kind from being valid at the same time.
const ruleOverlapConstraint = "excl_contribution_rules_overlap"

// Store persists the Elternabrechnung of a Träger in its Träger transaction.
type Store struct{}

// NewStore builds the store.
func NewStore() *Store { return &Store{} }

// traegerDB returns the Träger transaction of organizationID. A school,
// administrative or foreign Träger transaction is refused.
func traegerDB(ctx context.Context, organizationID int64) (bun.IDB, error) {
	if organizationID <= 0 || tenant.TraegerTransactionFromContext(ctx) != organizationID {
		return nil, errors.New("parent billing postgres: the Träger transaction of this Träger is required")
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("parent billing postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("parent billing postgres: unsupported transaction %T", transaction)
	}
	return tx, nil
}

type ruleRow struct {
	bun.BaseModel `bun:"table:parent_billing.contribution_rules"`

	ID                  int64          `bun:"id,pk,autoincrement"`
	OrganizationID      int64          `bun:"organization_id"`
	Service             string         `bun:"service"`
	Kind                string         `bun:"kind"`
	ValidFrom           calendar.Date  `bun:"valid_from,type:date"`
	ValidUntil          *calendar.Date `bun:"valid_until,type:date"`
	SiblingFromChild    *int           `bun:"sibling_from_child"`
	DiscountBasisPoints *int           `bun:"discount_basis_points"`
	DiscountCents       *int64         `bun:"discount_cents"`
	Note                string         `bun:"note"`
	UpdatedAt           time.Time      `bun:"updated_at,nullzero,default:current_timestamp"`
}

func (r ruleRow) rule() parentbilling.ContributionRule {
	return parentbilling.ContributionRule{
		ID: r.ID, Service: r.Service, Kind: r.Kind, ValidFrom: r.ValidFrom, ValidUntil: r.ValidUntil,
		SiblingFromChild: r.SiblingFromChild, DiscountBasisPoints: r.DiscountBasisPoints, DiscountCents: r.DiscountCents,
		Note: r.Note, UpdatedAt: r.UpdatedAt,
	}
}

func newRuleRow(organizationID int64, input parentbilling.ContributionRuleInput) *ruleRow {
	return &ruleRow{
		OrganizationID: organizationID, Service: input.Service, Kind: input.Kind, ValidFrom: input.ValidFrom,
		ValidUntil: input.ValidUntil, SiblingFromChild: input.SiblingFromChild, DiscountBasisPoints: input.DiscountBasisPoints,
		DiscountCents: input.DiscountCents, Note: input.Note,
	}
}

// ListContributionRules lists the rules of one service by kind and start.
func (s *Store) ListContributionRules(ctx context.Context, organizationID int64, service string) ([]parentbilling.ContributionRule, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []ruleRow
	if err := db.NewSelect().Model(&rows).
		Where("organization_id = ?", organizationID).Where("service = ?", service).
		OrderExpr("kind, valid_from, id").Scan(ctx); err != nil {
		return nil, err
	}
	rules := make([]parentbilling.ContributionRule, len(rows))
	for i, row := range rows {
		rules[i] = row.rule()
	}
	return rules, nil
}

// InsertContributionRule stores a new rule.
func (s *Store) InsertContributionRule(ctx context.Context, organizationID int64, input parentbilling.ContributionRuleInput) (parentbilling.ContributionRule, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.ContributionRule{}, err
	}
	row := newRuleRow(organizationID, input)
	if _, err := db.NewInsert().Model(row).Returning("id, updated_at").Exec(ctx); err != nil {
		return parentbilling.ContributionRule{}, ruleWriteError(err)
	}
	return row.rule(), nil
}

// UpdateContributionRule replaces the parameters and the validity of a rule.
func (s *Store) UpdateContributionRule(ctx context.Context, organizationID, ruleID int64, input parentbilling.ContributionRuleInput) (parentbilling.ContributionRule, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.ContributionRule{}, err
	}
	row := newRuleRow(organizationID, input)
	row.ID = ruleID
	result, err := db.NewUpdate().Model(row).
		Column("service", "kind", "valid_from", "valid_until", "sibling_from_child", "discount_basis_points", "discount_cents", "note").
		Where("id = ?", ruleID).Where("organization_id = ?", organizationID).
		Returning("updated_at").Exec(ctx)
	if err != nil {
		return parentbilling.ContributionRule{}, ruleWriteError(err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected == 0 {
		if err != nil {
			return parentbilling.ContributionRule{}, err
		}
		return parentbilling.ContributionRule{}, parentbilling.ErrContributionRuleNotFound
	}
	return row.rule(), nil
}

// DeleteContributionRule removes a rule.
func (s *Store) DeleteContributionRule(ctx context.Context, organizationID, ruleID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.NewDelete().TableExpr("parent_billing.contribution_rules").
		Where("id = ?", ruleID).Where("organization_id = ?", organizationID).Exec(ctx)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return parentbilling.ErrContributionRuleNotFound
	}
	return nil
}

// ruleWriteError names an overlap with another rule of the same kind.
func ruleWriteError(err error) error {
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) && pgErr.Field('n') == ruleOverlapConstraint {
		return parentbilling.ErrContributionRuleOverlap
	}
	return err
}
