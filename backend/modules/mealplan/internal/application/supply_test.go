package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestCatererTotalWithNamesHidesClassesInChildrenAndChanges(t *testing.T) {
	t.Parallel()
	child := mealplan.OrderChild{StudentID: 42, FirstName: "Mia", LastName: "Summe", SchoolClass: "1a"}
	week := mealplan.OrderWeek{Days: []mealplan.OrderDay{{Children: []mealplan.OrderChild{child}, Changes: []mealplan.OrderChange{{Child: child}}}}}
	got := catererWeek(week, domain.OrderingRules{Detail: domain.DetailTotal, CatererNames: true})
	want := mealplan.OrderChild{FirstName: "Mia", LastName: "Summe"}
	assert.Equal(t, []mealplan.OrderChild{want}, got.Days[0].Children)
	assert.Equal(t, want, got.Days[0].Changes[0].Child)
}

// ADR 0077: the caterer reads the delivery days from the day its supply
// started on, also those whose order was fixed before, and a confirmation
// from before the start is its predecessor's.
func TestDaysSinceKeepsTheDeliveryDaysOfTheSupply(t *testing.T) {
	t.Parallel()
	berlin := calendar.Berlin
	start := time.Date(2026, 9, 8, 10, 0, 0, 0, berlin)
	predecessor := time.Date(2026, 9, 7, 12, 0, 0, 0, berlin)
	own := time.Date(2026, 9, 8, 11, 0, 0, 0, berlin)
	before := domain.OrderDay{Date: "2026-09-07", Order: &domain.Order{DeadlineAt: time.Date(2026, 9, 6, 9, 0, 0, 0, berlin)}}
	startDay := domain.OrderDay{Date: "2026-09-08", Order: &domain.Order{DeadlineAt: time.Date(2026, 9, 7, 9, 0, 0, 0, berlin), ConfirmedAt: &predecessor}}
	fixedBefore := domain.OrderDay{Date: "2026-09-09", Order: &domain.Order{DeadlineAt: time.Date(2026, 9, 8, 9, 0, 0, 0, berlin), ConfirmedAt: &own}}
	open := domain.OrderDay{Date: "2026-09-10"}

	kept := daysSince([]domain.OrderDay{before, startDay, fixedBefore, open}, start)
	dates := make([]domain.Date, 0, len(kept))
	for _, day := range kept {
		dates = append(dates, day.Date)
	}
	assert.Equal(t, []domain.Date{"2026-09-08", "2026-09-09", "2026-09-10"}, dates, "the day before the supply stays hidden")
	assert.Nil(t, kept[0].Order.ConfirmedAt, "the predecessor's confirmation does not count")
	assert.Equal(t, &predecessor, startDay.Order.ConfirmedAt, "the school's view keeps it")
	assert.Equal(t, &own, kept[1].Order.ConfirmedAt)
}
