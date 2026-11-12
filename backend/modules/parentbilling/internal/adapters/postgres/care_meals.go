package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The Mensa of the monthly run (#4686, migration 1.15.811): the Mensa
// prices of the Träger and the Mensa months on its issued invoices. The
// request role inserts and reads positions and never changes them.

// careMealMonthConstraint bills a child's Mensa month at most once per
// invoice chain: once on a first invoice, once on the reissue of each
// cancelled one (ADR 0165).
const careMealMonthConstraint = "uq_care_meal_positions_child_month"

// notCancelled keeps the Mensa positions of invoices without a Storno: a
// cancelled invoice bills nothing any more, so the next run of its month
// bills its Mensa months again on the reissue (#4684, ADR 0165).
const notCancelled = `NOT EXISTS (SELECT 1 FROM parent_billing.care_invoice_cancellations AS cancellation
	WHERE cancellation.organization_id = care_meal_positions.organization_id
		AND cancellation.invoice_id = care_meal_positions.invoice_id)`

type careMealTariffRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_meal_tariffs"`

	ID                 int64         `bun:"id,pk,autoincrement"`
	OrganizationID     int64         `bun:"organization_id"`
	SchoolID           int64         `bun:"school_id"`
	ValidFrom          calendar.Date `bun:"valid_from,type:date"`
	Mode               string        `bun:"mode"`
	AmountCents        int64         `bun:"amount_cents"`
	CreatedByAccountID *int64        `bun:"created_by_account_id"`
	CreatedAt          time.Time     `bun:"created_at,nullzero,default:current_timestamp"`
}

type careMealPositionRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_meal_positions"`

	ID             int64         `bun:"id,pk,autoincrement"`
	OrganizationID int64         `bun:"organization_id"`
	InvoiceID      int64         `bun:"invoice_id"`
	Replaces       *int64        `bun:"replaces_invoice_id"`
	Position       int           `bun:"position"`
	SchoolID       int64         `bun:"school_id"`
	StudentID      int64         `bun:"student_id"`
	MealMonth      calendar.Date `bun:"meal_month,type:date"`
	Mode           string        `bun:"mode"`
	Quantity       int           `bun:"quantity"`
	UnitCents      int64         `bun:"unit_cents"`
	AmountCents    int64         `bun:"amount_cents"`
	NetCents       int64         `bun:"net_cents"`
	ReleaseVersion int           `bun:"release_version"`
	positionAccountsColumns
}

// CareMealTariffs reads the Mensa prices of the Träger.
func (s *Store) CareMealTariffs(ctx context.Context, organizationID int64) ([]parentbilling.CareMealTariff, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careMealTariffRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		OrderExpr("school_id, valid_from").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read Mensa prices: %w", err)
	}
	tariffs := make([]parentbilling.CareMealTariff, len(rows))
	for i, row := range rows {
		tariffs[i] = parentbilling.CareMealTariff{ID: row.ID, SchoolID: row.SchoolID, ValidFrom: row.ValidFrom, Mode: row.Mode,
			AmountCents: row.AmountCents, CreatedAt: row.CreatedAt}
	}
	return tariffs, nil
}

// ReplaceCareMealTariff sets the Mensa of a school from a month on.
func (s *Store) ReplaceCareMealTariff(ctx context.Context, organizationID, accountID int64, price parentbilling.CareMealPrice) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := s.DeleteCareMealTariff(ctx, organizationID, price.SchoolID, price.ValidFrom); err != nil {
		return err
	}
	row := &careMealTariffRow{OrganizationID: organizationID, SchoolID: price.SchoolID, ValidFrom: price.ValidFrom,
		Mode: price.Mode, AmountCents: price.AmountCents, CreatedByAccountID: &accountID}
	if _, err := db.NewInsert().Model(row).Exec(ctx); err != nil {
		return fmt.Errorf("insert Mensa price: %w", err)
	}
	return nil
}

// DeleteCareMealTariff removes the Mensa price of a school from a month.
func (s *Store) DeleteCareMealTariff(ctx context.Context, organizationID, schoolID int64, validFrom calendar.Date) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	result, err := db.NewDelete().Model((*careMealTariffRow)(nil)).Where("organization_id = ?", organizationID).
		Where("school_id = ?", schoolID).Where("valid_from = ?", validFrom).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete Mensa price: %w", err)
	}
	deleted, err := result.RowsAffected()
	return deleted, err
}

