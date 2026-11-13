package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The monthly contribution of the care contract (#4684): the price list,
// the monthly runs, their trail and the issued invoices. The request role
// inserts and reads invoices, positions and events and never changes them;
// a released run never changes (trigger of 1.15.789).

const (
	// openCareRunSchoolIndex keeps a school to one open run per month.
	openCareRunSchoolIndex = "uq_care_run_schools_open"
	// careInvoiceNumberConstraint keeps the numbers of the monthly
	// invoices unique.
	careInvoiceNumberConstraint = "uq_care_invoices_number"
	// careInvoiceMonthConstraint bills a contract at most once a month.
	careInvoiceMonthConstraint = "uq_care_invoices_contract_month"
	// careInvoiceReissueConstraint replaces a cancelled invoice at most once.
	careInvoiceReissueConstraint = "uq_care_invoices_reissue"
)

type careTariffRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_tariffs"`

	ID                 int64         `bun:"id,pk,autoincrement"`
	OrganizationID     int64         `bun:"organization_id"`
	SchoolID           int64         `bun:"school_id"`
	CareOfferingID     int64         `bun:"care_offering_id"`
	ModuleName         string        `bun:"module_name"`
	Weekday            *string       `bun:"weekday"`
	DaysPerWeek        *int          `bun:"days_per_week"`
	ValidFrom          calendar.Date `bun:"valid_from,type:date"`
	MonthlyCents       int64         `bun:"monthly_cents"`
	SiblingCents       *int64        `bun:"sibling_cents"`
	MealExtra          bool          `bun:"meal_extra"`
	CreatedByAccountID *int64        `bun:"created_by_account_id"`
	CreatedAt          time.Time     `bun:"created_at,nullzero,default:current_timestamp"`
}

type careRunRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_runs"`

	ID                 int64          `bun:"id,pk,autoincrement"`
	OrganizationID     int64          `bun:"organization_id"`
	BillingMonth       calendar.Date  `bun:"billing_month,type:date"`
	Status             string         `bun:"status"`
	CreatedAt          time.Time      `bun:"created_at,nullzero,default:current_timestamp"`
	CreatedByAccountID *int64         `bun:"created_by_account_id"`
	ReleasedAt         *time.Time     `bun:"released_at"`
	IssuedOn           *calendar.Date `bun:"issued_on,type:date"`
	DocumentCount      *int           `bun:"document_count"`
	TotalCents         *int64         `bun:"total_cents"`
	MonthRule          *string        `bun:"month_rule"`
	InvoiceDate        *calendar.Date `bun:"invoice_date,type:date"`
	DiscardedAt        *time.Time     `bun:"discarded_at"`
	// SchoolIDs come from parent_billing.care_run_schools (#5916).
	SchoolIDs []int64 `bun:"school_ids,array,scanonly"`
	// The immutable release event stores the free-month count.
	FreeOfCharge int `bun:"free_of_charge,scanonly"`
}

func (r careRunRow) run() parentbilling.CareRun {
	run := parentbilling.CareRun{
		ID: r.ID, BillingMonth: r.BillingMonth, Status: r.Status, CreatedAt: r.CreatedAt, ReleasedAt: r.ReleasedAt,
		IssuedOn: r.IssuedOn, DocumentCount: r.DocumentCount, TotalCents: r.TotalCents,
		FreeOfCharge: r.FreeOfCharge, DiscardedAt: r.DiscardedAt,
		CareRunScope: parentbilling.CareRunScope{SchoolIDs: r.SchoolIDs, InvoiceDate: r.InvoiceDate},
	}
	if run.SchoolIDs == nil {
		run.SchoolIDs = []int64{}
	}
	if r.MonthRule != nil {
		run.MonthRule = *r.MonthRule
	}
	return run
}

type careRunEventRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_run_events"`

	ID             int64          `bun:"id,pk,autoincrement"`
	OrganizationID int64          `bun:"organization_id"`
	RunID          int64          `bun:"run_id"`
	Action         string         `bun:"action"`
	ActorAccountID *int64         `bun:"actor_account_id"`
	Details        map[string]any `bun:"details,type:jsonb"`
}

type careInvoiceRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_invoices"`

	ID                 int64         `bun:"id,pk,autoincrement"`
	OrganizationID     int64         `bun:"organization_id"`
	RunID              int64         `bun:"run_id"`
	Number             string        `bun:"number"`
	NumberYear         int           `bun:"number_year"`
	NumberSequence     int64         `bun:"number_sequence"`
	IssuedOn           calendar.Date `bun:"issued_on,type:date"`
	BillingMonth       calendar.Date `bun:"billing_month,type:date"`
	CareContractID     int64         `bun:"care_contract_id"`
	Service            string        `bun:"service"`
	SchoolID           int64         `bun:"school_id"`
	SchoolName         string        `bun:"school_name"`
	ChildFirstName     string        `bun:"child_first_name"`
	ChildLastName      string        `bun:"child_last_name"`
	RecipientFirstName string        `bun:"recipient_first_name"`
	RecipientLastName  string        `bun:"recipient_last_name"`
	RecipientEmail     string        `bun:"recipient_email"`
	MonthRule          string        `bun:"month_rule"`
	TotalCents         int64         `bun:"total_cents"`
	ReplacesInvoiceID  *int64        `bun:"replaces_invoice_id"`
	StudentID          *int64        `bun:"student_id"`
	DocumentKind       string        `bun:"document_kind"`
	// The family, the recipient and the payment the release copied
	// (#4689); NULL on invoices issued before.
	FamilyEmail         *string        `bun:"family_email"`
	RecipientKind       *string        `bun:"recipient_kind"`
	RecipientSecondName *string        `bun:"recipient_second_name"`
	RecipientStreet     *string        `bun:"recipient_address_street"`
	RecipientPostalCode *string        `bun:"recipient_address_postal_code"`
	RecipientCity       *string        `bun:"recipient_address_city"`
	PaymentMethod       *string        `bun:"payment_method"`
	DueOn               *calendar.Date `bun:"due_on,type:date"`
	DueSource           *string        `bun:"due_source"`
	MandateID           *int64         `bun:"mandate_id"`
	MandateReference    *string        `bun:"mandate_reference"`
	MandateSignedOn     *calendar.Date `bun:"mandate_signed_on,type:date"`
	CreditorID          *string        `bun:"creditor_id"`
	DebtorIBANMasked    *string        `bun:"debtor_iban_masked"`
	SequenceType        *string        `bun:"sequence_type"`
	DebitGap            *string        `bun:"debit_gap"`
	DebtorNumber        *string        `bun:"debtor_number"`
	// CostShares are the issued cost bearer shares (#4687), read in the same
	// statement as the invoice.
	CostShares []parentbilling.CareInvoiceCostShare `bun:"cost_shares,type:jsonb,scanonly"`
	// Corrections are the changes the release applied (#6197), read in the
	// same statement as the invoice.
	Corrections []parentbilling.CareAmendment `bun:"corrections,type:jsonb,scanonly"`
}

// invoice is the issued invoice of the row with its positions and reductions
// and the recipient and payment its release copied (#4689).
func (row careInvoiceRow) invoice(positions []parentbilling.CarePosition, reductions []parentbilling.CareReduction) parentbilling.CareInvoice {
	issuedOn := row.IssuedOn
	invoice := parentbilling.CareInvoice{
		ID: row.ID, Number: row.Number, IssuedOn: &issuedOn, ContractID: row.CareContractID, Service: row.Service, SchoolID: row.SchoolID,
		SchoolName: row.SchoolName, ChildFirstName: row.ChildFirstName, ChildLastName: row.ChildLastName, StudentID: row.StudentID,
		Recipient: parentbilling.CareRecipient{FirstName: row.RecipientFirstName, LastName: row.RecipientLastName, Email: row.RecipientEmail,
			Kind: textOf(row.RecipientKind), SecondName: textOf(row.RecipientSecondName)},
		Positions: positions, Reductions: reductions, TotalCents: row.TotalCents, DocumentKind: row.DocumentKind,
		FamilyEmail: textOf(row.FamilyEmail), Payment: row.snapshot().payment(), DebtorNumber: textOf(row.DebtorNumber),
	}
	if row.RecipientStreet != nil && row.RecipientPostalCode != nil && row.RecipientCity != nil {
		invoice.Recipient.Address = &parentbilling.PostalAddress{Street: *row.RecipientStreet,
			PostalCode: *row.RecipientPostalCode, City: *row.RecipientCity}
	}
	if invoice.Reductions == nil {
		invoice.Reductions = []parentbilling.CareReduction{}
	}
	invoice.CostShares = []parentbilling.CareInvoiceCostShare{}
	invoice.Amendments = []parentbilling.CareAmendment{}
	if invoice.Positions == nil {
		invoice.Positions = []parentbilling.CarePosition{}
	}
	return invoice
}

type careInvoicePositionRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_invoice_positions"`

	ID                    int64           `bun:"id,pk,autoincrement"`
	OrganizationID        int64           `bun:"organization_id"`
	InvoiceID             int64           `bun:"invoice_id"`
	Position              int             `bun:"position"`
	CareOfferingID        int64           `bun:"care_offering_id"`
	ModuleName            string          `bun:"module_name"`
	ContractVersionNumber int             `bun:"contract_version_number"`
	ServiceFrom           calendar.Date   `bun:"service_from,type:date"`
	ServiceUntil          calendar.Date   `bun:"service_until,type:date"`
	Weekdays              json.RawMessage `bun:"weekdays,type:jsonb"`
	DaysPerWeek           *int            `bun:"days_per_week"`
	TierMonthlyCents      *int64          `bun:"tier_monthly_cents"`
	MealExtra             bool            `bun:"meal_extra"`
	AmountCents           int64           `bun:"amount_cents"`
	NetCents              int64           `bun:"net_cents"`
	PayableCents          int64           `bun:"payable_cents"`
	positionAccountsColumns
}

// CareTariffs reads every price of the Träger.
func (s *Store) CareTariffs(ctx context.Context, organizationID int64) ([]parentbilling.CareTariff, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careTariffRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		OrderExpr("school_id, valid_from, care_offering_id, weekday, days_per_week").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care tariffs: %w", err)
	}
	tariffs := make([]parentbilling.CareTariff, len(rows))
	for i, row := range rows {
		tariffs[i] = parentbilling.CareTariff{
			ID: row.ID, SchoolID: row.SchoolID, CareOfferingID: row.CareOfferingID, ModuleName: row.ModuleName,
			DaysPerWeek: row.DaysPerWeek, ValidFrom: row.ValidFrom, MonthlyCents: row.MonthlyCents,
			SiblingCents: row.SiblingCents, MealExtra: row.MealExtra, CreatedAt: row.CreatedAt,
		}
		if row.Weekday != nil {
			tariffs[i].Weekday = *row.Weekday
		}
	}
	return tariffs, nil
}

// ReplaceCarePrices replaces the prices of a school that start on
// validFrom by the given ones.
func (s *Store) ReplaceCarePrices(ctx context.Context, organizationID, accountID int64, list parentbilling.CarePriceList) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := s.DeleteCarePrices(ctx, organizationID, list.SchoolID, list.ValidFrom); err != nil {
		return err
	}
	free := list.FreeMonths
	if free == nil {
		free = []int{}
	}
	head := &carePriceListRow{OrganizationID: organizationID, SchoolID: list.SchoolID, ValidFrom: list.ValidFrom, FreeMonths: free}
	if _, err := db.NewInsert().Model(head).Exec(ctx); err != nil {
		return fmt.Errorf("insert care price list: %w", err)
	}
	if len(list.Prices) == 0 {
		return nil
	}
	rows := make([]careTariffRow, len(list.Prices))
	for i, price := range list.Prices {
		rows[i] = careTariffRow{
			OrganizationID: organizationID, SchoolID: list.SchoolID, CareOfferingID: price.CareOfferingID,
			ModuleName: price.ModuleName, DaysPerWeek: price.DaysPerWeek, ValidFrom: list.ValidFrom,
			MonthlyCents: price.MonthlyCents, SiblingCents: price.SiblingCents, MealExtra: price.MealExtra,
			CreatedByAccountID: &accountID,
		}
		if price.Weekday != "" {
			weekday := price.Weekday
			rows[i].Weekday = &weekday
		}
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("insert care tariffs: %w", err)
	}
	return nil
}

