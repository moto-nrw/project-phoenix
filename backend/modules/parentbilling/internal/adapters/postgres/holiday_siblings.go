package postgres

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The sibling groups of the holiday registrations (#5628). A member goes
// with its registration; a group with fewer than two members goes with its
// last one (trigger drop_lone_holiday_sibling_group).

type siblingGroupRow struct {
	bun.BaseModel `bun:"table:parent_billing.holiday_sibling_groups"`

	ID                 int64  `bun:"id,pk,autoincrement"`
	OrganizationID     int64  `bun:"organization_id"`
	HolidayPeriodID    int64  `bun:"holiday_period_id"`
	CreatedByAccountID *int64 `bun:"created_by_account_id"`
}

type siblingMemberRow struct {
	bun.BaseModel `bun:"table:parent_billing.holiday_sibling_group_members"`

	OrganizationID        int64 `bun:"organization_id"`
	HolidayPeriodID       int64 `bun:"holiday_period_id"`
	SiblingGroupID        int64 `bun:"sibling_group_id"`
	HolidayRegistrationID int64 `bun:"holiday_registration_id"`
}

// HolidaySiblingMembers lists the registrations of every group of a
// period in one statement, by group and registration.
func (s *Store) HolidaySiblingMembers(ctx context.Context, organizationID, periodID int64) ([]parentbilling.HolidaySiblingMember, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []siblingMemberRow
	if err := db.NewSelect().Model(&rows).
		Where("organization_id = ?", organizationID).Where("holiday_period_id = ?", periodID).
		OrderExpr("sibling_group_id, holiday_registration_id").Scan(ctx); err != nil {
		return nil, err
	}
	members := make([]parentbilling.HolidaySiblingMember, len(rows))
	for i, row := range rows {
		members[i] = parentbilling.HolidaySiblingMember{GroupID: row.SiblingGroupID, RegistrationID: row.HolidayRegistrationID}
	}
	return members, nil
}

// ReplaceHolidaySiblingGroups deletes the groups replaced and creates one
// group of the registrations.
func (s *Store) ReplaceHolidaySiblingGroups(ctx context.Context, organizationID, periodID, accountID int64, replaced, registrationIDs []int64) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	if len(replaced) > 0 {
		if _, err := db.NewDelete().TableExpr("parent_billing.holiday_sibling_groups").
			Where("organization_id = ?", organizationID).Where("id IN (?)", bun.List(replaced)).Exec(ctx); err != nil {
			return 0, err
		}
	}
	group := &siblingGroupRow{OrganizationID: organizationID, HolidayPeriodID: periodID, CreatedByAccountID: &accountID}
	if _, err := db.NewInsert().Model(group).Returning("id").Exec(ctx); err != nil {
		return 0, err
	}
	members := make([]siblingMemberRow, len(registrationIDs))
	for i, registrationID := range registrationIDs {
		members[i] = siblingMemberRow{
			OrganizationID: organizationID, HolidayPeriodID: periodID, SiblingGroupID: group.ID, HolidayRegistrationID: registrationID,
		}
	}
	if _, err := db.NewInsert().Model(&members).Exec(ctx); err != nil {
		return 0, err
	}
	return group.ID, nil
}

// RemoveHolidaySibling takes a registration out of its group; the trigger
// drops a group left with one registration.
func (s *Store) RemoveHolidaySibling(ctx context.Context, organizationID, registrationID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().TableExpr("parent_billing.holiday_sibling_group_members").
		Where("organization_id = ?", organizationID).Where("holiday_registration_id = ?", registrationID).Exec(ctx)
	return err
}