// BilledMealMonths names the Mensa months of [from, to] unavailable to the
// run of to. Cancellation permits reissue only in the original invoice's
// billing month, where replacement lookup preserves its invoice chain.
func (s *Store) BilledMealMonths(ctx context.Context, organizationID int64, from, to calendar.Date) ([]parentbilling.CareBilledMealPosition, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		SchoolID       int64         `bun:"school_id"`
		StudentID      int64         `bun:"student_id"`
		MealMonth      calendar.Date `bun:"meal_month,type:date"`
		ReleaseVersion int           `bun:"release_version"`
		Quantity       int           `bun:"quantity"`
		AmountCents    int64         `bun:"amount_cents"`
		InvoiceNumber  string        `bun:"invoice_number"`
		ChildFirstName string        `bun:"child_first_name"`
		ChildLastName  string        `bun:"child_last_name"`
		Cancelled      bool          `bun:"cancelled"`
	}
	// A cancelled position is reserved for the reissue of its own month; in
	// that month's run it is free to bill again.
	if err := db.NewRaw(`SELECT care_meal_positions.school_id, care_meal_positions.student_id, care_meal_positions.meal_month,
			care_meal_positions.release_version, care_meal_positions.quantity, care_meal_positions.amount_cents,
			invoice.number AS invoice_number, invoice.child_first_name, invoice.child_last_name, NOT (`+notCancelled+`) AS cancelled
		FROM parent_billing.care_meal_positions
		JOIN parent_billing.care_invoices AS invoice
			ON invoice.organization_id = care_meal_positions.organization_id AND invoice.id = care_meal_positions.invoice_id
		WHERE care_meal_positions.organization_id = ? AND care_meal_positions.meal_month BETWEEN ? AND ?
			AND (`+notCancelled+` OR invoice.billing_month <> ?)`, organizationID, from, to, to).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read billed Mensa months: %w", err)
	}
	billed := make([]parentbilling.CareBilledMealPosition, len(rows))
	for i, row := range rows {
		billed[i] = parentbilling.CareBilledMealPosition{
			CareBilledMeal: parentbilling.CareBilledMeal{SchoolID: row.SchoolID, StudentID: row.StudentID, Month: row.MealMonth},
			ReleaseVersion: row.ReleaseVersion, Quantity: row.Quantity, AmountCents: row.AmountCents, InvoiceNumber: row.InvoiceNumber,
			ChildFirstName: row.ChildFirstName, ChildLastName: row.ChildLastName, Cancelled: row.Cancelled,
		}
	}
	return billed, nil
}

// MealBilledUntil names per school the last Mensa month an issued invoice
// of the Träger bills.
func (s *Store) MealBilledUntil(ctx context.Context, organizationID int64) (map[int64]calendar.Date, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		SchoolID int64         `bun:"school_id"`
		Until    calendar.Date `bun:"until,type:date"`
	}
	if err := db.NewRaw(`SELECT school_id, MAX(meal_month) AS until FROM parent_billing.care_meal_positions
		WHERE organization_id = ? AND `+notCancelled+` GROUP BY school_id`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read billed Mensa months: %w", err)
	}
	until := make(map[int64]calendar.Date, len(rows))
	for _, row := range rows {
		until[row.SchoolID] = row.Until
	}
	return until, nil
}

// insertCareMealPositions stores the Mensa months of the issued invoices;
// rows[i] is the stored invoices[i]. A Mensa month another invoice bills is
// ErrBillingRunChanged.
func insertCareMealPositions(ctx context.Context, db bun.IDB, organizationID int64, rows []careInvoiceRow, invoices []parentbilling.CareInvoice) error {
	cancelled, err := cancelledMealHeads(ctx, db, organizationID, invoices)
	if err != nil {
		return err
	}
	var positions []careMealPositionRow
	for i, invoice := range invoices {
		for j, meal := range invoice.MealPositions {
			// A child's Mensa month follows its own chain: the invoice of the
			// last cancelled position of that month, whichever invoice now
			// bills it (the Mensa may have moved onto an invoice of its own
			// since, ADR 0167).
			replaces := rows[i].ReplacesInvoiceID
			if head, ok := cancelled[parentbilling.CareBilledMeal{SchoolID: meal.SchoolID, StudentID: meal.StudentID, Month: meal.MealMonth}]; ok {
				replaces = &head
			}
			positions = append(positions, careMealPositionRow{OrganizationID: organizationID, InvoiceID: rows[i].ID,
				Replaces: replaces, Position: j + 1, ReleaseVersion: max(meal.ReleaseVersion, 1),
				SchoolID: meal.SchoolID, StudentID: meal.StudentID, MealMonth: meal.MealMonth, Mode: meal.Mode, Quantity: meal.Quantity,
				UnitCents: meal.UnitCents, AmountCents: meal.AmountCents, NetCents: meal.AmountCents,
				positionAccountsColumns: positionAccountsOf(meal.Accounts)})
		}
	}
	if len(positions) == 0 {
		return nil
	}
	if _, err := db.NewInsert().Model(&positions).Exec(ctx); err != nil {
		if constraintOf(err) == careMealMonthConstraint {
			return parentbilling.ErrBillingRunChanged
		}
		return fmt.Errorf("insert Mensa positions: %w", err)
	}
	return nil
}

