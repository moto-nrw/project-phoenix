package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The billing runs, number ranges and invoices (#4668, ADR 0119). The
// request role may insert and read an invoice, a correction and an event,
// never change or delete them; a released run never changes.

const (
	// openRunIndex keeps a period to one calculated run.
	openRunIndex = "uq_runs_calculated_period"
	// invoiceNumberConstraint keeps the numbers of a Träger unique.
	invoiceNumberConstraint = "uq_documents_number"
)

type runRow struct {
	bun.BaseModel `bun:"table:parent_billing.runs"`

	ID              int64          `bun:"id,pk,autoincrement"`
	OrganizationID  int64          `bun:"organization_id"`
	Service         string         `bun:"service"`
	HolidayPeriodID int64          `bun:"holiday_period_id"`
	PeriodName      string         `bun:"period_name"`
	ServiceFrom     calendar.Date  `bun:"service_from,type:date"`
	ServiceUntil    calendar.Date  `bun:"service_until,type:date"`
	Status          string         `bun:"status"`
	CreatedAt       time.Time      `bun:"created_at,nullzero,default:current_timestamp"`
	ReleasedAt      *time.Time     `bun:"released_at"`
	IssuedOn        *calendar.Date `bun:"issued_on,type:date"`
	DocumentCount   *int           `bun:"document_count"`
	TotalCents      *int64         `bun:"total_cents"`
}

func (r runRow) run() parentbilling.BillingRun {
	return parentbilling.BillingRun{
		ID: r.ID, Service: r.Service, HolidayPeriodID: r.HolidayPeriodID, PeriodName: r.PeriodName,
		ServiceFrom: r.ServiceFrom, ServiceUntil: r.ServiceUntil, Status: r.Status, CreatedAt: r.CreatedAt,
		ReleasedAt: r.ReleasedAt, IssuedOn: r.IssuedOn, DocumentCount: r.DocumentCount, TotalCents: r.TotalCents,
	}
}

type correctionRow struct {
	bun.BaseModel `bun:"table:parent_billing.run_corrections"`

	ID                         int64     `bun:"id,pk,autoincrement"`
	OrganizationID             int64     `bun:"organization_id"`
	RunID                      int64     `bun:"run_id"`
	HolidayPeriodID            int64     `bun:"holiday_period_id"`
	HolidayRegistrationID      int64     `bun:"holiday_registration_id"`
	HolidayRegistrationChildID int64     `bun:"holiday_registration_child_id"`
	AmountCents                int64     `bun:"amount_cents"`
	TotalBeforeCents           int64     `bun:"total_before_cents"`
	TotalAfterCents            int64     `bun:"total_after_cents"`
	Reason                     string    `bun:"reason"`
	CreatedByAccountID         *int64    `bun:"created_by_account_id"`
	CreatedAt                  time.Time `bun:"created_at,nullzero,default:current_timestamp"`
}

type runEventRow struct {
	bun.BaseModel `bun:"table:parent_billing.run_events"`

	ID             int64          `bun:"id,pk,autoincrement"`
	OrganizationID int64          `bun:"organization_id"`
	RunID          int64          `bun:"run_id"`
	Action         string         `bun:"action"`
	ActorAccountID *int64         `bun:"actor_account_id"`
	Details        map[string]any `bun:"details,type:jsonb"`
}

type numberRangeRow struct {
	bun.BaseModel `bun:"table:parent_billing.number_ranges"`

	ID             int64     `bun:"id,pk,autoincrement"`
	OrganizationID int64     `bun:"organization_id"`
	Service        string    `bun:"service"`
	Pattern        string    `bun:"pattern"`
	UpdatedAt      time.Time `bun:"updated_at,nullzero,default:current_timestamp"`
}

func (r numberRangeRow) numberRange() parentbilling.NumberRange {
	updatedAt := r.UpdatedAt
	return parentbilling.NumberRange{Service: r.Service, Pattern: r.Pattern, UpdatedAt: &updatedAt}
}

type documentRow struct {
	bun.BaseModel `bun:"table:parent_billing.documents"`

	ID                    int64         `bun:"id,pk,autoincrement"`
	OrganizationID        int64         `bun:"organization_id"`
	RunID                 int64         `bun:"run_id"`
	Service               string        `bun:"service"`
	Number                string        `bun:"number"`
	NumberYear            int           `bun:"number_year"`
	NumberSequence        int64         `bun:"number_sequence"`
	IssuedOn              calendar.Date `bun:"issued_on,type:date"`
	HolidayPeriodID       int64         `bun:"holiday_period_id"`
	HolidayRegistrationID int64         `bun:"holiday_registration_id"`
	RecipientFirstName    string        `bun:"recipient_first_name"`
	RecipientLastName     string        `bun:"recipient_last_name"`
	RecipientEmail        string        `bun:"recipient_email"`
	ServiceFrom           calendar.Date `bun:"service_from,type:date"`
	ServiceUntil          calendar.Date `bun:"service_until,type:date"`
	TotalCents            int64         `bun:"total_cents"`
}

type documentPositionRow struct {
	bun.BaseModel `bun:"table:parent_billing.document_positions"`

	ID                         int64          `bun:"id,pk,autoincrement"`
	OrganizationID             int64          `bun:"organization_id"`
	DocumentID                 int64          `bun:"document_id"`
	Position                   int            `bun:"position"`
	HolidayRegistrationChildID int64          `bun:"holiday_registration_child_id"`
	ChildFirstName             string         `bun:"child_first_name"`
	ChildLastName              string         `bun:"child_last_name"`
	Kind                       string         `bun:"kind"`
	AmountCents                int64          `bun:"amount_cents"`
	NetCents                   int64          `bun:"net_cents"`
	TaxCents                   int64          `bun:"tax_cents"`
	WeekStartsOn               *calendar.Date `bun:"week_starts_on,type:date"`
	WeekEndsOn                 *calendar.Date `bun:"week_ends_on,type:date"`
	TimeModelName              string         `bun:"time_model_name"`
	ChildRank                  *int           `bun:"child_rank"`
	DiscountBasisPoints        *int           `bun:"discount_basis_points"`
	DiscountCents              *int64         `bun:"discount_cents"`
	Note                       string         `bun:"note"`
}

// ListBillingRuns lists the runs of the Träger, newest first.
func (s *Store) ListBillingRuns(ctx context.Context, organizationID int64) ([]parentbilling.BillingRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []runRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		OrderExpr("created_at DESC, id DESC").Scan(ctx); err != nil {
		return nil, err
	}
	runs := make([]parentbilling.BillingRun, len(rows))
	for i, row := range rows {
		runs[i] = row.run()
	}
	return runs, nil
}

// InsertBillingRun stores a calculated run.
func (s *Store) InsertBillingRun(ctx context.Context, organizationID int64, run parentbilling.BillingRun) (parentbilling.BillingRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.BillingRun{}, err
	}
	row := &runRow{
		OrganizationID: organizationID, Service: run.Service, HolidayPeriodID: run.HolidayPeriodID, PeriodName: run.PeriodName,
		ServiceFrom: run.ServiceFrom, ServiceUntil: run.ServiceUntil, Status: parentbilling.RunCalculated,
	}
	if _, err := db.NewInsert().Model(row).Returning("id, created_at").Exec(ctx); err != nil {
		if constraintOf(err) == openRunIndex {
			return parentbilling.BillingRun{}, parentbilling.ErrBillingRunOpen
		}
		return parentbilling.BillingRun{}, err
	}
	return row.run(), nil
}

