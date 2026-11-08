package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Jahresbescheide of the monthly care (#6219, ADR 0180). Every query
// names the Träger next to RLS; the scope tests run them as the admin role
// without RLS.

const (
	careNoticeNumbersTable      = "parent_billing.care_notices"
	openCareNoticeRunYearIndex  = "uq_care_notice_runs_calculated_year"
	careNoticeAmendsIndex       = "uq_care_notices_amends"
	careNoticeFirstOfYearIndex  = "uq_care_notices_first"
	careNoticeRunSelectColumnsQ = `id, billing_year, period_from, period_until, status, school_ids, notice_date, created_at,
		released_at, issued_on, notice_count, total_cents`
)

type careNoticeRunRow struct {
	bun.BaseModel      `bun:"table:parent_billing.care_notice_runs,alias:care_notice_run"`
	ID                 int64          `bun:"id,pk,autoincrement"`
	OrganizationID     int64          `bun:"organization_id,notnull"`
	Year               int            `bun:"billing_year,notnull"`
	PeriodFrom         calendar.Date  `bun:"period_from,notnull,type:date"`
	PeriodUntil        calendar.Date  `bun:"period_until,notnull,type:date"`
	Status             string         `bun:"status,notnull"`
	SchoolIDs          []int64        `bun:"school_ids,array,notnull"`
	NoticeDate         *calendar.Date `bun:"notice_date,type:date"`
	CreatedByAccountID *int64         `bun:"created_by_account_id"`
	CreatedAt          time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	ReleasedAt         *time.Time     `bun:"released_at"`
	IssuedOn           *calendar.Date `bun:"issued_on,type:date"`
	NoticeCount        *int           `bun:"notice_count"`
	TotalCents         *int64         `bun:"total_cents"`
}

func (r careNoticeRunRow) run() parentbilling.CareNoticeRun {
	return parentbilling.CareNoticeRun{ID: r.ID, Year: r.Year, PeriodFrom: r.PeriodFrom, PeriodUntil: r.PeriodUntil, Status: r.Status,
		SchoolIDs: r.SchoolIDs, NoticeDate: r.NoticeDate, CreatedAt: r.CreatedAt, ReleasedAt: r.ReleasedAt, IssuedOn: r.IssuedOn,
		NoticeCount: r.NoticeCount, TotalCents: r.TotalCents}
}

type careNoticeRow struct {
	bun.BaseModel  `bun:"table:parent_billing.care_notices,alias:care_notice"`
	ID             int64                          `bun:"id,pk,autoincrement"`
	OrganizationID int64                          `bun:"organization_id,notnull"`
	RunID          int64                          `bun:"run_id,notnull"`
	Number         string                         `bun:"number,notnull"`
	NumberYear     int                            `bun:"number_year,notnull"`
	NumberSequence int64                          `bun:"number_sequence,notnull"`
	IssuedOn       calendar.Date                  `bun:"issued_on,notnull,type:date"`
	Year           int                            `bun:"billing_year,notnull"`
	Kind           string                         `bun:"kind,notnull"`
	AmendsNoticeID *int64                         `bun:"amends_notice_id"`
	CareContractID int64                          `bun:"care_contract_id,notnull"`
	SchoolID       int64                          `bun:"school_id,notnull"`
	SchoolName     string                         `bun:"school_name,notnull"`
	ChildFirstName string                         `bun:"child_first_name,notnull"`
	ChildLastName  string                         `bun:"child_last_name,notnull"`
	StudentID      *int64                         `bun:"student_id"`
	RecipientEmail string                         `bun:"recipient_email,notnull"`
	FamilyEmail    string                         `bun:"family_email,notnull"`
	Recipient      parentbilling.CareRecipient    `bun:"recipient,type:jsonb,notnull"`
	PaymentMethod  string                         `bun:"payment_method,notnull"`
	PeriodFrom     calendar.Date                  `bun:"period_from,notnull,type:date"`
	PeriodUntil    calendar.Date                  `bun:"period_until,notnull,type:date"`
	Lines          []parentbilling.CareNoticeLine `bun:"lines,type:jsonb,notnull"`
	PreviousCents  int64                          `bun:"previous_cents,notnull"`
	TotalCents     int64                          `bun:"total_cents,notnull"`
	DeltaCents     int64                          `bun:"delta_cents,notnull"`
	LegalRemedy    string                         `bun:"legal_remedy,notnull"`
}

// careNoticeSnapshot is what a month of a notice bills, as the monthly
// computation billed it: the invoice of the month and the cents the rule
// chain allocated to each position, which its JSON leaves out.
type careNoticeSnapshot struct {
	Invoice parentbilling.CareInvoice `json:"invoice"`
	Payable []int64                   `json:"payable"`
}

type careNoticeMonthRow struct {
	bun.BaseModel  `bun:"table:parent_billing.care_notice_months,alias:care_notice_month"`
	ID             int64              `bun:"id,pk,autoincrement"`
	OrganizationID int64              `bun:"organization_id,notnull"`
	NoticeID       int64              `bun:"notice_id,notnull"`
	CareContractID int64              `bun:"care_contract_id,notnull"`
	BillingMonth   calendar.Date      `bun:"billing_month,notnull,type:date"`
	DueOn          calendar.Date      `bun:"due_on,notnull,type:date"`
	ImmediatelyDue bool               `bun:"immediately_due,notnull"`
	PreviousCents  int64              `bun:"previous_cents,notnull"`
	AmountCents    int64              `bun:"amount_cents,notnull"`
	Snapshot       careNoticeSnapshot `bun:"snapshot,type:jsonb,notnull"`
}

// ListCareNoticeRuns lists the notice runs, the latest year first.
func (s *Store) ListCareNoticeRuns(ctx context.Context, organizationID int64) ([]parentbilling.CareNoticeRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careNoticeRunRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		OrderExpr("billing_year DESC, id DESC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list notice runs: %w", err)
	}
	runs := make([]parentbilling.CareNoticeRun, len(rows))
	for i, row := range rows {
		runs[i] = row.run()
	}
	return runs, nil
}

