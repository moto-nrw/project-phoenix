package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// carePriceListRow is the head of a price list with its free months (#6196).
type carePriceListRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_price_lists"`

	OrganizationID int64         `bun:"organization_id,pk"`
	SchoolID       int64         `bun:"school_id,pk"`
	ValidFrom      calendar.Date `bun:"valid_from,pk,type:date"`
	FreeMonths     []int         `bun:"free_months,array"`
}

// carePriceBookRow is a head joined with one of its prices; a head without
// prices or a price without a head leaves the other side NULL.
type carePriceBookRow struct {
	HeadSchoolID  *int64         `bun:"head_school_id"`
	HeadValidFrom *calendar.Date `bun:"head_valid_from,type:date"`
	FreeMonths    []int          `bun:"free_months,array"`
	ID            *int64         `bun:"id"`
	SchoolID      *int64         `bun:"school_id"`
	OfferingID    *int64         `bun:"care_offering_id"`
	ModuleName    *string        `bun:"module_name"`
	Weekday       *string        `bun:"weekday"`
	DaysPerWeek   *int           `bun:"days_per_week"`
	ValidFrom     *calendar.Date `bun:"valid_from,type:date"`
	MonthlyCents  *int64         `bun:"monthly_cents"`
	SiblingCents  *int64         `bun:"sibling_cents"`
	MealExtra     *bool          `bun:"meal_extra"`
	CreatedAt     *time.Time     `bun:"created_at"`
}

// CarePriceLists reads every price and every head of the price lists of the
// Träger in one statement (#6196), so the price list and the monthly run
// keep their query budgets. Both tables name the Träger in their own filter.
func (s *Store) CarePriceLists(ctx context.Context, organizationID int64) ([]parentbilling.CareTariff, []parentbilling.CarePriceListHead, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, nil, err
	}
	var rows []carePriceBookRow
	if err := db.NewRaw(`SELECT h.school_id AS head_school_id, h.valid_from AS head_valid_from, h.free_months,
			t.id, t.school_id, t.care_offering_id, t.module_name, t.weekday, t.days_per_week, t.valid_from,
			t.monthly_cents, t.sibling_cents, t.meal_extra, t.created_at
		FROM (SELECT * FROM parent_billing.care_price_lists WHERE organization_id = ?) AS h
		FULL JOIN (SELECT * FROM parent_billing.care_tariffs WHERE organization_id = ?) AS t
			ON t.school_id = h.school_id AND t.valid_from = h.valid_from
		ORDER BY COALESCE(h.school_id, t.school_id), COALESCE(h.valid_from, t.valid_from),
			t.care_offering_id, t.weekday, t.days_per_week`, organizationID, organizationID).Scan(ctx, &rows); err != nil {
		return nil, nil, fmt.Errorf("read care price lists: %w", err)
	}
	tariffs := []parentbilling.CareTariff{}
	heads := []parentbilling.CarePriceListHead{}
	for _, row := range rows {
		if row.HeadSchoolID != nil && row.HeadValidFrom != nil {
			head := parentbilling.CarePriceListHead{SchoolID: *row.HeadSchoolID, ValidFrom: *row.HeadValidFrom, FreeMonths: row.FreeMonths}
			if head.FreeMonths == nil {
				head.FreeMonths = []int{}
			}
			if n := len(heads); n == 0 || heads[n-1].SchoolID != head.SchoolID || heads[n-1].ValidFrom != head.ValidFrom {
				heads = append(heads, head)
			}
		}
		if row.ID != nil {
			tariffs = append(tariffs, row.tariff())
		}
	}
	return tariffs, heads, nil
}

func (r carePriceBookRow) tariff() parentbilling.CareTariff {
	tariff := parentbilling.CareTariff{ID: *r.ID, SchoolID: *r.SchoolID, CareOfferingID: *r.OfferingID, ModuleName: *r.ModuleName,
		DaysPerWeek: r.DaysPerWeek, ValidFrom: *r.ValidFrom, MonthlyCents: *r.MonthlyCents, SiblingCents: r.SiblingCents,
		MealExtra: r.MealExtra != nil && *r.MealExtra}
	if r.Weekday != nil {
		tariff.Weekday = *r.Weekday
	}
	if r.CreatedAt != nil {
		tariff.CreatedAt = *r.CreatedAt
	}
	return tariff
}

// CarePriceListHeads reads the heads of the price lists of the Träger.
func (s *Store) CarePriceListHeads(ctx context.Context, organizationID int64) ([]parentbilling.CarePriceListHead, error) {
	_, heads, err := s.CarePriceLists(ctx, organizationID)
	return heads, err
}
