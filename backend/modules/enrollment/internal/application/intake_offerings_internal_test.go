package application

import (
	"testing"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	"github.com/moto-nrw/project-phoenix/modules/careplan/selection"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The intake validates and materializes offering picks in Enrollment's
// selection engine over the enrollment offering rows. These pin the rows'
// translation into the engine (availability rules, selection modes, group
// rules) and the refusals the intake returns: the engine's value, marked with
// Enrollment's public value at the boundary.

// requireSelectionError asserts the engine's refusal and the Enrollment value
// the handlers map after publicError.
func requireSelectionError(t *testing.T, err, engine, public error) {
	t.Helper()
	require.ErrorIs(t, err, engine)
	require.ErrorIs(t, publicError(err), public)
}

func TestMaterializeCareOfferingsEnforcesAvailabilityPerChild(t *testing.T) {
	t.Parallel()

	grade := func(value int16) *int16 { return &value }
	conditional := &offeringContract.CareOfferingRecord{
		Name: "Randstunde", DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}, AvailabilityRule: testGradeAvailabilityRule(offeringContract.AvailabilityOperatorIn, 1, 2),
	}
	conditional.ID = 10
	unconditional := &offeringContract.CareOfferingRecord{Name: "Alle Klassen", DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	unconditional.ID = 20
	catalog := map[int64]*offeringContract.CareOfferingRecord{10: conditional, 20: unconditional}

	children := []SubmitChild{
		{TargetGradeLevel: grade(2), OfferingIDs: []int64{10, 20}},
		{TargetGradeLevel: grade(3), OfferingIDs: []int64{20}},
	}
	_, err := materializeChildrenOfferingSelections(children, catalog, enrollmentNative.PhaseCareOfferingSelectionOptional)
	require.NoError(t, err)

	children[1].OfferingIDs = []int64{10}
	_, err = materializeChildrenOfferingSelections(children, catalog, enrollmentNative.PhaseCareOfferingSelectionOptional)
	requireSelectionError(t, err, selection.ErrCareOfferingUnavailable, enrollmentNative.ErrCareOfferingUnavailable)
}

func TestChangeRequestCareOfferingsUseEachChildsGradeIndependently(t *testing.T) {
	t.Parallel()

	grade2 := int16(2)
	grade3 := int16(3)
	forGradesOneAndTwo := &offeringContract.CareOfferingRecord{
		DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		AvailabilityRule: testGradeAvailabilityRule(
			offeringContract.AvailabilityOperatorIn, 1, 2,
		),
	}
	forGradesOneAndTwo.ID = 10
	forGradesThreeAndFour := &offeringContract.CareOfferingRecord{
		DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
		AvailableDays:  []string{"tue"},
		AvailabilityRule: testGradeAvailabilityRule(
			offeringContract.AvailabilityOperatorIn, 3, 4,
		),
	}
	forGradesThreeAndFour.ID = 20
	catalog := map[int64]*offeringContract.CareOfferingRecord{
		10: forGradesOneAndTwo,
		20: forGradesThreeAndFour,
	}

	selections, err := materializeChangeRequestChildren(
		[]SubmitChild{
			{TargetGradeLevel: &grade2, OfferingIDs: []int64{10}},
			{TargetGradeLevel: &grade3, OfferingIDs: []int64{20}},
		},
		[]*RequestChild{{}, {}},
		[]map[int64]*offeringContract.CareOfferingRecord{catalog, catalog},
		enrollmentNative.PhaseCareOfferingSelectionOptional,
		false,
	)
	require.NoError(t, err)
	require.Equal(t, int64(10), selections[0][0].OfferingID)
	require.Equal(t, int64(20), selections[1][0].OfferingID)
}

func TestMaterializeCareOfferingsMissingGradeRejectsPositiveAndNegativeRules(t *testing.T) {
	t.Parallel()

	for _, operator := range []string{offeringContract.AvailabilityOperatorIn, offeringContract.AvailabilityOperatorNotIn} {
		t.Run(operator, func(t *testing.T) {
			offering := &offeringContract.CareOfferingRecord{AvailabilityRule: testGradeAvailabilityRule(operator, 1, 2)}
			offering.ID = 10
			_, err := materializeChildrenOfferingSelections(
				[]SubmitChild{{OfferingIDs: []int64{10}}},
				map[int64]*offeringContract.CareOfferingRecord{10: offering},
				enrollmentNative.PhaseCareOfferingSelectionOptional,
			)
			requireSelectionError(t, err, selection.ErrCareOfferingUnavailable, enrollmentNative.ErrCareOfferingUnavailable)
		})
	}
}

func TestConditionalRequiredAndSelectionModeOnlyApplyWhenAvailable(t *testing.T) {
	t.Parallel()

	grade3 := int16(3)
	required := &offeringContract.CareOfferingRecord{
		IsRequired: true, AvailabilityRule: testGradeAvailabilityRule(offeringContract.AvailabilityOperatorIn, 1, 2),
	}
	required.ID = 10
	catalog := map[int64]*offeringContract.CareOfferingRecord{10: required}

	_, err := materializeChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade3}}, catalog, enrollmentNative.PhaseCareOfferingSelectionExactlyOne,
	)
	require.NoError(t, err)

	grade2 := int16(2)
	_, err = materializeChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade2}}, catalog, enrollmentNative.PhaseCareOfferingSelectionExactlyOne,
	)
	requireSelectionError(t, err, selection.ErrRequiredCareOfferingMissing, enrollmentNative.ErrRequiredCareOfferingMissing)
}