// BillingRun reads a run.
func (s *Store) BillingRun(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error) {
	return s.selectRun(ctx, organizationID, runID, false)
}

// LockBillingRun reads a run and holds it until the transaction ends.
func (s *Store) LockBillingRun(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error) {
	return s.selectRun(ctx, organizationID, runID, true)
}

func (s *Store) selectRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.BillingRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.BillingRun{}, err
	}
	row := &runRow{}
	query := db.NewSelect().Model(row).Where("organization_id = ?", organizationID).Where("id = ?", runID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.BillingRun{}, parentbilling.ErrBillingRunNotFound
		}
		return parentbilling.BillingRun{}, err
	}
	return row.run(), nil
}

// PeriodContributions reads the frozen contributions of a period with
// their lines, by registration and in the order of the form.
func (s *Store) PeriodContributions(ctx context.Context, organizationID, periodID int64) (map[int64]parentbilling.HolidayContribution, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayContributionRow
	if err := db.NewSelect().Model(&rows).
		Where("organization_id = ?", organizationID).Where("holiday_period_id = ?", periodID).
		OrderExpr("holiday_registration_id, holiday_registration_child_id").Scan(ctx); err != nil {
		return nil, err
	}
	contributions := map[int64]parentbilling.HolidayContribution{}
	if len(rows) == 0 {
		return contributions, nil
	}
	lines, err := contributionLines(ctx, db, organizationID, rows)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		contribution := contributions[row.HolidayRegistrationID]
		childLines := lines[row.ID]
		if childLines == nil {
			childLines = []parentbilling.HolidayContributionLine{}
		}
		contribution.Children = append(contribution.Children, parentbilling.HolidayChildContribution{
			ChildID: row.HolidayRegistrationChildID, Position: len(contribution.Children) + 1, Lines: childLines, TotalCents: row.TotalCents,
		})
		contribution.TotalCents += row.TotalCents
		contribution.Frozen = true
		contributions[row.HolidayRegistrationID] = contribution
	}
	return contributions, nil
}

