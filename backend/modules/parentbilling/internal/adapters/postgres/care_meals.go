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

// BilledMealMonths names the Mensa months of [from, to] an issued invoice
// of the Träger bills.
func (s *Store) BilledMealMonths(ctx context.Context, organizationID int64, from, to calendar.Date) ([]parentbilling.CareBilledMeal, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		SchoolID  int64         `bun:"school_id"`
		StudentID int64         `bun:"student_id"`
		MealMonth calendar.Date `bun:"meal_month,type:date"`
	}
	if err := db.NewRaw(`SELECT school_id, student_id, meal_month FROM parent_billing.care_meal_positions
		WHERE organization_id = ? AND meal_month BETWEEN ? AND ? AND `+notCancelled, organizationID, from, to).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read billed Mensa months: %w", err)
	}
	billed := make([]parentbilling.CareBilledMeal, len(rows))
	for i, row := range rows {
		billed[i] = parentbilling.CareBilledMeal{SchoolID: row.SchoolID, StudentID: row.StudentID, Month: row.MealMonth}
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
	var positions []careMealPositionRow
	for i, invoice := range invoices {
		for j, meal := range invoice.MealPositions {
			positions = append(positions, careMealPositionRow{OrganizationID: organizationID, InvoiceID: rows[i].ID,
				Replaces: rows[i].ReplacesInvoiceID, Position: j + 1,
				SchoolID: meal.SchoolID, StudentID: meal.StudentID, MealMonth: meal.MealMonth, Mode: meal.Mode, Quantity: meal.Quantity,
				UnitCents: meal.UnitCents, AmountCents: meal.AmountCents, NetCents: meal.AmountCents})
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
		'quantity', m.quantity, 'unit_cents', m.unit_cents, 'amount_cents', m.amount_cents) ORDER BY m.position)
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