// DeleteCarePrices removes the prices of a school that start on validFrom
// and counts them.
func (s *Store) DeleteCarePrices(ctx context.Context, organizationID, schoolID int64, validFrom calendar.Date) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	heads, err := db.NewDelete().TableExpr("parent_billing.care_price_lists").
		Where("organization_id = ?", organizationID).Where("school_id = ?", schoolID).Where("valid_from = ?", validFrom).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete care price list: %w", err)
	}
	result, err := db.NewDelete().TableExpr("parent_billing.care_tariffs").
		Where("organization_id = ?", organizationID).Where("school_id = ?", schoolID).Where("valid_from = ?", validFrom).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete care tariffs: %w", err)
	}
	prices, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	// A price list without prices (only free months) still counts as one.
	lists, err := heads.RowsAffected()
	return max(prices, lists), err
}

// CareBilledUntil names per school the last month an issued invoice bills.
func (s *Store) CareBilledUntil(ctx context.Context, organizationID int64) (map[int64]calendar.Date, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		SchoolID int64         `bun:"school_id"`
		Month    calendar.Date `bun:"month,type:date"`
	}
	if err := db.NewRaw(`SELECT school_id, MAX(billing_month) AS month FROM parent_billing.care_invoices
		WHERE organization_id = ? GROUP BY school_id`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read billed months: %w", err)
	}
	billed := make(map[int64]calendar.Date, len(rows))
	for _, row := range rows {
		billed[row.SchoolID] = row.Month
	}
	return billed, nil
}

// CareBilledPrices lists per module and weekday the months an issued
// invoice of the school billed, from the weekdays of its positions.
func (s *Store) CareBilledPrices(ctx context.Context, organizationID, schoolID int64) ([]parentbilling.CareBilledPrice, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		CareOfferingID int64         `bun:"care_offering_id"`
		Weekday        string        `bun:"weekday"`
		DaysPerWeek    *int          `bun:"days_per_week"`
		Month          calendar.Date `bun:"billing_month,type:date"`
	}
	// A position priced by a scale billed its step, not its weekdays.
	if err := db.NewRaw(`SELECT DISTINCT p.care_offering_id,
			CASE WHEN p.days_per_week IS NULL THEN w.value->>'weekday' ELSE '' END AS weekday,
			p.days_per_week, i.billing_month
		FROM parent_billing.care_invoices AS i
		JOIN parent_billing.care_invoice_positions AS p ON p.organization_id = i.organization_id AND p.invoice_id = i.id
		CROSS JOIN LATERAL jsonb_array_elements(p.weekdays) AS w
		WHERE i.organization_id = ? AND i.school_id = ?`, organizationID, schoolID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read billed prices: %w", err)
	}
	billed := make([]parentbilling.CareBilledPrice, len(rows))
	for i, row := range rows {
		billed[i] = parentbilling.CareBilledPrice{CareOfferingID: row.CareOfferingID, Weekday: row.Weekday,
			DaysPerWeek: row.DaysPerWeek, Month: row.Month}
	}
	return billed, nil
}

