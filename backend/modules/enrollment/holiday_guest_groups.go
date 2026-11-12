package enrollment

import (
	"context"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// HolidayGuestGroup is a group of the hosting school that is the
// Feriengruppe of a hosted week (ADR 0120): the week, its first and last day
// and how many guests of the week the group holds. Before the first day the
// guests wait in no group (Nachtrag 2026-11-03), so a group made for the week
// stands empty until then; the school's lists name the week and the guests
// instead of showing an unexplained empty group.
type HolidayGuestGroup struct {
	GroupID  int64         `json:"group_id"`
	WeekID   int64         `json:"week_id"`
	StartsOn calendar.Date `json:"starts_on"`
	EndsOn   calendar.Date `json:"ends_on"`
	// Guests counts the guests of the week whose access has not ended.
	Guests int `json:"guests"`
}

// HolidayGuestGroupList lists the Feriengruppen of a hosting school, in the
// tenant transaction of the school.
type HolidayGuestGroupList interface {
	// HolidayGuestGroups lists, by first day, every group the school picked
	// for a week of a published period it hosts whose last day is today or
	// later, in one statement for the whole school.
	HolidayGuestGroups(ctx context.Context, schoolID int64) ([]HolidayGuestGroup, error)
}