func TestSelectionModeStillRejectsAnEmptyCatalog(t *testing.T) {
	t.Parallel()

	_, err := materializeChildrenOfferingSelections(
		[]SubmitChild{{}},
		map[int64]*offeringContract.CareOfferingRecord{},
		enrollmentNative.PhaseCareOfferingSelectionExactlyOne,
	)
	requireSelectionError(t, err, selection.ErrCareOfferingExactlyOneRequired, enrollmentNative.ErrCareOfferingExactlyOneRequired)
}

func TestSelectionGroupsIgnoreUnavailableOfferings(t *testing.T) {
	t.Parallel()

	grade3 := int16(3)
	conditional := &offeringContract.CareOfferingRecord{
		SelectionGroup: "randstunde", SelectionRule: offeringContract.SelectionRuleExactlyOne,
		DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
		AvailabilityRule: testGradeAvailabilityRule(offeringContract.AvailabilityOperatorIn, 1, 2),
	}
	conditional.ID = 10
	available := &offeringContract.CareOfferingRecord{
		SelectionGroup: "randstunde", SelectionRule: offeringContract.SelectionRuleExactlyOne,
		DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	available.ID = 20
	catalog := map[int64]*offeringContract.CareOfferingRecord{10: conditional, 20: available}

	_, err := materializeChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade3, OfferingIDs: []int64{20}}},
		catalog,
		enrollmentNative.PhaseCareOfferingSelectionOptional,
	)
	require.NoError(t, err)

	_, err = materializeChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade3}},
		map[int64]*offeringContract.CareOfferingRecord{10: conditional},
		enrollmentNative.PhaseCareOfferingSelectionOptional,
	)
	require.NoError(t, err)
}

func TestUnavailableOfferingCannotBeAutoAdded(t *testing.T) {
	t.Parallel()

	grade3 := int16(3)
	trigger := &offeringContract.CareOfferingRecord{Name: "Trigger", DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	trigger.ID = 10
	target := &offeringContract.CareOfferingRecord{
		Name:                      "Auto",
		DaysOfWeekMode:            offeringContract.DaysOfWeekModeParentChoice,
		AvailableDays:             []string{"mon"},
		AutoAddTriggerOfferingIDs: []int64{10},
		AvailabilityRule:          testGradeAvailabilityRule(offeringContract.AvailabilityOperatorIn, 1, 2),
	}
	target.ID = 20
	children := []SubmitChild{{TargetGradeLevel: &grade3, OfferingIDs: []int64{10}}}
	selections, err := materializeChildrenOfferingSelections(children, map[int64]*offeringContract.CareOfferingRecord{10: trigger, 20: target}, enrollmentNative.PhaseCareOfferingSelectionOptional)
	require.NoError(t, err)
	require.Len(t, selections[0], 1)
	require.Equal(t, int64(10), selections[0][0].OfferingID)
}

func TestUnknownOfferingStillUsesClosedError(t *testing.T) {
	t.Parallel()

	_, err := materializeChildrenOfferingSelections(
		[]SubmitChild{{OfferingIDs: []int64{999}}}, map[int64]*offeringContract.CareOfferingRecord{}, enrollmentNative.PhaseCareOfferingSelectionOptional,
	)
	requireSelectionError(t, err, selection.ErrCareOfferingClosed, enrollmentNative.ErrCareOfferingClosed)
}

func testGradeAvailabilityRule(operator string, values ...int) *offeringContract.CareOfferingAvailabilityRule {
	return &offeringContract.CareOfferingAvailabilityRule{
		Match: offeringContract.AvailabilityMatchAll,
		Conditions: []offeringContract.CareOfferingAvailabilityCondition{{
			Source: offeringContract.AvailabilitySourceGradeLevel, Operator: operator, Value: values,
		}},
	}
}

func TestMaterializeAndValidateChildrenOfferingSelectionsValidatesFinalAutoAddedGroupRules(t *testing.T) {
	t.Parallel()

	manualOfferingID := int64(1001)
	automaticOfferingID := int64(1002)
	openByID := map[int64]*offeringContract.CareOfferingRecord{
		manualOfferingID: {
			ID:             manualOfferingID,
			Name:           "Frühbetreuung",
			DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon"},
			SelectionGroup: "randzeiten",
			SelectionRule:  offeringContract.SelectionRuleAtMostOne,
			SortOrder:      1,
		},
		automaticOfferingID: {
			ID:                        automaticOfferingID,
			Name:                      "Spätbetreuung",
			DaysOfWeekMode:            offeringContract.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon"},
			AutoAddTriggerOfferingIDs: []int64{manualOfferingID},
			SelectionGroup:            "randzeiten",
			SelectionRule:             offeringContract.SelectionRuleAtMostOne,
			SortOrder:                 2,
		},
	}
	children := []SubmitChild{{
		OfferingIDs:  []int64{manualOfferingID},
		OfferingDays: []SubmitOfferingDays{{OfferingID: manualOfferingID, SelectedDays: []string{"mon"}}},
	}}

	_, err := materializeChildrenOfferingSelections(children, openByID, enrollmentNative.PhaseCareOfferingSelectionOptional)

	requireSelectionError(t, err, selection.ErrCareOfferingRule, enrollmentNative.ErrCareOfferingRule)
}

func TestMaterializeAndValidateChildrenOfferingSelectionsIgnoresAutoAddedOfferingForExactlyOne(t *testing.T) {
	t.Parallel()

	manualOfferingID := int64(1101)
	automaticOfferingID := int64(1102)
	openByID := map[int64]*offeringContract.CareOfferingRecord{
		manualOfferingID: {
			ID:             manualOfferingID,
			Name:           "Ganztag",
			DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "wed"},
			SortOrder:      1,
		},
		automaticOfferingID: {
			ID:                        automaticOfferingID,
			Name:                      "Randstunde",
			DaysOfWeekMode:            offeringContract.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "wed"},
			AutoAddTriggerOfferingIDs: []int64{manualOfferingID},
			SortOrder:                 2,
		},
	}
	children := []SubmitChild{{
		OfferingIDs:  []int64{manualOfferingID},
		OfferingDays: []SubmitOfferingDays{{OfferingID: manualOfferingID, SelectedDays: []string{"mon", "wed"}}},
	}}

	selections, err := materializeChildrenOfferingSelections(children, openByID, enrollmentNative.PhaseCareOfferingSelectionExactlyOne)

	require.NoError(t, err)
	require.Len(t, selections, 1)
	require.Len(t, selections[0], 2)
	assert.Equal(t, []int64{manualOfferingID, automaticOfferingID}, children[0].OfferingIDs)
	assert.Equal(t, []string{"mon", "wed"}, selections[0][1].AutomaticSelectedDays)
}

