package owners

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestContractChangeImpactPreservesTheSchoolsFollowUp(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 11, 9)
	until := day.AddDays(14)
	result := careContractChangeImpact(careplan.RosterImpact{
		EffectiveFrom:   day,
		Adjusted:        []careplan.AdjustedRosterTemplate{{TemplateID: 12, Name: "Angebotsgruppe", RemovedWeekdays: []int{2}, AddedWeekdays: []int{3}}},
		NotAdjusted:     []careplan.ManualRosterTemplate{{TemplateID: 13, Name: "Handgruppe", Weekdays: []int{2}, ViaTarget: true, Until: &until}},
		PickupOverrides: []careplan.PickupOverride{{Weekday: 3, Date: day.AddDays(2), StoredTime: "15:00", BookedTime: "16:00", ResetNow: false}},
	})
	assert.Equal(t, &carecontract.ChangeImpact{
		EffectiveFrom:   day,
		Adjusted:        []carecontract.ChangeAdjustedTemplate{{TemplateID: "12", Name: "Angebotsgruppe", RemovedWeekdays: []int{2}, AddedWeekdays: []int{3}}},
		NotAdjusted:     []carecontract.ChangeManualTemplate{{TemplateID: "13", Name: "Handgruppe", Weekdays: []int{2}, ViaTarget: true, Until: &until}},
		PickupOverrides: []carecontract.ChangePickupOverride{{Weekday: 3, Date: day.AddDays(2), StoredTime: "15:00", BookedTime: "16:00", ResetNow: false}},
	}, result)
}
