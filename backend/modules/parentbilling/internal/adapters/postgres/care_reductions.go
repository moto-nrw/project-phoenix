package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The rule chain of the monthly contribution (#4685, 1.15.807): the care
// rules in contribution_rules, the evidences of the contracts with their
// trail, and the reductions of an issued invoice. Every statement names the
// Träger in its own filter, also below row level security.

const (
	// careEvidenceContractKey keeps an evidence to a contract of its Träger.
	careEvidenceContractKey = "fk_care_evidences_contract"
	// careEvidenceRuleKey keeps an income level an evidence names.
	careEvidenceRuleKey = "fk_care_evidences_rule"
	// careEvidenceOverlap keeps one evidence of a kind per contract and day.
	careEvidenceOverlap = "excl_care_evidences_overlap"
)

func newCareRuleRow(organizationID int64, input parentbilling.CareRuleInput) *ruleRow {
	return &ruleRow{
		OrganizationID: organizationID, Service: parentbilling.ServiceCare, Kind: input.Kind, Name: input.Name,
		ValidFrom: input.ValidFrom, ValidUntil: input.ValidUntil, SchoolIDs: idsOrEmpty(input.SchoolIDs),
		CareOfferingIDs: idsOrEmpty(input.CareOfferingIDs), CountSchoolIDs: idsOrEmpty(input.CountSchoolIDs),
		SiblingFromChild: input.SiblingFromChild, DiscountBasisPoints: input.DiscountBasisPoints,
		DiscountCents: input.DiscountCents, PriceCents: input.PriceCents, CapCents: input.CapCents,
		CountsFamilyChildren: input.CountsFamilyChildren,
	}
}

// InsertCareRule stores a new care rule.
func (s *Store) InsertCareRule(ctx context.Context, organizationID int64, input parentbilling.CareRuleInput) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewInsert().Model(newCareRuleRow(organizationID, input)).Exec(ctx); err != nil {
		return fmt.Errorf("insert care rule: %w", err)
	}
	return nil
}

// careRuleColumns are the columns a care rule changes.
var careRuleColumns = []string{
	"kind", "name", "valid_from", "valid_until", "school_ids", "care_offering_ids", "count_school_ids",
	"sibling_from_child", "discount_basis_points", "discount_cents", "price_cents", "cap_cents",
	"counts_family_children",
}

// UpdateCareRule replaces a care rule of the Träger. A holiday rule is no
// care rule. A kind change of an income level an evidence names fails on
// the evidence's key.
func (s *Store) UpdateCareRule(ctx context.Context, organizationID, ruleID int64, input parentbilling.CareRuleInput) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if input.Kind != parentbilling.RuleIncomeLevel {
		if inUse, err := ruleNamedByEvidence(ctx, db, organizationID, ruleID); err != nil || inUse {
			if err != nil {
				return err
			}
			return parentbilling.ErrCareRuleInUse
		}
	}
	row := newCareRuleRow(organizationID, input)
	result, err := db.NewUpdate().Model(row).Column(careRuleColumns...).Set("updated_at = CURRENT_TIMESTAMP").
		Where("id = ?", ruleID).Where("organization_id = ?", organizationID).Where("service = ?", parentbilling.ServiceCare).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update care rule: %w", err)
	}
	return affectedOrNotFound(result, parentbilling.ErrContributionRuleNotFound)
}

func ruleNamedByEvidence(ctx context.Context, db bun.IDB, organizationID, ruleID int64) (bool, error) {
	return db.NewSelect().TableExpr("parent_billing.care_evidences").
		Where("organization_id = ?", organizationID).Where("rule_id = ?", ruleID).Exists(ctx)
}

// DeleteCareRule removes a care rule of the Träger.
func (s *Store) DeleteCareRule(ctx context.Context, organizationID, ruleID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.NewDelete().TableExpr("parent_billing.contribution_rules").
		Where("id = ?", ruleID).Where("organization_id = ?", organizationID).Where("service = ?", parentbilling.ServiceCare).
		Exec(ctx)
	if err != nil {
		if constraintOf(err) == careEvidenceRuleKey {
			return parentbilling.ErrCareRuleInUse
		}
		return fmt.Errorf("delete care rule: %w", err)
	}
	return affectedOrNotFound(result, parentbilling.ErrContributionRuleNotFound)
}

func affectedOrNotFound(result interface{ RowsAffected() (int64, error) }, notFound error) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return notFound
	}
	return nil
}

type careEvidenceRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_evidences"`

	ID                  int64          `bun:"id,pk,autoincrement"`
	OrganizationID      int64          `bun:"organization_id"`
	CareContractID      int64          `bun:"care_contract_id"`
	Kind                string         `bun:"kind"`
	RuleID              *int64         `bun:"rule_id"`
	ValidFrom           calendar.Date  `bun:"valid_from,type:date"`
	ValidUntil          *calendar.Date `bun:"valid_until,type:date"`
	Evidence            string         `bun:"evidence"`
	CheckedOn           calendar.Date  `bun:"checked_on,type:date"`
	RecordedByAccountID *int64         `bun:"recorded_by_account_id"`
	UpdatedAt           time.Time      `bun:"updated_at,nullzero,default:current_timestamp"`
}

func (r careEvidenceRow) evidence() parentbilling.CareEvidence {
	return parentbilling.CareEvidence{
		ID: r.ID, ContractID: r.CareContractID, Kind: r.Kind, RuleID: r.RuleID, ValidFrom: r.ValidFrom,
		ValidUntil: r.ValidUntil, Evidence: r.Evidence, CheckedOn: r.CheckedOn, UpdatedAt: r.UpdatedAt,
	}
}

func newCareEvidenceRow(organizationID, accountID int64, input parentbilling.CareEvidenceInput) *careEvidenceRow {
	return &careEvidenceRow{
		OrganizationID: organizationID, CareContractID: input.ContractID, Kind: input.Kind, RuleID: input.RuleID,
		ValidFrom: input.ValidFrom, ValidUntil: input.ValidUntil, Evidence: input.Evidence, CheckedOn: input.CheckedOn,
		RecordedByAccountID: &accountID,
	}
}

// CareEvidences reads the evidences of the contracts of the Träger, all of
// them when contractIDs is nil, by contract, kind and start.
func (s *Store) CareEvidences(ctx context.Context, organizationID int64, contractIDs []int64) ([]parentbilling.CareEvidence, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if contractIDs != nil && len(contractIDs) == 0 {
		return []parentbilling.CareEvidence{}, nil
	}
	var rows []careEvidenceRow
	query := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID)
	if contractIDs != nil {
		query = query.Where("care_contract_id = ANY(?)", pgdialect.Array(contractIDs))
	}
	if err := query.OrderExpr("care_contract_id, kind, valid_from, id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care evidences: %w", err)
	}
	evidences := make([]parentbilling.CareEvidence, len(rows))
	for i, row := range rows {
		evidences[i] = row.evidence()
	}
	return evidences, nil
}

// InsertCareEvidence stores a new evidence.
func (s *Store) InsertCareEvidence(ctx context.Context, organizationID, accountID int64, input parentbilling.CareEvidenceInput) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	row := newCareEvidenceRow(organizationID, accountID, input)
	if _, err := db.NewInsert().Model(row).Returning("id").Exec(ctx); err != nil {
		return 0, evidenceWriteError(err)
	}
	return row.ID, nil
}

// UpdateCareEvidence replaces an evidence of the Träger and answers its
// contract. The contract of an evidence never changes.
func (s *Store) UpdateCareEvidence(ctx context.Context, organizationID, accountID, evidenceID int64, input parentbilling.CareEvidenceInput) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	row := newCareEvidenceRow(organizationID, accountID, input)
	if err := db.NewUpdate().Model(row).
		Column("kind", "rule_id", "valid_from", "valid_until", "evidence", "checked_on", "recorded_by_account_id").
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("id = ?", evidenceID).Where("organization_id = ?", organizationID).Where("care_contract_id = ?", input.ContractID).
		Returning("care_contract_id").Scan(ctx, &row.CareContractID); err != nil {
		if isNoRows(err) {
			return 0, parentbilling.ErrCareEvidenceNotFound
		}
		return 0, evidenceWriteError(err)
	}
	return row.CareContractID, nil
}

// DeleteCareEvidence removes an evidence of the Träger and answers its
// contract.
func (s *Store) DeleteCareEvidence(ctx context.Context, organizationID, evidenceID int64) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	var contractID int64
	if err := db.NewDelete().TableExpr("parent_billing.care_evidences").
		Where("id = ?", evidenceID).Where("organization_id = ?", organizationID).
		Returning("care_contract_id").Scan(ctx, &contractID); err != nil {
		if isNoRows(err) {
			return 0, parentbilling.ErrCareEvidenceNotFound
		}
		return 0, fmt.Errorf("delete care evidence: %w", err)
	}
	return contractID, nil
}

// evidenceWriteError names a contract of another Träger and an overlap.
func evidenceWriteError(err error) error {
	switch constraintOf(err) {
	case careEvidenceContractKey:
		return validation.Errors{"contract_id": errors.New("is no contract of the Träger")}
	case careEvidenceRuleKey:
		return validation.Errors{"rule_id": errors.New("must name an income level of the Träger")}
	case careEvidenceOverlap:
		return parentbilling.ErrCareEvidenceOverlap
	}
	return fmt.Errorf("write care evidence: %w", err)
}

type careEvidenceEventRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_evidence_events"`

	ID             int64  `bun:"id,pk,autoincrement"`
	OrganizationID int64  `bun:"organization_id"`
	CareContractID int64  `bun:"care_contract_id"`
	EvidenceID     *int64 `bun:"evidence_id"`
	Action         string `bun:"action"`
	ActorAccountID *int64 `bun:"actor_account_id"`
}

// InsertCareEvidenceEvents records one action per contract in one
// statement.
func (s *Store) InsertCareEvidenceEvents(ctx context.Context, organizationID, accountID int64, action string, evidenceID *int64, contractIDs []int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(contractIDs) == 0 {
		return nil
	}
	rows := make([]careEvidenceEventRow, len(contractIDs))
	for i, contractID := range contractIDs {
		rows[i] = careEvidenceEventRow{OrganizationID: organizationID, CareContractID: contractID, EvidenceID: evidenceID,
			Action: action, ActorAccountID: &accountID}
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("insert care evidence events: %w", err)
	}
	return nil
}

type careInvoiceReductionRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_invoice_reductions"`

	ID                  int64   `bun:"id,pk,autoincrement"`
	OrganizationID      int64   `bun:"organization_id"`
	InvoiceID           int64   `bun:"invoice_id"`
	Position            int     `bun:"position"`
	Kind                string  `bun:"kind"`
	RuleID              int64   `bun:"rule_id"`
	Name                string  `bun:"name"`
	ChildRank           *int    `bun:"child_rank"`
	FamilyChildren      *int    `bun:"family_children"`
	DiscountBasisPoints *int    `bun:"discount_basis_points"`
	DiscountCents       *int64  `bun:"discount_cents"`
	PriceCents          *int64  `bun:"price_cents"`
	CapCents            *int64  `bun:"cap_cents"`
	CareOfferingIDs     []int64 `bun:"care_offering_ids,array"`
	AmountCents         int64   `bun:"amount_cents"`
}

// insertCareReductions stores the reductions of the invoices just issued;
// rows run parallel to invoices.
func insertCareReductions(ctx context.Context, db bun.IDB, organizationID int64, rows []careInvoiceRow, invoices []parentbilling.CareInvoice) error {
	var reductions []careInvoiceReductionRow
	for i, invoice := range invoices {
		for j, reduction := range invoice.Reductions {
			reductions = append(reductions, careInvoiceReductionRow{
				OrganizationID: organizationID, InvoiceID: rows[i].ID, Position: j + 1, Kind: reduction.Kind,
				RuleID: reduction.RuleID, Name: reduction.Name, ChildRank: reduction.ChildRank,
				FamilyChildren: reduction.FamilyChildren, DiscountBasisPoints: reduction.DiscountBasisPoints,
				DiscountCents: reduction.DiscountCents, PriceCents: reduction.PriceCents, CapCents: reduction.CapCents,
				CareOfferingIDs: idsOrEmpty(reduction.CareOfferingIDs), AmountCents: reduction.AmountCents,
			})
		}
	}
	if len(reductions) == 0 {
		return nil
	}
	if _, err := db.NewInsert().Model(&reductions).Exec(ctx); err != nil {
		return fmt.Errorf("insert care invoice reductions: %w", err)
	}
	return nil
}

// careReductionsOf reads the reductions of the invoices just read under the
// Träger's filter, by invoice and position.
func careReductionsOf(ctx context.Context, db bun.IDB, organizationID int64, invoiceIDs []int64) (map[int64][]parentbilling.CareReduction, error) {
	var rows []careInvoiceReductionRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		Where("invoice_id = ANY(?)", pgdialect.Array(invoiceIDs)).OrderExpr("invoice_id, position").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care invoice reductions: %w", err)
	}
	byInvoice := map[int64][]parentbilling.CareReduction{}
	for _, row := range rows {
		byInvoice[row.InvoiceID] = append(byInvoice[row.InvoiceID], parentbilling.CareReduction{
			Kind: row.Kind, RuleID: row.RuleID, Name: row.Name, ChildRank: row.ChildRank, FamilyChildren: row.FamilyChildren,
			DiscountBasisPoints: row.DiscountBasisPoints, DiscountCents: row.DiscountCents, PriceCents: row.PriceCents,
			CapCents: row.CapCents, CareOfferingIDs: idsOrEmpty(row.CareOfferingIDs), AmountCents: row.AmountCents,
		})
	}
	return byInvoice, nil
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
