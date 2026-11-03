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
// of their registration. A merge or split of siblings adds a new version of
// the contribution of a confirmed registration (#5628); the highest version
// of a registration is valid, the older ones stay.

// currentVersion keeps the rows of the newest version of their
// registration.
const currentVersion = `?TableAlias.version = (SELECT max(newer.version) FROM parent_billing.holiday_contributions AS newer
	WHERE newer.organization_id = ?TableAlias.organization_id AND newer.holiday_registration_id = ?TableAlias.holiday_registration_id)`

type holidayContributionRow struct {
	bun.BaseModel `bun:"table:parent_billing.holiday_contributions"`

	ID                         int64     `bun:"id,pk,autoincrement"`
	OrganizationID             int64     `bun:"organization_id"`
	HolidayPeriodID            int64     `bun:"holiday_period_id"`
	HolidayRegistrationID      int64     `bun:"holiday_registration_id"`
	HolidayRegistrationChildID int64     `bun:"holiday_registration_child_id"`
	TotalCents                 int64     `bun:"total_cents"`
	Version                    int       `bun:"version"`
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
	CostBearerID          *int64         `bun:"cost_bearer_id"`
	CostBearerName        string         `bun:"cost_bearer_name"`
}

func (l holidayContributionLineRow) line() parentbilling.HolidayContributionLine {
	return parentbilling.HolidayContributionLine{
		Kind: l.Kind, AmountCents: l.AmountCents, WeekStartsOn: l.WeekStartsOn, WeekEndsOn: l.WeekEndsOn,
		TimeModelName: l.TimeModelName, ChildRank: l.ChildRank, DiscountBasisPoints: l.DiscountBasisPoints,
		DiscountCents: l.DiscountCents, Note: l.Note, CostBearerID: l.CostBearerID, CostBearerName: l.CostBearerName,
	}
}

// HolidayContributions reads the newest frozen contributions of a
// registration with their lines, the children in the order of the form.
func (s *Store) HolidayContributions(ctx context.Context, organizationID, registrationID int64) (parentbilling.HolidayContribution, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.HolidayContribution{}, false, err
	}
	var rows []holidayContributionRow
	if err := db.NewSelect().Model(&rows).
		Where("organization_id = ?", organizationID).Where("holiday_registration_id = ?", registrationID).
		Where(currentVersion).OrderExpr("holiday_registration_child_id").Scan(ctx); err != nil {
		return parentbilling.HolidayContribution{}, false, err
	}
	if len(rows) == 0 {
		return parentbilling.HolidayContribution{}, false, nil
	}
	byContribution, err := contributionLines(ctx, db, organizationID, rows)
	if err != nil {
		return parentbilling.HolidayContribution{}, false, err
	}
	confirmedAt := rows[0].ConfirmedAt
	contribution := parentbilling.HolidayContribution{
		Children: make([]parentbilling.HolidayChildContribution, len(rows)), Frozen: true, ConfirmedAt: &confirmedAt,
	}
	for _, lines := range byContribution {
		for _, line := range lines {
			switch line.Kind {
			case parentbilling.LineBenefitsExemption:
				contribution.BenefitsExemption = true
			case parentbilling.LineCostTakeover:
				contribution.CostBearerID, contribution.CostBearerName = line.CostBearerID, line.CostBearerName
			}
		}
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
// registration with its lines, as the next version of the registration's
// contribution: the first at the confirmation, a further one after a merge
// or split of siblings (#5628).
func (s *Store) InsertHolidayContributions(ctx context.Context, organizationID, periodID, registrationID, accountID int64, contribution parentbilling.HolidayContribution) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if len(contribution.Children) == 0 {
		return nil
	}
	// A later version keeps the time of the confirmation; its account is
	// the one that merged or split the siblings.
	var version int
	var confirmedAt *time.Time
	if err := db.NewRaw(`SELECT COALESCE(max(version), 0) + 1, min(confirmed_at) FROM parent_billing.holiday_contributions
		WHERE organization_id = ? AND holiday_registration_id = ?`, organizationID, registrationID).Scan(ctx, &version, &confirmedAt); err != nil {
		return err
	}
	rows := make([]holidayContributionRow, len(contribution.Children))
	for i, child := range contribution.Children {
		rows[i] = holidayContributionRow{
			OrganizationID: organizationID, HolidayPeriodID: periodID, HolidayRegistrationID: registrationID,
			HolidayRegistrationChildID: child.ChildID, TotalCents: child.TotalCents, Version: version, ConfirmedByAccountID: &accountID,
		}
		if confirmedAt != nil {
			rows[i].ConfirmedAt = *confirmedAt
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
				DiscountCents: line.DiscountCents, Note: line.Note, CostBearerID: line.CostBearerID, CostBearerName: line.CostBearerName,
			})
		}
	}
	if len(lines) == 0 {
		return nil
	}
	_, err = db.NewInsert().Model(&lines).Exec(ctx)
	return err
}