// IssuedRegistrations names the registrations of the period an invoice of
// the service bills.
func (s *Store) IssuedRegistrations(ctx context.Context, organizationID int64, service string, periodID int64) (map[int64]bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var ids []int64
	if err := db.NewSelect().TableExpr("parent_billing.documents").Column("holiday_registration_id").
		Where("organization_id = ?", organizationID).Where("service = ?", service).Where("holiday_period_id = ?", periodID).
		Scan(ctx, &ids); err != nil {
		return nil, err
	}
	issued := make(map[int64]bool, len(ids))
	for _, id := range ids {
		issued[id] = true
	}
	return issued, nil
}

// RunCorrections lists the corrections of a run in their order.
func (s *Store) RunCorrections(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingCorrection, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []correctionRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).
		OrderExpr("id").Scan(ctx); err != nil {
		return nil, err
	}
	corrections := make([]parentbilling.BillingCorrection, len(rows))
	for i, row := range rows {
		corrections[i] = parentbilling.BillingCorrection{
			ID: row.ID, ChildID: row.HolidayRegistrationChildID, AmountCents: row.AmountCents, TotalBeforeCents: row.TotalBeforeCents,
			TotalAfterCents: row.TotalAfterCents, Reason: row.Reason, CreatedByAccountID: row.CreatedByAccountID, CreatedAt: row.CreatedAt,
		}
	}
	return corrections, nil
}

// InsertRunCorrection adds a correction to a calculated run.
func (s *Store) InsertRunCorrection(ctx context.Context, organizationID int64, run parentbilling.BillingRun, registrationID, accountID int64, correction parentbilling.BillingCorrection) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	_, err = db.NewInsert().Model(&correctionRow{
		OrganizationID: organizationID, RunID: run.ID, HolidayPeriodID: run.HolidayPeriodID, HolidayRegistrationID: registrationID,
		HolidayRegistrationChildID: correction.ChildID, AmountCents: correction.AmountCents, TotalBeforeCents: correction.TotalBeforeCents,
		TotalAfterCents: correction.TotalAfterCents, Reason: correction.Reason, CreatedByAccountID: &accountID,
	}).Exec(ctx)
	return err
}

// InsertRunEvent appends an entry to the trail of a run.
func (s *Store) InsertRunEvent(ctx context.Context, organizationID, runID, accountID int64, action string, details map[string]any) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if details == nil {
		details = map[string]any{}
	}
	_, err = db.NewInsert().Model(&runEventRow{
		OrganizationID: organizationID, RunID: runID, Action: action, ActorAccountID: &accountID, Details: details,
	}).Exec(ctx)
	return err
}

// ListNumberRanges lists the number ranges of the Träger by service.
func (s *Store) ListNumberRanges(ctx context.Context, organizationID int64) ([]parentbilling.NumberRange, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []numberRangeRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).OrderExpr("service").Scan(ctx); err != nil {
		return nil, err
	}
	ranges := make([]parentbilling.NumberRange, len(rows))
	for i, row := range rows {
		ranges[i] = row.numberRange()
	}
	return ranges, nil
}