// LockCareBilling holds the monthly billing of the Träger until the
// transaction ends: a release and a change of the price list run one after
// the other, so no price starts in a month a parallel release bills.
func (s *Store) LockCareBilling(ctx context.Context, organizationID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('parent_billing.care_billing:' || ?::text, 0))`,
		organizationID).Exec(ctx)
	return err
}

// ListCareRuns lists the monthly runs, the latest month first.
func (s *Store) ListCareRuns(ctx context.Context, organizationID int64) ([]parentbilling.CareRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careRunRow
	if err := withRunSchools(db.NewSelect().Model(&rows)).Where("care_run_row.organization_id = ?", organizationID).
		OrderExpr("care_run_row.billing_month DESC, care_run_row.id DESC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list care runs: %w", err)
	}
	runs := make([]parentbilling.CareRun, len(rows))
	for i, row := range rows {
		runs[i] = row.run()
	}
	return runs, nil
}

// InsertCareRun starts a calculated run of a month for its schools; a
// school another open run of the month bills is ErrCareRunOpen, also when
// both start at once (uq_care_run_schools_open, #5916).
func (s *Store) InsertCareRun(ctx context.Context, organizationID, accountID int64, start parentbilling.CareRunStart) (parentbilling.CareRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	row := &careRunRow{OrganizationID: organizationID, BillingMonth: start.BillingMonth, Status: parentbilling.RunCalculated,
		CreatedByAccountID: &accountID, InvoiceDate: start.InvoiceDate}
	if _, err := db.NewInsert().Model(row).Returning("id, created_at").Exec(ctx); err != nil {
		return parentbilling.CareRun{}, fmt.Errorf("insert care run: %w", err)
	}
	if err := insertCareRunSchools(ctx, db, organizationID, row.ID, start.BillingMonth, start.SchoolIDs); err != nil {
		return parentbilling.CareRun{}, err
	}
	row.SchoolIDs = slices.Sorted(slices.Values(start.SchoolIDs))
	return row.run(), nil
}

// CareRun reads a run, for update when lock is set; a missing one is
// ErrCareRunNotFound.
func (s *Store) CareRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.CareRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	row := &careRunRow{}
	query := withRunSchools(db.NewSelect().Model(row)).
		Where("care_run_row.organization_id = ?", organizationID).Where("care_run_row.id = ?", runID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.CareRun{}, parentbilling.ErrCareRunNotFound
		}
		return parentbilling.CareRun{}, fmt.Errorf("read care run: %w", err)
	}
	return row.run(), nil
}

// InvoicedContracts reads the immutable amounts and scope of earlier runs.
// Only the invoices are filtered by the Träger: positions and reductions
// follow them by invoice ID, which is unique across Träger, and their
// composite key (organization_id, invoice_id) ties them to its Träger.
func (s *Store) InvoicedContracts(ctx context.Context, organizationID int64, month calendar.Date) (map[int64]parentbilling.CareInvoice, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []invoicedContractRow
	// A Mensa-only invoice (#4686) has no module position; it still bills
	// its contract, so the positions join from the left for an invoice with
	// Mensa months. A month a Jahresbescheid nobody amended assesses bills
	// its contract too (#6219, ADR 0182): it comes from the snapshot of the
	// month with the cents the rule chain allocated, and names the notice.
	if err := db.NewRaw(invoicedContractsSQL, organizationID, month, organizationID, month).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read invoiced contracts: %w", err)
	}
	return invoicedFromRows(rows), nil
}

// InsertCareRunEvent records an action of the run's trail.
func (s *Store) InsertCareRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if details == nil {
		details = map[string]any{}
	}
	row := &careRunEventRow{OrganizationID: organizationID, RunID: runID, Action: action, ActorAccountID: &accountID, Details: details}
	_, err = db.NewInsert().Model(row).Exec(ctx)
	return err
}

// Tables of issued numbers. The holiday documents, the monthly invoices and
// their Storno take their numbers from two number ranges; a Träger may give
// both the same pattern, so none may repeat a number of another (#4684).
const (
	holidayNumbersTable          = "parent_billing.documents"
	careNumbersTable             = "parent_billing.care_invoices"
	careCancellationNumbersTable = "parent_billing.care_invoice_cancellations"
)

// claimInvoiceNumbers holds the invoice numbers of the Träger until the
// transaction ends, so a holiday and a monthly issue run one after the
// other, and reports a number otherTable or a Storno of a monthly invoice
// already carries as ErrInvoiceNumberTaken (#4684). Every issue path calls
// it before it inserts; a Storno calls it for both other tables.
func claimInvoiceNumbers(ctx context.Context, db bun.IDB, organizationID int64, numbers []string, otherTable string) error {
	if len(numbers) == 0 {
		return nil
	}
	if _, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('parent_billing.invoice_numbers:' || ?::text, 0))`,
		organizationID).Exec(ctx); err != nil {
		return fmt.Errorf("hold the invoice numbers: %w", err)
	}
	for _, table := range []string{otherTable, careCancellationNumbersTable, careNoticeNumbersTable} {
		var taken bool
		if err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM ? WHERE organization_id = ? AND number = ANY(?))`,
			bun.Safe(table), organizationID, pgdialect.Array(numbers)).Scan(ctx, &taken); err != nil {
			return fmt.Errorf("check invoice numbers: %w", err)
		}
		if taken {
			return parentbilling.ErrInvoiceNumberTaken
		}
	}
	return nil
}

// IssueCareInvoices stores the numbered invoices of a release with their
// positions; the i-th invoice has the sequence first+i of the counter of
// year. A number a holiday document or another monthly invoice carries is
// ErrInvoiceNumberTaken, a contract an invoice of the month already bills
// ErrBillingRunChanged.
func (s *Store) IssueCareInvoices(ctx context.Context, organizationID int64, run parentbilling.CareRun, rule string, year int, first int64, invoices []parentbilling.CareInvoice) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	numbers := make([]string, len(invoices))
	for i, invoice := range invoices {
		numbers[i] = invoice.Number
	}
	if err := claimInvoiceNumbers(ctx, db, organizationID, numbers, holidayNumbersTable); err != nil {
		return err
	}
	replaced, err := cancelledInvoicesToReplace(ctx, db, organizationID, run.BillingMonth, invoices)
	if err != nil {
		return err
	}
	rows := make([]careInvoiceRow, len(invoices))
	for i, invoice := range invoices {
		rows[i] = careInvoiceRow{
			OrganizationID: organizationID, RunID: run.ID, Number: invoice.Number, NumberYear: year, NumberSequence: first + int64(i),
			IssuedOn: *invoice.IssuedOn, BillingMonth: run.BillingMonth, CareContractID: invoice.ContractID, Service: invoiceService(invoice),
			SchoolID: invoice.SchoolID, SchoolName: invoice.SchoolName, ChildFirstName: invoice.ChildFirstName,
			ChildLastName: invoice.ChildLastName, RecipientFirstName: invoice.Recipient.FirstName,
			RecipientLastName: invoice.Recipient.LastName, RecipientEmail: invoice.Recipient.Email,
			MonthRule: rule, TotalCents: invoice.TotalCents, ReplacesInvoiceID: replaced[replacedKey{invoice.ContractID, invoiceService(invoice)}],
			StudentID: invoice.StudentID, FamilyEmail: textRef(invoice.FamilyEmail), RecipientKind: textRef(invoice.Recipient.Kind),
			RecipientSecondName: textRef(invoice.Recipient.SecondName),
			DocumentKind:        careDocumentKind(invoice.DocumentKind), DebtorNumber: textRef(invoice.DebtorNumber),
		}
		if address := invoice.Recipient.Address; address != nil {
			rows[i].RecipientStreet, rows[i].RecipientPostalCode, rows[i].RecipientCity = &address.Street, &address.PostalCode, &address.City
		}
		rows[i].setPayment(snapshotOf(invoice.Payment))
	}
	if _, err := db.NewInsert().Model(&rows).Returning("id").Exec(ctx); err != nil {
		switch constraintOf(err) {
		case careInvoiceNumberConstraint:
			return parentbilling.ErrInvoiceNumberTaken
		case careInvoiceMonthConstraint, careInvoiceReissueConstraint:
			return parentbilling.ErrBillingRunChanged
		}
		return fmt.Errorf("insert care invoices: %w", err)
	}
	if err := insertCareInvoiceParts(ctx, db, organizationID, rows, invoices); err != nil {
		return err
	}
	return insertSEPADebits(ctx, db, organizationID, false, careInvoiceIDs(rows))
}

// insertCareInvoiceParts stores what the invoices just issued carry: their
// positions, reductions, Mensa months (#4686), the changes of the review
// (#6197) and cost bearer shares (#4687);
// rows run parallel to invoices.
func insertCareInvoiceParts(ctx context.Context, db bun.IDB, organizationID int64, rows []careInvoiceRow, invoices []parentbilling.CareInvoice) error {
	positions, err := carePositionRows(organizationID, rows, invoices)
	if err != nil {
		return err
	}
	if len(positions) > 0 {
		if _, err := db.NewInsert().Model(&positions).Exec(ctx); err != nil {
			return fmt.Errorf("insert care invoice positions: %w", err)
		}
	}
	if err := insertCareReductions(ctx, db, organizationID, rows, invoices); err != nil {
		return err
	}
	if err := insertCareMealPositions(ctx, db, organizationID, rows, invoices); err != nil {
		return err
	}
	if err := insertCareCorrections(ctx, db, organizationID, rows, invoices); err != nil {
		return err
	}
	return insertCareCostShares(ctx, db, organizationID, rows, invoices)
}

func carePositionRows(organizationID int64, rows []careInvoiceRow, invoices []parentbilling.CareInvoice) ([]careInvoicePositionRow, error) {
	var positions []careInvoicePositionRow
	for i, invoice := range invoices {
		for j, position := range invoice.Positions {
			weekdays, err := json.Marshal(position.Weekdays)
			if err != nil {
				return nil, err
			}
			positions = append(positions, careInvoicePositionRow{
				OrganizationID: organizationID, InvoiceID: rows[i].ID, Position: j + 1, CareOfferingID: position.CareOfferingID,
				ModuleName: position.ModuleName, ContractVersionNumber: position.VersionNumber, ServiceFrom: position.ServiceFrom,
				ServiceUntil: position.ServiceUntil, Weekdays: weekdays, AmountCents: position.AmountCents, NetCents: position.AmountCents, PayableCents: position.PayableCents,
				DaysPerWeek: position.DaysPerWeek, TierMonthlyCents: position.TierMonthlyCents, MealExtra: position.MealExtra,
				positionAccountsColumns: positionAccountsOf(position.Accounts),
			})
		}
	}
	return positions, nil
}

// MarkCareRunReleased closes a calculated run with its issue date, sums
// and the rule it billed by.
func (s *Store) MarkCareRunReleased(ctx context.Context, organizationID, runID int64, issuedOn calendar.Date, count int, totalCents int64, rule string) (parentbilling.CareRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	row := &careRunRow{}
	if err := db.NewUpdate().Model(row).
		Set("status = ?", parentbilling.RunReleased).Set("released_at = CURRENT_TIMESTAMP").Set("issued_on = ?", issuedOn).
		Set("document_count = ?", count).Set("total_cents = ?", totalCents).Set("month_rule = ?", rule).
		Where("organization_id = ?", organizationID).Where("id = ?", runID).Where("status = ?", parentbilling.RunCalculated).
		Returning("*").Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.CareRun{}, parentbilling.ErrBillingRunReleased
		}
		return parentbilling.CareRun{}, fmt.Errorf("release care run: %w", err)
	}
	schools, err := closeCareRunSchools(ctx, db, organizationID, runID)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	row.SchoolIDs = schools
	return row.run(), nil
}

// CareRunInvoices reads the issued invoices of a run with their positions,
// by number.
func (s *Store) CareRunInvoices(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareInvoice, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	// The Mensa months (#4686) come with their invoice in one statement.
	var rows []careInvoiceMealRow
	if err := db.NewSelect().Model(&rows).ModelTableExpr(`parent_billing.care_invoices AS "care_invoice_row"`).
		ColumnExpr("?TableColumns").ColumnExpr(careInvoiceMealsColumn).ColumnExpr(careInvoiceCostSharesColumn).ColumnExpr(careInvoiceCorrectionsColumn).
		Where("care_invoice_row.organization_id = ?", organizationID).Where("care_invoice_row.run_id = ?", runID).
		OrderExpr("care_invoice_row.number_sequence").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care invoices: %w", err)
	}
	if len(rows) == 0 {
		return []parentbilling.CareInvoice{}, nil
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	// The positions belong to the invoices just read under the Träger's
	// filter; their IDs are unique across Träger, so they name the Träger.
	var positions []careInvoicePositionRow
	if err := db.NewSelect().Model(&positions).
		Where("invoice_id = ANY(?)", pgdialect.Array(ids)).OrderExpr("invoice_id, position").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care invoice positions: %w", err)
	}
	reductions, err := careReductionsOf(ctx, db, organizationID, ids)
	if err != nil {
		return nil, err
	}
	byInvoice := map[int64][]parentbilling.CarePosition{}
	for _, row := range positions {
		position := parentbilling.CarePosition{
			CareOfferingID: row.CareOfferingID, ModuleName: row.ModuleName, VersionNumber: row.ContractVersionNumber,
			ServiceFrom: row.ServiceFrom, ServiceUntil: row.ServiceUntil, AmountCents: row.AmountCents,
			DaysPerWeek: row.DaysPerWeek, TierMonthlyCents: row.TierMonthlyCents, MealExtra: row.MealExtra,
		}
		if err := json.Unmarshal(row.Weekdays, &position.Weekdays); err != nil {
			return nil, fmt.Errorf("read care invoice weekdays: %w", err)
		}
		byInvoice[row.InvoiceID] = append(byInvoice[row.InvoiceID], position)
	}
	invoices := make([]parentbilling.CareInvoice, len(rows))
	for i, row := range rows {
		invoices[i] = row.invoice(byInvoice[row.ID], reductions[row.ID])
		if len(row.Corrections) > 0 {
			invoices[i].Amendments = row.Corrections
		}
		if len(row.CostShares) > 0 {
			invoices[i].CostShares = row.CostShares
		}
	}
	return invoices, withCareMealPositions(rows, invoices)
}

type careRunHoldRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_run_holds"`

	ID             int64                                 `bun:"id,pk,autoincrement"`
	OrganizationID int64                                 `bun:"organization_id"`
	RunID          int64                                 `bun:"run_id"`
	Position       int                                   `bun:"position"`
	Reason         string                                `bun:"reason"`
	SchoolIDs      []int64                               `bun:"school_ids,array"`
	ChildFirstName string                                `bun:"child_first_name"`
	ChildLastName  string                                `bun:"child_last_name"`
	Contracts      []parentbilling.CareDuplicateContract `bun:"contracts,type:jsonb"`
	Slots          []parentbilling.CareHeldSlot          `bun:"slots,type:jsonb"`
}