// InsertCareNoticeRun stores a calculated run; a second calculated run of
// the year is ErrCareNoticeRunOpen, also when two start at once.
func (s *Store) InsertCareNoticeRun(ctx context.Context, organizationID, accountID int64, run parentbilling.CareNoticeRun) (parentbilling.CareNoticeRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareNoticeRun{}, err
	}
	row := &careNoticeRunRow{OrganizationID: organizationID, Year: run.Year, PeriodFrom: run.PeriodFrom, PeriodUntil: run.PeriodUntil,
		Status: parentbilling.RunCalculated, SchoolIDs: run.SchoolIDs, NoticeDate: run.NoticeDate, CreatedByAccountID: &accountID}
	if _, err := db.NewInsert().Model(row).ExcludeColumn("released_at", "issued_on", "notice_count", "total_cents").
		Returning("id, created_at").Exec(ctx); err != nil {
		if constraintOf(err) == openCareNoticeRunYearIndex {
			return parentbilling.CareNoticeRun{}, parentbilling.ErrCareNoticeRunOpen
		}
		return parentbilling.CareNoticeRun{}, fmt.Errorf("insert notice run: %w", err)
	}
	return row.run(), nil
}

// CareNoticeRun reads a run, for update when lock is set; a missing one is
// ErrCareNoticeRunNotFound.
func (s *Store) CareNoticeRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.CareNoticeRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareNoticeRun{}, err
	}
	row := &careNoticeRunRow{}
	query := db.NewSelect().Model(row).Where("organization_id = ?", organizationID).Where("id = ?", runID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.CareNoticeRun{}, parentbilling.ErrCareNoticeRunNotFound
		}
		return parentbilling.CareNoticeRun{}, fmt.Errorf("read notice run: %w", err)
	}
	return row.run(), nil
}

// InsertCareNoticeRunEvent records an action of the run's trail.
func (s *Store) InsertCareNoticeRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if details == nil {
		details = map[string]any{}
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("encode notice run event: %w", err)
	}
	var actor *int64
	if accountID > 0 {
		actor = &accountID
	}
	if _, err := db.NewRaw(`INSERT INTO parent_billing.care_notice_run_events (organization_id, run_id, action, actor_account_id, details)
		VALUES (?, ?, ?, ?, ?::jsonb)`, organizationID, runID, action, actor, string(encoded)).Exec(ctx); err != nil {
		return fmt.Errorf("insert notice run event: %w", err)
	}
	return nil
}