// SetNumberRange stores or replaces the pattern of a service.
func (s *Store) SetNumberRange(ctx context.Context, organizationID int64, service, pattern string) (parentbilling.NumberRange, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.NumberRange{}, err
	}
	row := &numberRangeRow{OrganizationID: organizationID, Service: service, Pattern: pattern}
	if _, err := db.NewInsert().Model(row).
		On("CONFLICT (organization_id, service) DO UPDATE").Set("pattern = EXCLUDED.pattern").
		Returning("id, updated_at").Exec(ctx); err != nil {
		return parentbilling.NumberRange{}, err
	}
	return row.numberRange(), nil
}

// ReserveNumbers raises the counter by count in one statement. The row stays
// locked until the transaction ends; a rollback returns the numbers, so the
// numbers of committed invoices have no gap.
func (s *Store) ReserveNumbers(ctx context.Context, organizationID int64, service string, year, count int) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	var last int64
	if err := db.NewRaw(`INSERT INTO parent_billing.number_range_counters AS counter (organization_id, service, year, last_number)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (organization_id, service, year)
		DO UPDATE SET last_number = counter.last_number + EXCLUDED.last_number, updated_at = CURRENT_TIMESTAMP
		RETURNING last_number`, organizationID, service, year, count).Scan(ctx, &last); err != nil {
		return 0, err
	}
	return last - int64(count) + 1, nil
}

// IssueDocuments stores the invoices of a release with their positions.
func (s *Store) IssueDocuments(ctx context.Context, organizationID int64, run parentbilling.BillingRun, year int, first int64, documents []parentbilling.BillingDocument) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(documents) == 0 {
		return nil
	}
	rows := make([]documentRow, len(documents))
	for i, document := range documents {
		rows[i] = documentRow{
			OrganizationID: organizationID, RunID: run.ID, Service: run.Service, Number: document.Number,
			NumberYear: year, NumberSequence: first + int64(i), IssuedOn: *document.IssuedOn,
			HolidayPeriodID: run.HolidayPeriodID, HolidayRegistrationID: document.RegistrationID,
			RecipientFirstName: document.RecipientFirstName, RecipientLastName: document.RecipientLastName,
			RecipientEmail: document.RecipientEmail, ServiceFrom: document.ServiceFrom, ServiceUntil: document.ServiceUntil,
			TotalCents: document.TotalCents,
		}
	}
	if _, err := db.NewInsert().Model(&rows).Returning("id").Exec(ctx); err != nil {
		if constraintOf(err) == invoiceNumberConstraint {
			return parentbilling.ErrInvoiceNumberTaken
		}
		return err
	}
	var positions []documentPositionRow
	for i, document := range documents {
		number := 0
		for _, child := range document.Children {
			for _, position := range child.Positions {
				number++
				positions = append(positions, documentPositionRow{
					OrganizationID: organizationID, DocumentID: rows[i].ID, Position: number,
					HolidayRegistrationChildID: child.ChildID, ChildFirstName: child.FirstName, ChildLastName: child.LastName,
					Kind: position.Kind, AmountCents: position.AmountCents, NetCents: position.AmountCents,
					WeekStartsOn: position.WeekStartsOn, WeekEndsOn: position.WeekEndsOn, TimeModelName: position.TimeModelName,
					ChildRank: position.ChildRank, DiscountBasisPoints: position.DiscountBasisPoints,
					DiscountCents: position.DiscountCents, Note: position.Note,
				})
			}
		}
	}
	if len(positions) == 0 {
		return nil
	}
	_, err = db.NewInsert().Model(&positions).Exec(ctx)
	return err
}

