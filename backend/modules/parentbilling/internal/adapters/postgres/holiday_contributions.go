package postgres

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The frozen holiday contributions (#4664, ADR 0115). The request role may
// insert and read them, never change or delete them; they go with the child
// of their registration.

type holidayContributionRow struct {
	bun.BaseModel `bun:"table:parent_billing.holiday_contributions"`

	ID                         int64     `bun:"id,pk,autoincrement"`
	OrganizationID             int64     `bun:"organization_id"`
	HolidayPeriodID            int64     `bun:"holiday_period_id"`
	HolidayRegistrationID      int64     `bun:"holiday_registration_id"`
	HolidayRegistrationChildID int64     `bun:"holiday_registration_child_id"`
	TotalCents                 int64     `bun:"total_cents"`
	ConfirmedAt                time.Time `bun:"confirmed_at,nullzero,default:current_timestamp"`
	ConfirmedByAccountID       *int64    `bun:"confirmed_by_account_id"`
}

type holidayContributionLineRow struct {
	bun.BaseModel `bun:"table:parent_billing.holiday_contribution_lines"`

	ID                    int64          `bun:"id,pk,autoincrement"`
	OrganizationID        int64          `bun:"organization_id"`
	HolidayContributionID int64          `bun:"holiday_contribution_id"`
	Position              int            `bun:"position"`
	Kind                  string         `bun:"kind"`
	AmountCents           int64          `bun:"amount_cents"`
	WeekStartsOn          *calendar.Date `bun:"week_starts_on,type:date"`
	WeekEndsOn            *calendar.Date `bun:"week_ends_on,type:date"`
	TimeModelName         string         `bun:"time_model_name"`
	ChildRank             *int           `bun:"child_rank"`
	DiscountBasisPoints   *int           `bun:"discount_basis_points"`
	DiscountCents         *int64         `bun:"discount_cents"`
	Note                  string         `bun:"note"`
}

// HolidayContributions reads the frozen contributions of a registration
// with their lines, the children in the order of the form.
func (s *Store) HolidayContributions(ctx context.Context, organizationID, registrationID int64) (parentbilling.HolidayContribution, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.HolidayContribution{}, false, err
	}
	var rows []holidayContributionRow
	if err := db.NewSelect().Model(&rows).
		Where("organization_id = ?", organizationID).Where("holiday_registration_id = ?", registrationID).
		OrderExpr("holiday_registration_child_id").Scan(ctx); err != nil {
		return parentbilling.HolidayContribution{}, false, err
	}
	if len(rows) == 0 {
		return parentbilling.HolidayContribution{}, false, nil
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	var lines []holidayContributionLineRow
	if err := db.NewSelect().Model(&lines).
		Where("organization_id = ?", organizationID).Where("holiday_contribution_id IN (?)", bun.List(ids)).
		OrderExpr("holiday_contribution_id, position").Scan(ctx); err != nil {
		return parentbilling.HolidayContribution{}, false, err
	}
	byContribution := map[int64][]parentbilling.HolidayContributionLine{}
	exemption := false
	for _, line := range lines {
		exemption = exemption || line.Kind == parentbilling.LineBenefitsExemption
		byContribution[line.HolidayContributionID] = append(byContribution[line.HolidayContributionID], parentbilling.HolidayContributionLine{
			Kind: line.Kind, AmountCents: line.AmountCents, WeekStartsOn: line.WeekStartsOn, WeekEndsOn: line.WeekEndsOn,
			TimeModelName: line.TimeModelName, ChildRank: line.ChildRank, DiscountBasisPoints: line.DiscountBasisPoints,
			DiscountCents: line.DiscountCents, Note: line.Note,
		})
	}
	confirmedAt := rows[0].ConfirmedAt
	contribution := parentbilling.HolidayContribution{
		Children: make([]parentbilling.HolidayChildContribution, len(rows)), BenefitsExemption: exemption,
		Frozen: true, ConfirmedAt: &confirmedAt,
	}
	for i, row := range rows {
		childLines := byContribution[row.ID]
		if childLines == nil {
			childLines = []parentbilling.HolidayContributionLine{}
		}
		contribution.Children[i] = parentbilling.HolidayChildContribution{
			ChildID: row.HolidayRegistrationChildID, Position: i + 1, Lines: childLines, TotalCents: row.TotalCents,
		}
		contribution.TotalCents += row.TotalCents
	}
	return contribution, true, nil
}

// InsertHolidayContributions freezes the contribution of every child of a
// registration with its lines.
func (s *Store) InsertHolidayContributions(ctx context.Context, organizationID, periodID, registrationID, accountID int64, contribution parentbilling.HolidayContribution) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(contribution.Children) == 0 {
		return nil
	}
	rows := make([]holidayContributionRow, len(contribution.Children))
	for i, child := range contribution.Children {
		rows[i] = holidayContributionRow{
			OrganizationID: organizationID, HolidayPeriodID: periodID, HolidayRegistrationID: registrationID,
			HolidayRegistrationChildID: child.ChildID, TotalCents: child.TotalCents, ConfirmedByAccountID: &accountID,
		}
	}
	if _, err := db.NewInsert().Model(&rows).Returning("id").Exec(ctx); err != nil {
		return err
	}
	var lines []holidayContributionLineRow
	for i, child := range contribution.Children {
		for position, line := range child.Lines {
			lines = append(lines, holidayContributionLineRow{
				OrganizationID: organizationID, HolidayContributionID: rows[i].ID, Position: position + 1,
				Kind: line.Kind, AmountCents: line.AmountCents, WeekStartsOn: line.WeekStartsOn, WeekEndsOn: line.WeekEndsOn,
				TimeModelName: line.TimeModelName, ChildRank: line.ChildRank, DiscountBasisPoints: line.DiscountBasisPoints,
				DiscountCents: line.DiscountCents, Note: line.Note,
			})
		}
	}
	if len(lines) == 0 {
		return nil
	}
	_, err = db.NewInsert().Model(&lines).Exec(ctx)
	return err
}