func TestMaterializeAndValidateChildrenOfferingSelectionsCountsManualAutoTargetForExactlyOne(t *testing.T) {
	t.Parallel()

	manualOfferingID := int64(1201)
	automaticOfferingID := int64(1202)
	openByID := map[int64]*offeringContract.CareOfferingRecord{
		manualOfferingID: {
			ID:             manualOfferingID,
			Name:           "Ganztag",
			DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon"},
			SortOrder:      1,
		},
		automaticOfferingID: {
			ID:                        automaticOfferingID,
			Name:                      "Randstunde",
			DaysOfWeekMode:            offeringContract.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon"},
			AutoAddTriggerOfferingIDs: []int64{manualOfferingID},
			SortOrder:                 2,
		},
	}
	children := []SubmitChild{{
		OfferingIDs: []int64{manualOfferingID, automaticOfferingID},
		OfferingDays: []SubmitOfferingDays{
			{OfferingID: manualOfferingID, SelectedDays: []string{"mon"}},
			{OfferingID: automaticOfferingID, SelectedDays: []string{"mon"}},
		},
	}}

	_, err := materializeChildrenOfferingSelections(children, openByID, enrollmentNative.PhaseCareOfferingSelectionExactlyOne)

	requireSelectionError(t, err, selection.ErrCareOfferingExactlyOneRequired, enrollmentNative.ErrCareOfferingExactlyOneRequired)
}

// TestOrdinarySaveStillEnforcesRequiredCare pins that the complete
// withdrawal Care Plan's offering adjustments allow (#3561) never reaches an
// ordinary enrollment save: required care and the minimum selection hold.
func TestOrdinarySaveStillEnforcesRequiredCare(t *testing.T) {
	t.Parallel()
	care := &offeringContract.CareOfferingRecord{
		ID: 1, Name: "Ganztag", CountsAsCare: true,
		IsRequired: true, DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon", "tue"},
	}
	lunch := &offeringContract.CareOfferingRecord{
		ID: 2, Name: "Mittagessen", CountsAsCare: false,
		DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	catalog := map[int64]*offeringContract.CareOfferingRecord{care.ID: care, lunch.ID: lunch}
	children := []SubmitChild{{OfferingIDs: []int64{lunch.ID}}}

	_, err := materializeChildrenOfferingSelections(
		children, catalog, enrollmentNative.PhaseCareOfferingSelectionAtLeastOne,
	)
	require.Error(t, err, "ordinary saves must still enforce required care and minimum selection")
}
