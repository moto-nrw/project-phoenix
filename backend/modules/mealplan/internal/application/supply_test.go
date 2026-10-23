package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
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

func TestDaysSinceUsesDeadlineEvenWhenSettledAfterSupplyStarted(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 9, 7, 11, 0, 0, 0, time.UTC)
	day := domain.OrderDay{Date: "2026-09-08", Order: &domain.Order{DeadlineAt: start.Add(-time.Hour), FrozenAt: start.Add(time.Hour)}}
	own := domain.OrderDay{Date: "2026-09-09", Order: &domain.Order{DeadlineAt: start, FrozenAt: start.Add(time.Hour)}}
	open := domain.OrderDay{Date: "2026-09-10"}
	assert.Equal(t, []domain.OrderDay{own, open}, daysSince([]domain.OrderDay{day, own, open}, start))
}
