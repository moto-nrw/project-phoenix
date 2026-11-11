package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The terms of the services, the deviations of the families and the due
// dates of the runs (#4689, ADR 0167). Every statement runs in the Träger
// transaction and names the Träger in its own filter.

type serviceTermsRow struct {
	bun.BaseModel `bun:"table:parent_billing.service_terms"`

	OrganizationID     int64     `bun:"organization_id,pk" json:"-"`
	Service            string    `bun:"service,pk" json:"service"`
	PaymentMethod      string    `bun:"payment_method" json:"payment_method"`
	DueDays            int       `bun:"due_days" json:"due_days"`
	BundleWith         *string   `bun:"bundle_with" json:"bundle_with"`
	UpdatedByAccountID *int64    `bun:"updated_by_account_id" json:"-"`
	UpdatedAt          time.Time `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
}

func (r serviceTermsRow) terms() parentbilling.ServiceTerms {
	updatedAt := r.UpdatedAt
	terms := parentbilling.ServiceTerms{Service: r.Service, PaymentMethod: r.PaymentMethod, DueDays: r.DueDays, UpdatedAt: &updatedAt}
	if r.BundleWith != nil {
		terms.BundleWith = *r.BundleWith
	}
	return terms
}

type familyBillingRow struct {
	bun.BaseModel `bun:"table:parent_billing.family_billing"`

	ID              int64   `bun:"id,pk,autoincrement" json:"id"`
	OrganizationID  int64   `bun:"organization_id" json:"-"`
	FamilyEmail     string  `bun:"family_email" json:"family_email"`
	CareContractID  *int64  `bun:"care_contract_id" json:"care_contract_id"`
	RecipientKind   string  `bun:"recipient_kind" json:"recipient_kind"`
	SecondFirstName string  `bun:"second_first_name" json:"second_first_name"`
	SecondLastName  string  `bun:"second_last_name" json:"second_last_name"`
	PayerFirstName  string  `bun:"payer_first_name" json:"payer_first_name"`
	PayerLastName   string  `bun:"payer_last_name" json:"payer_last_name"`
	PayerEmail      string  `bun:"payer_email" json:"payer_email"`
	PayerStreet     string  `bun:"payer_street" json:"payer_street"`
	PayerPostalCode string  `bun:"payer_postal_code" json:"payer_postal_code"`
	PayerCity       string  `bun:"payer_city" json:"payer_city"`
	PaymentMethod   *string `bun:"payment_method" json:"payment_method"`
	DueDay          *int    `bun:"due_day" json:"due_day"`
	DebtorNumber    *string `bun:"debtor_number" json:"debtor_number"`
	HolderFirstName string  `bun:"holder_first_name" json:"holder_first_name"`
	HolderLastName  string  `bun:"holder_last_name" json:"holder_last_name"`
	// LastUsedOn is set on a row of the holiday care (#6692); NULL on a
	// deviation set from a contract alone.
	LastUsedOn         *calendar.Date `bun:"last_used_on,type:date" json:"last_used_on"`
	UpdatedByAccountID *int64         `bun:"updated_by_account_id" json:"-"`
	UpdatedAt          time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
}

func (r familyBillingRow) family() parentbilling.FamilyBilling {
	family := parentbilling.FamilyBilling{
		ID: r.ID, FamilyEmail: r.FamilyEmail, ContractID: idOf(r.CareContractID), RecipientKind: r.RecipientKind,
		SecondFirstName: r.SecondFirstName, SecondLastName: r.SecondLastName, PayerFirstName: r.PayerFirstName,
		PayerLastName: r.PayerLastName, PayerEmail: r.PayerEmail, DueDay: r.DueDay, UpdatedAt: r.UpdatedAt,
		DebtorNumber: textOf(r.DebtorNumber), HolderFirstName: r.HolderFirstName, HolderLastName: r.HolderLastName,
		LastUsedOn: r.LastUsedOn,
	}
	if r.PaymentMethod != nil {
		family.PaymentMethod = *r.PaymentMethod
	}
	if r.PayerStreet != "" {
		family.PayerAddress = &parentbilling.PostalAddress{Street: r.PayerStreet, PostalCode: r.PayerPostalCode, City: r.PayerCity}
	}
	return family
}

type familyBillingEventRow struct {
	bun.BaseModel `bun:"table:parent_billing.family_billing_events"`

	ID              int64  `bun:"id,pk,autoincrement"`
	OrganizationID  int64  `bun:"organization_id"`
	FamilyBillingID int64  `bun:"family_billing_id"`
	CareContractID  *int64 `bun:"care_contract_id"`
	Action          string `bun:"action"`
	ActorAccountID  *int64 `bun:"actor_account_id"`
}

type runDueDateRow struct {
	bun.BaseModel `bun:"table:parent_billing.run_due_dates"`

	ID             int64         `bun:"id,pk,autoincrement"`
	OrganizationID int64         `bun:"organization_id"`
	CareRunID      *int64        `bun:"care_run_id"`
	HolidayRunID   *int64        `bun:"holiday_run_id"`
	PaymentMethod  string        `bun:"payment_method"`
	DueOn          calendar.Date `bun:"due_on,type:date"`
	SetByAccountID *int64        `bun:"set_by_account_id"`
}

// ServiceTerms reads the terms the Träger set, by service.
func (s *Store) ServiceTerms(ctx context.Context, organizationID int64) ([]parentbilling.ServiceTerms, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []serviceTermsRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).OrderExpr("service").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read service terms: %w", err)
	}
	terms := make([]parentbilling.ServiceTerms, len(rows))
	for i, row := range rows {
		terms[i] = row.terms()
	}
	return terms, nil
}

// SaveServiceTerms inserts or replaces the terms of one service.
func (s *Store) SaveServiceTerms(ctx context.Context, organizationID, accountID int64, terms parentbilling.ServiceTerms) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	row := &serviceTermsRow{OrganizationID: organizationID, Service: terms.Service, PaymentMethod: terms.PaymentMethod,
		DueDays: terms.DueDays, UpdatedByAccountID: accountRef(accountID), UpdatedAt: time.Now()}
	if terms.BundleWith != "" {
		row.BundleWith = &terms.BundleWith
	}
	if _, err := db.NewInsert().Model(row).
		On("CONFLICT (organization_id, service) DO UPDATE").
		Set("payment_method = EXCLUDED.payment_method").Set("due_days = EXCLUDED.due_days").
		Set("bundle_with = EXCLUDED.bundle_with").Set("updated_by_account_id = EXCLUDED.updated_by_account_id").
		Set("updated_at = CURRENT_TIMESTAMP").Exec(ctx); err != nil {
		return fmt.Errorf("save service terms: %w", err)
	}
	return nil
}

// FamilyBillings reads the deviations of the families, by family.
func (s *Store) FamilyBillings(ctx context.Context, organizationID int64) ([]parentbilling.FamilyBilling, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []familyBillingRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).OrderExpr("family_email").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read family billing: %w", err)
	}
	families := make([]parentbilling.FamilyBilling, len(rows))
	for i, row := range rows {
		families[i] = row.family()
	}
	return families, nil
}

// SaveFamilyBilling inserts or replaces the deviation of its family.
func (s *Store) SaveFamilyBilling(ctx context.Context, organizationID, accountID int64, family parentbilling.FamilyBilling) (parentbilling.FamilyBilling, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.FamilyBilling{}, err
	}
	row := &familyBillingRow{
		OrganizationID: organizationID, FamilyEmail: family.FamilyEmail, CareContractID: idRef(family.ContractID),
		RecipientKind: family.RecipientKind, SecondFirstName: family.SecondFirstName, SecondLastName: family.SecondLastName,
		PayerFirstName: family.PayerFirstName, PayerLastName: family.PayerLastName, PayerEmail: family.PayerEmail,
		DueDay: family.DueDay, DebtorNumber: textRef(family.DebtorNumber), UpdatedByAccountID: accountRef(accountID), UpdatedAt: time.Now(),
	}
	if family.PaymentMethod != "" {
		row.PaymentMethod = &family.PaymentMethod
	}
	if address := family.PayerAddress; address != nil {
		row.PayerStreet, row.PayerPostalCode, row.PayerCity = address.Street, address.PostalCode, address.City
	}
	if err := db.NewInsert().Model(row).
		On("CONFLICT (organization_id, family_email) DO UPDATE").
		Set("care_contract_id = EXCLUDED.care_contract_id").Set("recipient_kind = EXCLUDED.recipient_kind").
		Set("second_first_name = EXCLUDED.second_first_name").Set("second_last_name = EXCLUDED.second_last_name").
		Set("payer_first_name = EXCLUDED.payer_first_name").Set("payer_last_name = EXCLUDED.payer_last_name").
		Set("payer_email = EXCLUDED.payer_email").Set("payer_street = EXCLUDED.payer_street").
		Set("payer_postal_code = EXCLUDED.payer_postal_code").Set("payer_city = EXCLUDED.payer_city").
		Set("payment_method = EXCLUDED.payment_method").Set("due_day = EXCLUDED.due_day").
		Set("debtor_number = EXCLUDED.debtor_number").
		Set("updated_by_account_id = EXCLUDED.updated_by_account_id").Set("updated_at = CURRENT_TIMESTAMP").
		Returning("*").Scan(ctx); err != nil {
		return parentbilling.FamilyBilling{}, fmt.Errorf("save family billing: %w", err)
	}
	return row.family(), nil
}

// DeleteFamilyBilling removes a deviation by its ID or, with id zero, by its
// family, and answers the deviation it removed.
func (s *Store) DeleteFamilyBilling(ctx context.Context, organizationID, id int64, familyEmail string) (*parentbilling.FamilyBilling, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	query := db.NewDelete().Model((*familyBillingRow)(nil)).Where("organization_id = ?", organizationID)
	if id > 0 {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("family_email = ?", familyEmail)
	}
	var removed []familyBillingRow
	if err := query.Returning("*").Scan(ctx, &removed); err != nil {
		return nil, fmt.Errorf("delete family billing: %w", err)
	}
	if len(removed) == 0 {
		return nil, nil
	}
	family := removed[0].family()
	return &family, nil
}

// InsertFamilyBillingEvents records one action of the trail for each
// deviation in one statement.
func (s *Store) InsertFamilyBillingEvents(ctx context.Context, organizationID, accountID int64, action string, families []parentbilling.FamilyBilling) error {
	if len(families) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	rows := make([]familyBillingEventRow, len(families))
	for i, family := range families {
		rows[i] = familyBillingEventRow{OrganizationID: organizationID, FamilyBillingID: family.ID, CareContractID: idRef(family.ContractID),
			Action: action, ActorAccountID: accountRef(accountID)}
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("record family billing: %w", err)
	}
	return nil
}

// ReplaceRunDueDates replaces the due dates of a monthly or a holiday run;
// the trigger refuses it for a released run.
func (s *Store) ReplaceRunDueDates(ctx context.Context, organizationID, accountID, careRunID, holidayRunID int64, dates parentbilling.RunDueDates) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewDelete().Model((*runDueDateRow)(nil)).Where("organization_id = ?", organizationID).
		Where("(care_run_id = ? OR holiday_run_id = ?)", careRunID, holidayRunID).Exec(ctx); err != nil {
		return fmt.Errorf("replace run due dates: %w", err)
	}
	if len(dates) == 0 {
		return nil
	}
	rows := make([]runDueDateRow, 0, len(dates))
	for _, method := range parentbilling.PaymentMethods {
		due, ok := dates[method]
		if !ok {
			continue
		}
		row := runDueDateRow{OrganizationID: organizationID, PaymentMethod: method, DueOn: due, SetByAccountID: accountRef(accountID)}
		if careRunID > 0 {
			row.CareRunID = &careRunID
		} else {
			row.HolidayRunID = &holidayRunID
		}
		rows = append(rows, row)
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("insert run due dates: %w", err)
	}
	return nil
}

// invoicePaymentTermsSQL reads the terms, the deviations and the due dates
// of one run in one statement, each part under the Träger's own filter.
const invoicePaymentTermsSQL = `SELECT
	(SELECT COALESCE(jsonb_agg(to_jsonb(terms) ORDER BY terms.service), '[]'::jsonb)
		FROM (SELECT service, payment_method, due_days, bundle_with, updated_at
			FROM parent_billing.service_terms WHERE organization_id = ?0) AS terms) AS terms,
	(SELECT COALESCE(jsonb_agg(to_jsonb(family) ORDER BY family.family_email), '[]'::jsonb)
		FROM (SELECT id, family_email, care_contract_id, recipient_kind, second_first_name, second_last_name,
				payer_first_name, payer_last_name, payer_email, payer_street, payer_postal_code, payer_city,
				payment_method, due_day, debtor_number, updated_at
			FROM parent_billing.family_billing WHERE organization_id = ?0) AS family) AS families,
	(SELECT COALESCE(jsonb_object_agg(payment_method, due_on), '{}'::jsonb)
		FROM parent_billing.run_due_dates
		WHERE organization_id = ?0 AND (care_run_id = ?1 OR holiday_run_id = ?2)) AS dates`

// InvoicePaymentTerms reads in one statement what decides the recipient
// and the payment of the invoices of a run (careRunID or holidayRunID).
func (s *Store) InvoicePaymentTerms(ctx context.Context, organizationID, careRunID, holidayRunID int64) (
	[]parentbilling.ServiceTerms, []parentbilling.FamilyBilling, parentbilling.RunDueDates, error,
) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, nil, nil, err
	}
	var raw struct {
		Terms    json.RawMessage `bun:"terms"`
		Families json.RawMessage `bun:"families"`
		Dates    json.RawMessage `bun:"dates"`
	}
	if err := db.NewRaw(invoicePaymentTermsSQL, organizationID, careRunID, holidayRunID).Scan(ctx, &raw); err != nil {
		return nil, nil, nil, fmt.Errorf("read payment terms: %w", err)
	}
	var termRows []serviceTermsRow
	var familyRows []familyBillingRow
	dates := parentbilling.RunDueDates{}
	if err := json.Unmarshal(raw.Terms, &termRows); err != nil {
		return nil, nil, nil, fmt.Errorf("read payment terms: %w", err)
	}
	if err := json.Unmarshal(raw.Families, &familyRows); err != nil {
		return nil, nil, nil, fmt.Errorf("read family billing: %w", err)
	}
	if err := json.Unmarshal(raw.Dates, &dates); err != nil {
		return nil, nil, nil, fmt.Errorf("read run due dates: %w", err)
	}
	terms := make([]parentbilling.ServiceTerms, len(termRows))
	for i, row := range termRows {
		terms[i] = row.terms()
	}
	families := make([]parentbilling.FamilyBilling, len(familyRows))
	for i, row := range familyRows {
		families[i] = row.family()
	}
	return terms, families, dates, nil
}

// accountRef is the account of a write, nil for none.
func accountRef(accountID int64) *int64 {
	if accountID <= 0 {
		return nil
	}
	return &accountID
}

// idRef is a reference by ID, NULL for none.
func idRef(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	return &id
}

// idOf is a referenced ID, zero for NULL.
func idOf(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

// textRef is a copied text, NULL when empty.
func textRef(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// textOf is a copied text, empty for NULL.
func textOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// paymentSnapshot is the payment an issued invoice copied: how and by when
// it is paid (#4689) and, for a direct debit, the mandate it is collected
// with or why it is not (#4695). Every column is NULL on an invoice issued
// before; the mandate columns are NULL on any other payment.
type paymentSnapshot struct {
	PaymentMethod    *string
	DueOn            *calendar.Date
	DueSource        *string
	MandateID        *int64
	MandateReference *string
	MandateSignedOn  *calendar.Date
	CreditorID       *string
	DebtorIBANMasked *string
	SequenceType     *string
	DebitGap         *string
}

// snapshotOf is the copy of a payment, all NULL without one.
func snapshotOf(payment *parentbilling.InvoicePayment) paymentSnapshot {
	if payment == nil {
		return paymentSnapshot{}
	}
	due := payment.DueOn
	snapshot := paymentSnapshot{PaymentMethod: textRef(payment.Method), DueOn: &due, DueSource: textRef(payment.DueSource),
		DebitGap: textRef(payment.DebitGap)}
	if mandate := payment.Mandate; mandate != nil {
		id, signedOn := mandate.MandateID, mandate.SignedOn
		snapshot.MandateID, snapshot.MandateReference, snapshot.MandateSignedOn = &id, textRef(mandate.Reference), &signedOn
		snapshot.CreditorID, snapshot.DebtorIBANMasked = textRef(mandate.CreditorID), textRef(mandate.IBANMasked)
		snapshot.SequenceType = textRef(mandate.SequenceType)
	}
	return snapshot
}

// payment is the copied payment of an invoice, nil for one issued before
// #4689.
func (s paymentSnapshot) payment() *parentbilling.InvoicePayment {
	if s.PaymentMethod == nil || s.DueOn == nil || s.DueSource == nil {
		return nil
	}
	payment := &parentbilling.InvoicePayment{Method: *s.PaymentMethod, DueOn: *s.DueOn, DueSource: *s.DueSource, DebitGap: textOf(s.DebitGap)}
	if s.MandateID != nil && s.MandateSignedOn != nil {
		payment.Mandate = &parentbilling.InvoiceMandate{MandateID: *s.MandateID, Reference: textOf(s.MandateReference),
			SignedOn: *s.MandateSignedOn, CreditorID: textOf(s.CreditorID), IBANMasked: textOf(s.DebtorIBANMasked),
			SequenceType: textOf(s.SequenceType)}
	}
	return payment
}

// FamilyDebtorNumbers reads the debtor number of each family that has one
// (#4693).
func (s *Store) FamilyDebtorNumbers(ctx context.Context, organizationID int64) (map[string]string, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []familyBillingRow
	if err := db.NewSelect().Model(&rows).Column("family_email", "debtor_number").Where("organization_id = ?", organizationID).
		Where("debtor_number IS NOT NULL").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read the debtor numbers of the families: %w", err)
	}
	numbers := make(map[string]string, len(rows))
	for _, row := range rows {
		numbers[row.FamilyEmail] = textOf(row.DebtorNumber)
	}
	return numbers, nil
}

// SetFamilyDebtorNumbers gives each family its debtor number where it has
// none: a family without a deviation gets one that keeps the registration as
// recipient. A family that got a number since keeps it. Each change is a
// "save" in the family's trail.
func (s *Store) SetFamilyDebtorNumbers(ctx context.Context, organizationID, accountID int64, numbers []parentbilling.FamilyDebtorNumber) error {
	if len(numbers) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	rows := make([]familyBillingRow, len(numbers))
	for i, number := range numbers {
		rows[i] = familyBillingRow{OrganizationID: organizationID, FamilyEmail: number.FamilyEmail, CareContractID: idRef(number.ContractID),
			RecipientKind: parentbilling.RecipientApplicant, DebtorNumber: textRef(number.DebtorNumber),
			UpdatedByAccountID: accountRef(accountID), UpdatedAt: time.Now()}
	}
	var saved []familyBillingRow
	if err := db.NewInsert().Model(&rows).
		On("CONFLICT (organization_id, family_email) DO UPDATE").
		Set("debtor_number = EXCLUDED.debtor_number").Set("updated_by_account_id = EXCLUDED.updated_by_account_id").
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("family_billing_row.debtor_number IS NULL").
		Returning("id, care_contract_id").Scan(ctx, &saved); err != nil {
		return fmt.Errorf("set the debtor numbers of the families: %w", err)
	}
	families := make([]parentbilling.FamilyBilling, len(saved))
	for i, row := range saved {
		families[i] = parentbilling.FamilyBilling{ID: row.ID, ContractID: idOf(row.CareContractID)}
	}
	return s.InsertFamilyBillingEvents(ctx, organizationID, accountID, "save", families)
}
