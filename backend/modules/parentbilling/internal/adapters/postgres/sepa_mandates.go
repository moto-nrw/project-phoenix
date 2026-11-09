package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The creditor of a Träger and the SEPA mandates of its families (#4695,
// ADR 0171). Every statement runs in the Träger transaction and names the
// Träger in its own filter. The sealed IBAN leaves this file only towards
// the debit file (SealedSEPAMandates); every other read leaves it out.

// Constraints whose violation a caller can act on.
const (
	sepaMandateReferenceConstraint = "uq_sepa_mandates_reference"
	sepaMandateInForceConstraint   = "uq_sepa_mandates_in_force"
)

type sepaCreditorRow struct {
	bun.BaseModel `bun:"table:parent_billing.sepa_creditors"`

	OrganizationID      int64     `bun:"organization_id,pk" json:"-"`
	CreditorID          string    `bun:"creditor_id" json:"creditor_id"`
	PrenotificationDays int       `bun:"prenotification_days" json:"prenotification_days"`
	MissingMandate      string    `bun:"missing_mandate" json:"missing_mandate"`
	UpdatedByAccountID  *int64    `bun:"updated_by_account_id" json:"-"`
	UpdatedAt           time.Time `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
}

func (r sepaCreditorRow) creditor() parentbilling.SEPACreditor {
	updatedAt := r.UpdatedAt
	return parentbilling.SEPACreditor{CreditorID: r.CreditorID, PrenotificationDays: r.PrenotificationDays,
		MissingMandate: r.MissingMandate, UpdatedAt: &updatedAt}
}

// sepaMandateRow is a mandate. Every read but SealedSEPAMandates leaves
// the sealed IBAN out (mandateColumns).
type sepaMandateRow struct {
	bun.BaseModel `bun:"table:parent_billing.sepa_mandates,alias:mandate"`

	ID                 int64          `bun:"id,pk,autoincrement" json:"id"`
	OrganizationID     int64          `bun:"organization_id" json:"-"`
	FamilyEmail        string         `bun:"family_email" json:"family_email"`
	CareContractID     int64          `bun:"care_contract_id" json:"care_contract_id"`
	Reference          string         `bun:"reference" json:"reference"`
	SignedOn           calendar.Date  `bun:"signed_on,type:date" json:"signed_on"`
	Kind               string         `bun:"kind" json:"kind"`
	Sequence           string         `bun:"sequence" json:"sequence"`
	CreditorID         string         `bun:"creditor_id" json:"creditor_id"`
	HolderSealed       []byte         `bun:"account_holder_sealed" json:"-"`
	LastCollectedOn    *calendar.Date `bun:"last_collected_on,type:date" json:"last_collected_on"`
	IBANMasked         string         `bun:"iban_masked" json:"iban_masked"`
	BIC                string         `bun:"bic" json:"bic"`
	Source             string         `bun:"source" json:"source"`
	RevokedOn          *calendar.Date `bun:"revoked_on,type:date" json:"revoked_on"`
	RevokedByAccountID *int64         `bun:"revoked_by_account_id" json:"-"`
	RevokedAt          *time.Time     `bun:"revoked_at" json:"revoked_at"`
	CreatedByAccountID *int64         `bun:"created_by_account_id" json:"-"`
	CreatedAt          time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedByAccountID *int64         `bun:"updated_by_account_id" json:"-"`
	UpdatedAt          time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
	IBANSealed         []byte         `bun:"iban_sealed" json:"-"`
	// CreditorCurrent tells whether the mandate is given to the Träger's
	// creditor identifier now; read with the row.
	CreditorCurrent bool `bun:"creditor_current,scanonly" json:"-"`
}

func (r sepaMandateRow) mandate() parentbilling.SEPAMandate {
	return parentbilling.SEPAMandate{
		ID: r.ID, ContractID: r.CareContractID, FamilyEmail: r.FamilyEmail, Reference: r.Reference, SignedOn: r.SignedOn,
		Kind: r.Kind, Sequence: r.Sequence, CreditorID: r.CreditorID, CreditorCurrent: r.CreditorCurrent,
		LastCollectedOn: r.LastCollectedOn, HolderSealed: r.HolderSealed, IBANMasked: r.IBANMasked, BIC: r.BIC,
		Source: r.Source, RevokedOn: r.RevokedOn, RevokedAt: r.RevokedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// mandateReturning lists the columns of a mandate without the sealed IBAN.
const mandateReturning = "id, organization_id, family_email, care_contract_id, reference, signed_on, kind, sequence, " +
	"creditor_id, account_holder_sealed, last_collected_on, iban_masked, bic, source, revoked_on, revoked_by_account_id, " +
	"revoked_at, created_by_account_id, created_at, updated_by_account_id, updated_at"

// mandateCreditorCurrent compares the creditor a mandate was given to with
// the Träger's creditor identifier now, in the statement that reads it.
const mandateCreditorCurrent = `mandate.creditor_id = COALESCE((SELECT creditor.creditor_id FROM parent_billing.sepa_creditors AS creditor
	WHERE creditor.organization_id = mandate.organization_id), '') AS creditor_current`

// mandateColumns are the columns of mandateReturning.
var mandateColumns = strings.Split(mandateReturning, ", ")

type sepaMandateEventRow struct {
	bun.BaseModel `bun:"table:parent_billing.sepa_mandate_events"`

	ID             int64     `bun:"id,pk,autoincrement"`
	OrganizationID int64     `bun:"organization_id"`
	MandateID      int64     `bun:"mandate_id"`
	CareContractID int64     `bun:"care_contract_id"`
	Action         string    `bun:"action"`
	Source         string    `bun:"source"`
	Fields         []string  `bun:"fields,array"`
	ActorAccountID *int64    `bun:"actor_account_id"`
	OccurredAt     time.Time `bun:"occurred_at,nullzero,default:current_timestamp"`
}

// SEPACreditor reads the creditor of the Träger, nil without one.
func (s *Store) SEPACreditor(ctx context.Context, organizationID int64) (*parentbilling.SEPACreditor, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []sepaCreditorRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Scan(ctx); err != nil {
		return nil, fmt.Errorf("read SEPA creditor: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	creditor := rows[0].creditor()
	return &creditor, nil
}

// SaveSEPACreditor inserts or replaces the creditor of the Träger.
func (s *Store) SaveSEPACreditor(ctx context.Context, organizationID, accountID int64, creditor parentbilling.SEPACreditor) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	row := &sepaCreditorRow{OrganizationID: organizationID, CreditorID: creditor.CreditorID,
		PrenotificationDays: creditor.PrenotificationDays, MissingMandate: creditor.MissingMandate,
		UpdatedByAccountID: accountRef(accountID), UpdatedAt: time.Now()}
	if _, err := db.NewInsert().Model(row).
		On("CONFLICT (organization_id) DO UPDATE").
		Set("creditor_id = EXCLUDED.creditor_id").Set("prenotification_days = EXCLUDED.prenotification_days").
		Set("missing_mandate = EXCLUDED.missing_mandate").Set("updated_by_account_id = EXCLUDED.updated_by_account_id").
		Set("updated_at = CURRENT_TIMESTAMP").Exec(ctx); err != nil {
		return fmt.Errorf("save SEPA creditor: %w", err)
	}
	return nil
}

// SEPAMandates reads the mandates of one family or of all, the one in force
// first, then the newest.
func (s *Store) SEPAMandates(ctx context.Context, organizationID int64, familyEmail string) ([]parentbilling.SEPAMandate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []sepaMandateRow
	query := db.NewSelect().Model(&rows).Column(mandateColumns...).ColumnExpr(mandateCreditorCurrent).
		Where("organization_id = ?", organizationID)
	if familyEmail != "" {
		query = query.Where("family_email = ?", familyEmail)
	}
	if err := query.OrderExpr("revoked_on IS NOT NULL, signed_on DESC, id DESC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read SEPA mandates: %w", err)
	}
	mandates := make([]parentbilling.SEPAMandate, len(rows))
	for i, row := range rows {
		mandates[i] = row.mandate()
	}
	return mandates, nil
}

// SEPAMandate reads one mandate, locked for a change when lock is set.
func (s *Store) SEPAMandate(ctx context.Context, organizationID, id int64, lock bool) (parentbilling.SEPAMandate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	row := &sepaMandateRow{}
	query := db.NewSelect().Model(row).Column(mandateColumns...).Where("organization_id = ?", organizationID).Where("id = ?", id)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateNotFound
		}
		return parentbilling.SEPAMandate{}, fmt.Errorf("read SEPA mandate: %w", err)
	}
	return row.mandate(), nil
}

// InsertSEPAMandate stores a new mandate with its sealed IBAN.
func (s *Store) InsertSEPAMandate(ctx context.Context, organizationID, accountID int64, mandate parentbilling.SealedSEPAMandate) (parentbilling.SEPAMandate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	m := mandate.Mandate
	row := &sepaMandateRow{
		OrganizationID: organizationID, FamilyEmail: m.FamilyEmail, CareContractID: m.ContractID, Reference: m.Reference,
		SignedOn: m.SignedOn, Kind: m.Kind, Sequence: m.Sequence, CreditorID: m.CreditorID, HolderSealed: mandate.Holder,
		IBANMasked: m.IBANMasked, LastCollectedOn: m.LastCollectedOn,
		BIC: m.BIC, Source: m.Source, CreatedByAccountID: accountRef(accountID), UpdatedByAccountID: accountRef(accountID),
		IBANSealed: mandate.IBAN,
	}
	if _, err := db.NewInsert().Model(row).Returning(mandateReturning).Exec(ctx); err != nil {
		switch constraintOf(err) {
		case sepaMandateReferenceConstraint:
			return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateReferenceTaken
		case sepaMandateInForceConstraint:
			return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateInForce
		}
		return parentbilling.SEPAMandate{}, fmt.Errorf("insert SEPA mandate: %w", err)
	}
	return row.mandate(), nil
}

// UpdateSEPAMandate stores the account of a mandate; a nil IBAN keeps the
// sealed one.
func (s *Store) UpdateSEPAMandate(ctx context.Context, organizationID, accountID int64, mandate parentbilling.SealedSEPAMandate) (parentbilling.SEPAMandate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	m := mandate.Mandate
	row := &sepaMandateRow{}
	query := db.NewUpdate().Model(row).Where("organization_id = ?", organizationID).Where("id = ?", m.ID).
		Set("bic = ?", m.BIC).Set("sequence = ?", m.Sequence).
		Set("updated_by_account_id = ?", accountRef(accountID)).Set("updated_at = CURRENT_TIMESTAMP")
	if mandate.IBAN != nil {
		query = query.Set("iban_sealed = ?", mandate.IBAN).Set("iban_masked = ?", m.IBANMasked)
	}
	if mandate.Holder != nil {
		query = query.Set("account_holder_sealed = ?", mandate.Holder)
	}
	if err := query.Returning(mandateReturning).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateNotFound
		}
		return parentbilling.SEPAMandate{}, fmt.Errorf("update SEPA mandate: %w", err)
	}
	return row.mandate(), nil
}

// RevokeSEPAMandate ends a mandate in force.
func (s *Store) RevokeSEPAMandate(ctx context.Context, organizationID, accountID, id int64, revokedOn calendar.Date) (parentbilling.SEPAMandate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.SEPAMandate{}, err
	}
	row := &sepaMandateRow{}
	if err := db.NewUpdate().Model(row).Where("organization_id = ?", organizationID).Where("id = ?", id).
		Where("revoked_on IS NULL").
		Set("revoked_on = ?", revokedOn).Set("revoked_at = CURRENT_TIMESTAMP").Set("revoked_by_account_id = ?", accountRef(accountID)).
		Set("updated_by_account_id = ?", accountRef(accountID)).Set("updated_at = CURRENT_TIMESTAMP").
		Returning(mandateReturning).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.SEPAMandate{}, parentbilling.ErrSEPAMandateNotFound
		}
		return parentbilling.SEPAMandate{}, fmt.Errorf("revoke SEPA mandate: %w", err)
	}
	return row.mandate(), nil
}

// InsertSEPAMandateEvents records one action for each mandate in one
// statement.
func (s *Store) InsertSEPAMandateEvents(ctx context.Context, organizationID, accountID int64, action, source string, fields []string,
	mandates []parentbilling.SEPAMandate,
) error {
	if len(mandates) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if fields == nil {
		fields = []string{}
	}
	rows := make([]sepaMandateEventRow, len(mandates))
	for i, mandate := range mandates {
		rows[i] = sepaMandateEventRow{OrganizationID: organizationID, MandateID: mandate.ID, CareContractID: mandate.ContractID,
			Action: action, Source: source, Fields: fields, ActorAccountID: accountRef(accountID)}
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("record SEPA mandate: %w", err)
	}
	return nil
}

// SEPAMandateEvents reads the trail of a mandate, oldest first.
func (s *Store) SEPAMandateEvents(ctx context.Context, organizationID, id int64) ([]parentbilling.SEPAMandateEvent, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []sepaMandateEventRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("mandate_id = ?", id).
		OrderExpr("occurred_at, id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read SEPA mandate trail: %w", err)
	}
	events := make([]parentbilling.SEPAMandateEvent, len(rows))
	for i, row := range rows {
		events[i] = parentbilling.SEPAMandateEvent{ID: row.ID, Action: row.Action, Source: row.Source, Fields: row.Fields,
			AccountID: row.ActorAccountID, OccurredAt: row.OccurredAt}
		if events[i].Fields == nil {
			events[i].Fields = []string{}
		}
	}
	return events, nil
}

// SealedSEPAMandates reads mandates of the Träger with their sealed IBAN.
func (s *Store) SealedSEPAMandates(ctx context.Context, organizationID int64, ids []int64) ([]parentbilling.SealedSEPAMandate, error) {
	if len(ids) == 0 {
		return []parentbilling.SealedSEPAMandate{}, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []sepaMandateRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("id IN (?)", bun.List(ids)).
		OrderExpr("id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read sealed SEPA mandates: %w", err)
	}
	sealed := make([]parentbilling.SealedSEPAMandate, len(rows))
	for i, row := range rows {
		sealed[i] = parentbilling.SealedSEPAMandate{Mandate: row.mandate(), IBAN: row.IBANSealed, Holder: row.HolderSealed}
	}
	return sealed, nil
}

// invoiceDebitTermsSQL reads the creditor and the mandates in force of a
// Träger in one statement, each part under the Träger's own filter; the
// sealed IBAN stays out.
const invoiceDebitTermsSQL = `SELECT
	(SELECT to_jsonb(creditor) FROM (SELECT creditor_id, prenotification_days, missing_mandate, updated_at
		FROM parent_billing.sepa_creditors WHERE organization_id = ?0) AS creditor) AS creditor,
	(SELECT COALESCE(jsonb_agg(to_jsonb(mandate) ORDER BY mandate.id), '[]'::jsonb)
		FROM (SELECT id, family_email, care_contract_id, reference, signed_on, kind, sequence, creditor_id, last_collected_on,
				iban_masked, bic, source, revoked_on, revoked_at, created_at, updated_at
			FROM parent_billing.sepa_mandates WHERE organization_id = ?0 AND revoked_on IS NULL) AS mandate) AS mandates`

// InvoiceDebitTerms reads the creditor of the Träger (nil without one) and
// its mandates in force.
func (s *Store) InvoiceDebitTerms(ctx context.Context, organizationID int64) (*parentbilling.SEPACreditor, []parentbilling.SEPAMandate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, nil, err
	}
	var raw struct {
		Creditor json.RawMessage `bun:"creditor"`
		Mandates json.RawMessage `bun:"mandates"`
	}
	if err := db.NewRaw(invoiceDebitTermsSQL, organizationID).Scan(ctx, &raw); err != nil {
		return nil, nil, fmt.Errorf("read SEPA debit terms: %w", err)
	}
	var creditor *parentbilling.SEPACreditor
	if len(raw.Creditor) > 0 && string(raw.Creditor) != "null" {
		var row sepaCreditorRow
		if err := json.Unmarshal(raw.Creditor, &row); err != nil {
			return nil, nil, fmt.Errorf("read SEPA creditor: %w", err)
		}
		value := row.creditor()
		creditor = &value
	}
	var rows []sepaMandateRow
	if err := json.Unmarshal(raw.Mandates, &rows); err != nil {
		return nil, nil, fmt.Errorf("read SEPA mandates: %w", err)
	}
	mandates := make([]parentbilling.SEPAMandate, len(rows))
	for i, row := range rows {
		mandates[i] = row.mandate()
	}
	return creditor, mandates, nil
}