// careInvoiceMealRow is an issued invoice with its Mensa months as JSON,
// read in the statement of the invoice.
type careInvoiceMealRow struct {
	careInvoiceRow `bun:",extend"`
	MealPositions  json.RawMessage `bun:"meal_positions,scanonly"`
}

// careInvoiceMealsColumn aggregates the Mensa months of the invoice the row
// reads; the positions name their invoice, whose Träger the row filters on.
const careInvoiceMealsColumn = `COALESCE((SELECT jsonb_agg(jsonb_build_object(
		'school_id', m.school_id, 'student_id', m.student_id, 'meal_month', m.meal_month, 'mode', m.mode,
		'quantity', m.quantity, 'unit_cents', m.unit_cents, 'amount_cents', m.amount_cents,
		'release_version', m.release_version) ORDER BY m.position)
	FROM parent_billing.care_meal_positions AS m
	WHERE m.organization_id = care_invoice_row.organization_id AND m.invoice_id = care_invoice_row.id), '[]'::jsonb) AS meal_positions`

// withCareMealPositions sets the Mensa months of the invoices rows[i] read.
func withCareMealPositions(rows []careInvoiceMealRow, invoices []parentbilling.CareInvoice) error {
	for i := range invoices {
		invoices[i].MealPositions = []parentbilling.CareMealPosition{}
		if len(rows[i].MealPositions) == 0 {
			continue
		}
		if err := json.Unmarshal(rows[i].MealPositions, &invoices[i].MealPositions); err != nil {
			return fmt.Errorf("read Mensa positions: %w", err)
		}
	}
	return nil
}

// MealInvoicedContracts names the contracts an issued Mensa invoice of their
// own bills in month (#4686, ADR 0167); a cancelled one bills nothing.
func (s *Store) MealInvoicedContracts(ctx context.Context, organizationID int64, month calendar.Date) (map[int64]bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var contracts []int64
	if err := db.NewRaw(`SELECT i.care_contract_id FROM parent_billing.care_invoices AS i
		WHERE i.organization_id = ? AND i.billing_month = ? AND i.service = ?
			AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoice_cancellations AS cancellation
				WHERE cancellation.organization_id = i.organization_id AND cancellation.invoice_id = i.id)`,
		organizationID, month, parentbilling.ServiceMeal).Scan(ctx, &contracts); err != nil {
		return nil, fmt.Errorf("read the contracts with a Mensa invoice: %w", err)
	}
	invoiced := make(map[int64]bool, len(contracts))
	for _, contract := range contracts {
		invoiced[contract] = true
	}
	return invoiced, nil
}

// cancelledMealHeads names per child's Mensa month the invoice of its last
// position when that invoice is cancelled: the predecessor a new position of
// the month replaces (#4686).
func cancelledMealHeads(ctx context.Context, db bun.IDB, organizationID int64, invoices []parentbilling.CareInvoice) (map[parentbilling.CareBilledMeal]int64, error) {
	var schools, students []int64
	var months []string
	for _, invoice := range invoices {
		for _, meal := range invoice.MealPositions {
			schools, students, months = append(schools, meal.SchoolID), append(students, meal.StudentID), append(months, meal.MealMonth.String())
		}
	}
	heads := map[parentbilling.CareBilledMeal]int64{}
	if len(schools) == 0 {
		return heads, nil
	}
	var rows []struct {
		SchoolID  int64         `bun:"school_id"`
		StudentID int64         `bun:"student_id"`
		MealMonth calendar.Date `bun:"meal_month,type:date"`
		InvoiceID int64         `bun:"invoice_id"`
		Cancelled bool          `bun:"cancelled"`
	}
	if err := db.NewRaw(`SELECT DISTINCT ON (p.school_id, p.student_id, p.meal_month) p.school_id, p.student_id, p.meal_month,
			p.invoice_id, EXISTS (SELECT 1 FROM parent_billing.care_invoice_cancellations AS c
				WHERE c.organization_id = p.organization_id AND c.invoice_id = p.invoice_id) AS cancelled
		FROM parent_billing.care_meal_positions AS p
		JOIN unnest(?::bigint[], ?::bigint[], ?::date[]) AS wanted(school_id, student_id, meal_month)
			ON wanted.school_id = p.school_id AND wanted.student_id = p.student_id AND wanted.meal_month = p.meal_month
		WHERE p.organization_id = ?
		ORDER BY p.school_id, p.student_id, p.meal_month, p.id DESC`,
		pgdialect.Array(schools), pgdialect.Array(students), pgdialect.Array(months), organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read the cancelled Mensa months: %w", err)
	}
	for _, row := range rows {
		if row.Cancelled {
			heads[parentbilling.CareBilledMeal{SchoolID: row.SchoolID, StudentID: row.StudentID, Month: row.MealMonth}] = row.InvoiceID
		}
	}
	return heads, nil
}