// EffectiveCareNotices reads the notices of the year nobody amended, with
// their installments and months in the order of the months.
func (s *Store) EffectiveCareNotices(ctx context.Context, organizationID int64, year int) ([]parentbilling.CareNotice, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careNoticeRow
	if err := db.NewSelect().Model(&rows).
		Where("care_notice.organization_id = ?", organizationID).Where("care_notice.billing_year = ?", year).
		Where(`NOT EXISTS (SELECT 1 FROM parent_billing.care_notices AS amendment
			WHERE amendment.organization_id = care_notice.organization_id AND amendment.amends_notice_id = care_notice.id)`).
		OrderExpr("care_notice.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read effective notices: %w", err)
	}
	return s.withNoticeMonths(ctx, db, organizationID, rows)
}

// CareNoticeRunNotices reads the issued notices of a run with the notice
// each amends and the one that amends it.
func (s *Store) CareNoticeRunNotices(ctx context.Context, organizationID, runID int64) ([]parentbilling.CareNotice, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careNoticeRow
	if err := db.NewSelect().Model(&rows).Where("care_notice.organization_id = ?", organizationID).
		Where("care_notice.run_id = ?", runID).OrderExpr("care_notice.number_sequence").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read run notices: %w", err)
	}
	notices, err := s.withNoticeMonths(ctx, db, organizationID, rows)
	if err != nil {
		return nil, err
	}
	return notices, noticeReferences(ctx, db, organizationID, notices)
}

// withNoticeMonths reads the months of the notices in one statement.
func (s *Store) withNoticeMonths(ctx context.Context, db bun.IDB, organizationID int64, rows []careNoticeRow) ([]parentbilling.CareNotice, error) {
	notices := make([]parentbilling.CareNotice, len(rows))
	index := map[int64]int{}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		notices[i] = row.notice()
		index[row.ID] = i
		ids[i] = row.ID
	}
	if len(ids) == 0 {
		return notices, nil
	}
	var months []careNoticeMonthRow
	if err := db.NewSelect().Model(&months).Where("organization_id = ?", organizationID).
		Where("notice_id = ANY(?)", pgdialect.Array(ids)).OrderExpr("notice_id, billing_month").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read notice months: %w", err)
	}
	for _, month := range months {
		notice := &notices[index[month.NoticeID]]
		notice.Installments = append(notice.Installments, parentbilling.CareNoticeInstallment{Month: month.BillingMonth,
			DueOn: month.DueOn, ImmediatelyDue: month.ImmediatelyDue, PreviousCents: month.PreviousCents,
			AmountCents: month.AmountCents, DeltaCents: month.AmountCents - month.PreviousCents})
		invoice := month.Snapshot.Invoice
		for i := range invoice.Positions {
			if i < len(month.Snapshot.Payable) {
				invoice.Positions[i].PayableCents = month.Snapshot.Payable[i]
			}
		}
		notice.Months = append(notice.Months, invoice)
	}
	return notices, nil
}

func (r careNoticeRow) notice() parentbilling.CareNotice {
	issuedOn := r.IssuedOn
	notice := parentbilling.CareNotice{ID: r.ID, Number: r.Number, IssuedOn: &issuedOn, Kind: r.Kind, ContractID: r.CareContractID,
		SchoolID: r.SchoolID, SchoolName: r.SchoolName, ChildFirstName: r.ChildFirstName, ChildLastName: r.ChildLastName,
		StudentID: r.StudentID, Recipient: r.Recipient, FamilyEmail: r.FamilyEmail, PaymentMethod: r.PaymentMethod, Year: r.Year, PeriodFrom: r.PeriodFrom,
		PeriodUntil: r.PeriodUntil, Lines: r.Lines, PreviousCents: r.PreviousCents, TotalCents: r.TotalCents,
		DeltaCents: r.DeltaCents, LegalRemedy: r.LegalRemedy, Installments: []parentbilling.CareNoticeInstallment{},
		Months: []parentbilling.CareInvoice{}}
	if notice.Lines == nil {
		notice.Lines = []parentbilling.CareNoticeLine{}
	}
	if r.AmendsNoticeID != nil {
		notice.Amends = &parentbilling.CareNoticeReference{ID: *r.AmendsNoticeID}
	}
	return notice
}

