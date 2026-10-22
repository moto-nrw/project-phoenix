package application

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan/selection"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
)

func TestCheckApprovalDayRulesScopesOfferingsToTheApprovedPhase(t *testing.T) {
	t.Parallel()

	const (
		phaseID      int64 = 10
		otherPhaseID int64 = 20
	)
	offering := func(id, offeringPhaseID int64) *offeringContract.CareOfferingRecord {
		return &offeringContract.CareOfferingRecord{
			ID: id, PhaseID: offeringPhaseID, DaysOfWeekMode: "parent_choice", AvailableDays: []string{"mon"},
			SelectionGroup: "Betreuungszeit", SelectionRule: selection.SelectionRuleAtMostOnePerDay,
		}
	}
	link := func(id int64) *enrollmentNative.RequestChildOfferingRecord {
		return &enrollmentNative.RequestChildOfferingRecord{CareOfferingID: id, SelectedDays: []string{"mon"}}
	}

	t.Run("ignores a same-named group from another phase", func(t *testing.T) {
		err := checkApprovalDayRules(phaseID,
			[]*enrollmentNative.RequestChildOfferingRecord{link(1), link(2)},
			[]*offeringContract.CareOfferingRecord{offering(1, phaseID), offering(2, otherPhaseID)},
		)
		require.NoError(t, err)
	})

	t.Run("still rejects two offerings from the approved phase on one day", func(t *testing.T) {
		err := checkApprovalDayRules(phaseID,
			[]*enrollmentNative.RequestChildOfferingRecord{link(1), link(3)},
			[]*offeringContract.CareOfferingRecord{offering(1, phaseID), offering(3, phaseID)},
		)
		require.ErrorIs(t, err, selection.ErrCareOfferingDayConflict)
	})
}