// InsertCareRunHolds stores the children a release held back as they are
// now (ADR 0158 E5); the rows never change.
func (s *Store) InsertCareRunHolds(ctx context.Context, organizationID, runID int64, holds []parentbilling.CareRunHold) error {
	if len(holds) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	rows := make([]careRunHoldRow, len(holds))
	for i, hold := range holds {
		schools := []int64{}
		for _, contract := range hold.Contracts {
			if !slices.Contains(schools, contract.SchoolID) {
				schools = append(schools, contract.SchoolID)
			}
		}
		slots := hold.Slots
		if slots == nil {
			slots = []parentbilling.CareHeldSlot{}
		}
		rows[i] = careRunHoldRow{OrganizationID: organizationID, RunID: runID, Position: hold.Position, Reason: hold.Reason,
			SchoolIDs: schools, ChildFirstName: hold.ChildFirstName, ChildLastName: hold.ChildLastName,
			Contracts: hold.Contracts, Slots: slots}
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return fmt.Errorf("insert care run holds: %w", err)
	}
	return nil
}

// CareRunHolds reads the children a release held back, in their order.
func (s *Store) CareRunHolds(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareRunHold, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careRunHoldRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).
		OrderExpr("position").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read care run holds: %w", err)
	}
	holds := make([]parentbilling.CareRunHold, len(rows))
	for i, row := range rows {
		holds[i] = parentbilling.CareRunHold{Position: row.Position, Reason: row.Reason, ChildFirstName: row.ChildFirstName,
			ChildLastName: row.ChildLastName, Contracts: row.Contracts, Slots: row.Slots}
	}
	return holds, nil
}