// noticeReferences fills number and date of the notice each notice amends
// and names the notice that amends it, in one statement.
func noticeReferences(ctx context.Context, db bun.IDB, organizationID int64, notices []parentbilling.CareNotice) error {
	if len(notices) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(notices)*2)
	for _, notice := range notices {
		ids = append(ids, notice.ID)
		if notice.Amends != nil {
			ids = append(ids, notice.Amends.ID)
		}
	}
	var rows []struct {
		ID             int64         `bun:"id"`
		Number         string        `bun:"number"`
		IssuedOn       calendar.Date `bun:"issued_on"`
		AmendsNoticeID *int64        `bun:"amends_notice_id"`
	}
	if err := db.NewRaw(`SELECT id, number, issued_on, amends_notice_id FROM parent_billing.care_notices
		WHERE organization_id = ? AND (id = ANY(?) OR amends_notice_id = ANY(?))`,
		organizationID, pgdialect.Array(ids), pgdialect.Array(ids)).Scan(ctx, &rows); err != nil {
		return fmt.Errorf("read notice references: %w", err)
	}
	for i := range notices {
		for _, row := range rows {
			applyNoticeReference(&notices[i], row.ID, row.Number, row.IssuedOn, row.AmendsNoticeID)
		}
	}
	return nil
}

// applyNoticeReference fills the notice a notice amends and names the one
// that amends it, from one row of the references.
func applyNoticeReference(notice *parentbilling.CareNotice, id int64, number string, issuedOn calendar.Date, amends *int64) {
	reference := &parentbilling.CareNoticeReference{ID: id, Number: number, IssuedOn: &issuedOn}
	if notice.Amends != nil && id == notice.Amends.ID {
		notice.Amends = reference
	}
	if amends != nil && *amends == notice.ID {
		notice.AmendedBy = reference
	}
}

// IssueCareNotices stores the numbered notices of a release with their
// months; the i-th notice has the sequence first+i of the counter of the
// year. A number a holiday document, a monthly invoice, a Storno or another
// notice carries is ErrInvoiceNumberTaken; a notice amended in the meantime
// or a second first notice of a contract and year is ErrBillingRunChanged.
func (s *Store) IssueCareNotices(ctx context.Context, organizationID int64, run parentbilling.CareNoticeRun, numberYear int, first int64,
	notices []parentbilling.CareNotice,
) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	numbers := make([]string, len(notices))
	for i, notice := range notices {
		numbers[i] = notice.Number
	}
	for _, other := range []string{holidayNumbersTable, careNumbersTable} {
		if err := claimInvoiceNumbers(ctx, db, organizationID, numbers, other); err != nil {
			return err
		}
	}
	for i, notice := range notices {
		row := noticeRow(organizationID, run.ID, numberYear, first+int64(i), notice)
		if err := insertNotice(ctx, db, row); err != nil {
			return err
		}
		months := noticeMonthRows(organizationID, row.ID, notice)
		if len(months) == 0 {
			continue
		}
		if _, err := db.NewInsert().Model(&months).Exec(ctx); err != nil {
			return fmt.Errorf("insert notice months: %w", err)
		}
	}
	return nil
}

// insertNotice stores a notice; a notice amended in the meantime or a second
// first notice of a contract and year is ErrBillingRunChanged, a number
// taken in between ErrInvoiceNumberTaken.
func insertNotice(ctx context.Context, db bun.IDB, row *careNoticeRow) error {
	if _, err := db.NewInsert().Model(row).Returning("id").Exec(ctx); err != nil {
		switch constraintOf(err) {
		case careNoticeAmendsIndex, careNoticeFirstOfYearIndex:
			return parentbilling.ErrBillingRunChanged
		case "uq_care_notices_number":
			return parentbilling.ErrInvoiceNumberTaken
		}
		return fmt.Errorf("insert notice: %w", err)
	}
	return nil
}

