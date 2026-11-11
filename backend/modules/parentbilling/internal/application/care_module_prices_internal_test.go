package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TestModulePriceListKeepsTheOtherModules pins ADR 0195 section 6: the
// prices of a Träger offering replace only its own module in the school's
// list of that month, the other modules stay, and the free months come from
// the list before when the month has none.
func TestModulePriceListKeepsTheOtherModules(t *testing.T) {
	t.Parallel()
	december := calendar.NewDate(2026, 12, 1)
	sibling := int64(1200)
	tariffs := []parentbilling.CareTariff{
		{SchoolID: 7, CareOfferingID: 1, ModuleName: "Früh", Weekday: "mon", ValidFrom: december, MonthlyCents: 650},
		{SchoolID: 7, CareOfferingID: 2, ModuleName: "Mittag", Weekday: "mon", ValidFrom: december, MonthlyCents: 1600},
		{SchoolID: 8, CareOfferingID: 3, ModuleName: "Fremd", Weekday: "mon", ValidFrom: december, MonthlyCents: 999},
	}
	heads := []parentbilling.CarePriceListHead{
		{SchoolID: 7, ValidFrom: calendar.NewDate(2026, 8, 1), FreeMonths: []int{8}},
		{SchoolID: 8, ValidFrom: december, FreeMonths: []int{1}},
	}
	list := modulePriceList(tariffs, heads, parentbilling.CareModulePriceList{
		SchoolID: 7, CareOfferingID: 2, ModuleName: "Mittag neu", ValidFrom: december,
		Prices: []parentbilling.CarePrice{{Weekday: "mon", MonthlyCents: 1700, SiblingCents: &sibling}},
	})
	assert.Equal(t, []int{8}, list.FreeMonths, "the free months of the list before stay")
	assert.Equal(t, []parentbilling.CarePrice{
		{CareOfferingID: 1, ModuleName: "Früh", Weekday: "mon", MonthlyCents: 650},
		{CareOfferingID: 2, ModuleName: "Mittag neu", Weekday: "mon", MonthlyCents: 1700, SiblingCents: &sibling},
	}, list.Prices, "the other module of the school stays; another school's price never joins")
}
