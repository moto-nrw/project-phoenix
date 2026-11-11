package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// guestGroupsQuery reads the group of every hosted week of a published
// period that ends on today or later, with the guests whose access has not
// ended. The host policies admit only the weeks the school hosts; a week
// given to another school or a period taken back drops out by itself.
const guestGroupsQuery = `SELECT host_group.group_id, week.id AS week_id, week.starts_on, week.ends_on,
		(SELECT COUNT(*) FROM enrollment.holiday_guests AS guest
			WHERE guest.tenant_id = host_group.tenant_id AND guest.holiday_period_week_id = week.id
				AND guest.ended_on IS NULL) AS guests
	FROM enrollment.holiday_host_groups AS host_group
	JOIN enrollment.holiday_period_weeks AS week
		ON week.organization_id = host_group.organization_id AND week.id = host_group.holiday_period_week_id
			AND week.host_school_id = host_group.tenant_id
	JOIN enrollment.holiday_periods AS period
		ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
	WHERE host_group.tenant_id = ? AND week.ends_on >= ? AND period.status = 'published'
	ORDER BY week.starts_on, week.id`

type holidayGuestGroupRow struct {
	GroupID  int64         `bun:"group_id"`
	WeekID   int64         `bun:"week_id"`
	StartsOn calendar.Date `bun:"starts_on"`
	EndsOn   calendar.Date `bun:"ends_on"`
	Guests   int           `bun:"guests"`
}

// ListGuestGroups lists the Feriengruppen of the school's hosted weeks that
// end on today or later, in one statement for the whole school.
func (s *HolidayGuestStore) ListGuestGroups(ctx context.Context, schoolID int64, today calendar.Date) ([]enrollment.HolidayGuestGroup, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var rows []holidayGuestGroupRow
	if err := db.NewRaw(guestGroupsQuery, schoolID, today).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list holiday guest groups: %w", err)
	}
	groups := make([]enrollment.HolidayGuestGroup, len(rows))
	for i, row := range rows {
		groups[i] = enrollment.HolidayGuestGroup(row)
	}
	return groups, nil
}
