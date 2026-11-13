package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// TestContractEndIsTheEndOfTheNewestEffectiveVersion: a module switch ends
// the first version but not the contract, and a change put in front of a
// planned version has the highest number without being the newest (#4681).
func TestContractEndIsTheEndOfTheNewestEffectiveVersion(t *testing.T) {
	t.Parallel()
	switchDay, termEnd := calendar.NewDate(2027, 2, 28), calendar.NewDate(2027, 7, 31)
	first := carecontract.Version{Number: 1, Status: carecontract.VersionSuperseded,
		EffectiveFrom: calendar.NewDate(2026, 8, 1), EndsOn: &switchDay}
	planned := carecontract.Version{Number: 2, Status: carecontract.VersionEffective, EffectiveFrom: calendar.NewDate(2027, 3, 1)}
	inFront := carecontract.Version{Number: 3, Status: carecontract.VersionEffective,
		EffectiveFrom: calendar.NewDate(2027, 2, 1), EndsOn: &termEnd}
	requested := carecontract.Version{Number: 4, Status: carecontract.VersionRequested, EffectiveFrom: calendar.NewDate(2027, 4, 1), EndsOn: &termEnd}

	assert.Nil(t, contractEnd([]carecontract.Version{first, planned, inFront, requested}, inFront),
		"the planned open version from March decides, not version 3 or a requested one")
	assert.Equal(t, &termEnd, contractEnd(nil, carecontract.Version{EndsOn: &termEnd}),
		"without an effective version the current one decides")
}