// noticeMonthRows are the months of a notice with the snapshot of what each
// bills; the snapshot carries the notice number, so the monthly run names
// the notice that assesses the month.
func noticeMonthRows(organizationID, noticeID int64, notice parentbilling.CareNotice) []careNoticeMonthRow {
	months := make([]careNoticeMonthRow, len(notice.Installments))
	for j, installment := range notice.Installments {
		snapshot := careNoticeSnapshot{Invoice: parentbilling.CareInvoice{}, Payable: []int64{}}
		if j < len(notice.Months) {
			snapshot.Invoice = notice.Months[j]
			for _, position := range notice.Months[j].Positions {
				snapshot.Payable = append(snapshot.Payable, position.PayableCents)
			}
		}
		snapshot.Invoice.Number, snapshot.Invoice.DocumentKind = notice.Number, parentbilling.DocumentKindFeeNotice
		months[j] = careNoticeMonthRow{OrganizationID: organizationID, NoticeID: noticeID, CareContractID: notice.ContractID,
			BillingMonth: installment.Month, DueOn: installment.DueOn, ImmediatelyDue: installment.ImmediatelyDue,
			PreviousCents: installment.PreviousCents, AmountCents: installment.AmountCents, Snapshot: snapshot}
	}
	return months
}

// noticeFamily is the family of the contract party the sibling rules count
// by (#4689); the payer of a deviation only receives the notice.
func noticeFamily(notice parentbilling.CareNotice) string {
	if notice.FamilyEmail != "" {
		return notice.FamilyEmail
	}
	return parentbilling.FamilyKey(notice.Recipient.Email)
}

func noticeRow(organizationID, runID int64, numberYear int, sequence int64, notice parentbilling.CareNotice) *careNoticeRow {
	row := &careNoticeRow{OrganizationID: organizationID, RunID: runID, Number: notice.Number, NumberYear: numberYear,
		NumberSequence: sequence, IssuedOn: *notice.IssuedOn, Year: notice.Year, Kind: notice.Kind, CareContractID: notice.ContractID,
		SchoolID: notice.SchoolID, SchoolName: notice.SchoolName, ChildFirstName: notice.ChildFirstName,
		ChildLastName: notice.ChildLastName, StudentID: notice.StudentID, RecipientEmail: notice.Recipient.Email,
		FamilyEmail: noticeFamily(notice), Recipient: notice.Recipient,
		PaymentMethod: notice.PaymentMethod, PeriodFrom: notice.PeriodFrom, PeriodUntil: notice.PeriodUntil, Lines: notice.Lines,
		PreviousCents: notice.PreviousCents, TotalCents: notice.TotalCents, DeltaCents: notice.DeltaCents, LegalRemedy: notice.LegalRemedy}
	if notice.Amends != nil {
		row.AmendsNoticeID = &notice.Amends.ID
	}
	if row.PaymentMethod == "" {
		row.PaymentMethod = parentbilling.PaymentTransfer
	}
	return row
}

// MarkCareNoticeRunReleased closes a calculated run with its totals.
func (s *Store) MarkCareNoticeRunReleased(ctx context.Context, organizationID, runID int64, issuedOn calendar.Date, count int, totalCents int64) (parentbilling.CareNoticeRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareNoticeRun{}, err
	}
	row := &careNoticeRunRow{}
	if err := db.NewRaw(`UPDATE parent_billing.care_notice_runs
		SET status = 'released', released_at = CURRENT_TIMESTAMP, issued_on = ?, notice_count = ?, total_cents = ?
		WHERE organization_id = ? AND id = ? AND status = 'calculated'
		RETURNING `+careNoticeRunSelectColumnsQ, issuedOn, count, totalCents, organizationID, runID).Scan(ctx, row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.CareNoticeRun{}, parentbilling.ErrBillingRunReleased
		}
		return parentbilling.CareNoticeRun{}, fmt.Errorf("release notice run: %w", err)
	}
	row.OrganizationID = organizationID
	return row.run(), nil
}