// MarkRunReleased closes a calculated run with its issue date and sums.
func (s *Store) MarkRunReleased(ctx context.Context, organizationID, runID int64, issuedOn calendar.Date, count int, totalCents int64) (parentbilling.BillingRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.BillingRun{}, err
	}
	row := &runRow{}
	if err := db.NewUpdate().Model(row).
		Set("status = ?", parentbilling.RunReleased).Set("released_at = CURRENT_TIMESTAMP").Set("issued_on = ?", issuedOn).
		Set("document_count = ?", count).Set("total_cents = ?", totalCents).
		Where("organization_id = ?", organizationID).Where("id = ?", runID).Where("status = ?", parentbilling.RunCalculated).
		Returning("*").Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.BillingRun{}, parentbilling.ErrBillingRunReleased
		}
		return parentbilling.BillingRun{}, err
	}
	return row.run(), nil
}

// RunDocuments reads the issued invoices of a run in the order of their
// numbers, the positions grouped by child.
func (s *Store) RunDocuments(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingDocument, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []documentRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Where("run_id = ?", runID).
		OrderExpr("number_sequence, id").Scan(ctx); err != nil {
		return nil, err
	}
	documents := make([]parentbilling.BillingDocument, len(rows))
	if len(rows) == 0 {
		return documents, nil
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	var positions []documentPositionRow
	if err := db.NewSelect().Model(&positions).
		Where("organization_id = ?", organizationID).Where("document_id IN (?)", bun.List(ids)).
		OrderExpr("document_id, position").Scan(ctx); err != nil {
		return nil, err
	}
	byDocument := map[int64][]documentPositionRow{}
	for _, position := range positions {
		byDocument[position.DocumentID] = append(byDocument[position.DocumentID], position)
	}
	for i, row := range rows {
		issuedOn := row.IssuedOn
		documents[i] = parentbilling.BillingDocument{
			ID: row.ID, Number: row.Number, IssuedOn: &issuedOn, RegistrationID: row.HolidayRegistrationID,
			RecipientFirstName: row.RecipientFirstName, RecipientLastName: row.RecipientLastName, RecipientEmail: row.RecipientEmail,
			ServiceFrom: row.ServiceFrom, ServiceUntil: row.ServiceUntil, Children: issuedChildren(byDocument[row.ID]),
			TotalCents: row.TotalCents,
		}
	}
	return documents, nil
}

// issuedChildren groups the positions of an invoice by child, in the order
// the children first appear.
func issuedChildren(positions []documentPositionRow) []parentbilling.BillingChild {
	children := []parentbilling.BillingChild{}
	index := map[int64]int{}
	for _, position := range positions {
		at, ok := index[position.HolidayRegistrationChildID]
		if !ok {
			at = len(children)
			index[position.HolidayRegistrationChildID] = at
			children = append(children, parentbilling.BillingChild{
				ChildID: position.HolidayRegistrationChildID, FirstName: position.ChildFirstName, LastName: position.ChildLastName,
				Positions: []parentbilling.BillingPosition{},
			})
		}
		children[at].Positions = append(children[at].Positions, parentbilling.BillingPosition{
			Kind: position.Kind, AmountCents: position.AmountCents, WeekStartsOn: position.WeekStartsOn, WeekEndsOn: position.WeekEndsOn,
			TimeModelName: position.TimeModelName, ChildRank: position.ChildRank, DiscountBasisPoints: position.DiscountBasisPoints,
			DiscountCents: position.DiscountCents, Note: position.Note,
		})
		children[at].TotalCents += position.AmountCents
	}
	return children
}

// contributionLines reads the lines of frozen contributions by
// contribution.
func contributionLines(ctx context.Context, db bun.IDB, organizationID int64, rows []holidayContributionRow) (map[int64][]parentbilling.HolidayContributionLine, error) {
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	var lines []holidayContributionLineRow
	if err := db.NewSelect().Model(&lines).
		Where("organization_id = ?", organizationID).Where("holiday_contribution_id IN (?)", bun.List(ids)).
		OrderExpr("holiday_contribution_id, position").Scan(ctx); err != nil {
		return nil, err
	}
	byContribution := map[int64][]parentbilling.HolidayContributionLine{}
	for _, line := range lines {
		byContribution[line.HolidayContributionID] = append(byContribution[line.HolidayContributionID], line.line())
	}
	return byContribution, nil
}

// constraintOf names the constraint a write violated, if any.
func constraintOf(err error) string {
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.Field('n')
	}
	return ""
}