// careDocumentKind is the kind a monthly invoice is issued as; one without a
// kind is issued as before #4688, an invoice.
func careDocumentKind(kind string) string {
	if kind == "" {
		return parentbilling.DocumentKindInvoice
	}
	return kind
}

// snapshot is the copied payment of the row (#4689, #4695).
func (r careInvoiceRow) snapshot() paymentSnapshot {
	return paymentSnapshot{PaymentMethod: r.PaymentMethod, DueOn: r.DueOn, DueSource: r.DueSource, MandateID: r.MandateID,
		MandateReference: r.MandateReference, MandateSignedOn: r.MandateSignedOn, CreditorID: r.CreditorID,
		DebtorIBANMasked: r.DebtorIBANMasked, SequenceType: r.SequenceType, DebitGap: r.DebitGap}
}

// setPayment copies a payment onto the row.
func (r *careInvoiceRow) setPayment(s paymentSnapshot) {
	r.PaymentMethod, r.DueOn, r.DueSource, r.MandateID = s.PaymentMethod, s.DueOn, s.DueSource, s.MandateID
	r.MandateReference, r.MandateSignedOn, r.CreditorID = s.MandateReference, s.MandateSignedOn, s.CreditorID
	r.DebtorIBANMasked, r.SequenceType, r.DebitGap = s.DebtorIBANMasked, s.SequenceType, s.DebitGap
}
