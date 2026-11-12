package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ReplaceCarePriceLists persists an already validated school batch with a
// constant number of statements, retaining every unrelated school and month.
func (s *Store) ReplaceCarePriceLists(ctx context.Context, organizationID, accountID int64, lists []parentbilling.CarePriceList) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil || len(lists) == 0 {
		return err
	}
	type key struct {
		SchoolID  int64         `json:"school_id"`
		ValidFrom calendar.Date `json:"valid_from"`
	}
	keys := make([]key, 0, len(lists))
	heads := make([]carePriceListRow, 0, len(lists))
	prices := []careTariffRow{}
	for _, list := range lists {
		keys = append(keys, key{list.SchoolID, list.ValidFrom})
		heads = append(heads, carePriceListRow{OrganizationID: organizationID, SchoolID: list.SchoolID, ValidFrom: list.ValidFrom, FreeMonths: list.FreeMonths})
		for _, price := range list.Prices {
			row := careTariffRow{OrganizationID: organizationID, SchoolID: list.SchoolID, CareOfferingID: price.CareOfferingID,
				ModuleName: price.ModuleName, DaysPerWeek: price.DaysPerWeek, ValidFrom: list.ValidFrom, MonthlyCents: price.MonthlyCents,
				SiblingCents: price.SiblingCents, MealExtra: price.MealExtra, CreatedByAccountID: &accountID}
			if price.Weekday != "" {
				day := price.Weekday
				row.Weekday = &day
			}
			prices = append(prices, row)
		}
	}
	encoded, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	if _, err = db.NewRaw(`DELETE FROM parent_billing.care_tariffs AS target
  USING jsonb_to_recordset(?::jsonb) AS batch(school_id bigint,valid_from date)
  WHERE target.organization_id = ? AND target.school_id = batch.school_id AND target.valid_from = batch.valid_from`, string(encoded), organizationID).Exec(ctx); err != nil {
		return fmt.Errorf("replace module tariffs: %w", err)
	}
	if _, err = db.NewRaw(`DELETE FROM parent_billing.care_price_lists AS target
  USING jsonb_to_recordset(?::jsonb) AS batch(school_id bigint,valid_from date)
  WHERE target.organization_id = ? AND target.school_id = batch.school_id AND target.valid_from = batch.valid_from`, string(encoded), organizationID).Exec(ctx); err != nil {
		return fmt.Errorf("replace module price heads: %w", err)
	}
	if _, err = db.NewInsert().Model(&heads).Exec(ctx); err != nil {
		return err
	}
	if len(prices) > 0 {
		_, err = db.NewInsert().Model(&prices).Exec(ctx)
	}
	return err
}
