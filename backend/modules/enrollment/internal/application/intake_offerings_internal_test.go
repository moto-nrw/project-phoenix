package application

import (
	"testing"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
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
	conditional := &enrollmentModels.CareOffering{
		Name: "Randstunde", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}, AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	conditional.ID = 10
	unconditional := &enrollmentModels.CareOffering{Name: "Alle Klassen", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	unconditional.ID = 20
	catalog := map[int64]*enrollmentModels.CareOffering{10: conditional, 20: unconditional}

	children := []SubmitChild{
		{TargetGradeLevel: grade(2), OfferingIDs: []int64{10, 20}},
		{TargetGradeLevel: grade(3), OfferingIDs: []int64{20}},
	}
	_, err := materializeChildrenOfferingSelections(children, catalog, enrollmentModels.PhaseCareOfferingSelectionOptional)
	require.NoError(t, err)

	children[1].OfferingIDs = []int64{10}
	_, err = materializeChildrenOfferingSelections(children, catalog, enrollmentModels.PhaseCareOfferingSelectionOptional)
	requireSelectionError(t, err, selection.ErrCareOfferingUnavailable, enrollment.ErrCareOfferingUnavailable)
}

func TestChangeRequestCareOfferingsUseEachChildsGradeIndependently(t *testing.T) {
	t.Parallel()

	grade2 := int16(2)
	grade3 := int16(3)
	forGradesOneAndTwo := &enrollmentModels.CareOffering{
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		AvailabilityRule: testGradeAvailabilityRule(
			enrollmentModels.AvailabilityOperatorIn, 1, 2,
		),
	}
	forGradesOneAndTwo.ID = 10
	forGradesThreeAndFour := &enrollmentModels.CareOffering{
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"tue"},
		AvailabilityRule: testGradeAvailabilityRule(
			enrollmentModels.AvailabilityOperatorIn, 3, 4,
		),
	}
	forGradesThreeAndFour.ID = 20
	catalog := map[int64]*enrollmentModels.CareOffering{
		10: forGradesOneAndTwo,
		20: forGradesThreeAndFour,
	}

	selections, err := materializeChangeRequestChildren(
		[]SubmitChild{
			{TargetGradeLevel: &grade2, OfferingIDs: []int64{10}},
			{TargetGradeLevel: &grade3, OfferingIDs: []int64{20}},
		},
		[]*RequestChild{{}, {}},
		[]map[int64]*enrollmentModels.CareOffering{catalog, catalog},
		enrollmentModels.PhaseCareOfferingSelectionOptional,
		false,
	)
	require.NoError(t, err)
	require.Equal(t, int64(10), selections[0][0].OfferingID)
	require.Equal(t, int64(20), selections[1][0].OfferingID)
}

func TestMaterializeCareOfferingsMissingGradeRejectsPositiveAndNegativeRules(t *testing.T) {
	t.Parallel()

	for _, operator := range []string{enrollmentModels.AvailabilityOperatorIn, enrollmentModels.AvailabilityOperatorNotIn} {
		t.Run(operator, func(t *testing.T) {
			offering := &enrollmentModels.CareOffering{AvailabilityRule: testGradeAvailabilityRule(operator, 1, 2)}
			offering.ID = 10
			_, err := materializeChildrenOfferingSelections(
				[]SubmitChild{{OfferingIDs: []int64{10}}},
				map[int64]*enrollmentModels.CareOffering{10: offering},
				enrollmentModels.PhaseCareOfferingSelectionOptional,
			)
			requireSelectionError(t, err, selection.ErrCareOfferingUnavailable, enrollment.ErrCareOfferingUnavailable)
		})
	}
}

func TestConditionalRequiredAndSelectionModeOnlyApplyWhenAvailable(t *testing.T) {
	t.Parallel()

	grade3 := int16(3)
	required := &enrollmentModels.CareOffering{
		IsRequired: true, AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	required.ID = 10
	catalog := map[int64]*enrollmentModels.CareOffering{10: required}

	_, err := materializeChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade3}}, catalog, enrollmentModels.PhaseCareOfferingSelectionExactlyOne,
	)
	require.NoError(t, err)

	grade2 := int16(2)
	_, err = materializeChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade2}}, catalog, enrollmentModels.PhaseCareOfferingSelectionExactlyOne,
	)
	requireSelectionError(t, err, selection.ErrRequiredCareOfferingMissing, enrollment.ErrRequiredCareOfferingMissing)
}

func TestSelectionModeStillRejectsAnEmptyCatalog(t *testing.T) {
	t.Parallel()

	_, err := materializeChildrenOfferingSelections(
		[]SubmitChild{{}},
		map[int64]*enrollmentModels.CareOffering{},
		enrollmentModels.PhaseCareOfferingSelectionExactlyOne,
	)
	requireSelectionError(t, err, selection.ErrCareOfferingExactlyOneRequired, enrollment.ErrCareOfferingExactlyOneRequired)
}

func TestSelectionGroupsIgnoreUnavailableOfferings(t *testing.T) {
	t.Parallel()

	grade3 := int16(3)
	conditional := &enrollmentModels.CareOffering{
		SelectionGroup: "randstunde", SelectionRule: enrollmentModels.SelectionRuleExactlyOne,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
		AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	conditional.ID = 10
	available := &enrollmentModels.CareOffering{
		SelectionGroup: "randstunde", SelectionRule: enrollmentModels.SelectionRuleExactlyOne,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	available.ID = 20
	catalog := map[int64]*enrollmentModels.CareOffering{10: conditional, 20: available}

	_, err := materializeChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade3, OfferingIDs: []int64{20}}},
		catalog,
		enrollmentModels.PhaseCareOfferingSelectionOptional,
	)
	require.NoError(t, err)

	_, err = materializeChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade3}},
		map[int64]*enrollmentModels.CareOffering{10: conditional},
		enrollmentModels.PhaseCareOfferingSelectionOptional,
	)
	require.NoError(t, err)
}

func TestUnavailableOfferingCannotBeAutoAdded(t *testing.T) {
	t.Parallel()

	grade3 := int16(3)
	trigger := &enrollmentModels.CareOffering{Name: "Trigger", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	trigger.ID = 10
	target := &enrollmentModels.CareOffering{
		Name:                      "Auto",
		DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:             []string{"mon"},
		AutoAddTriggerOfferingIDs: []int64{10},
		AvailabilityRule:          testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	target.ID = 20
	children := []SubmitChild{{TargetGradeLevel: &grade3, OfferingIDs: []int64{10}}}
	selections, err := materializeChildrenOfferingSelections(children, map[int64]*enrollmentModels.CareOffering{10: trigger, 20: target}, enrollmentModels.PhaseCareOfferingSelectionOptional)
	require.NoError(t, err)
	require.Len(t, selections[0], 1)
	require.Equal(t, int64(10), selections[0][0].OfferingID)
}

func TestUnknownOfferingStillUsesClosedError(t *testing.T) {
	t.Parallel()

	_, err := materializeChildrenOfferingSelections(
		[]SubmitChild{{OfferingIDs: []int64{999}}}, map[int64]*enrollmentModels.CareOffering{}, enrollmentModels.PhaseCareOfferingSelectionOptional,
	)
	requireSelectionError(t, err, selection.ErrCareOfferingClosed, enrollment.ErrCareOfferingClosed)
}

func testGradeAvailabilityRule(operator string, values ...int) *enrollmentModels.CareOfferingAvailabilityRule {
	return &enrollmentModels.CareOfferingAvailabilityRule{
		Match: enrollmentModels.AvailabilityMatchAll,
		Conditions: []enrollmentModels.CareOfferingAvailabilityCondition{{
			Source: enrollmentModels.AvailabilitySourceGradeLevel, Operator: operator, Value: values,
		}},
	}
}

func TestMaterializeAndValidateChildrenOfferingSelectionsValidatesFinalAutoAddedGroupRules(t *testing.T) {
	t.Parallel()

	manualOfferingID := int64(1001)
	automaticOfferingID := int64(1002)
	openByID := map[int64]*enrollmentModels.CareOffering{
		manualOfferingID: {
			ID:             manualOfferingID,
			Name:           "Frühbetreuung",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon"},
			SelectionGroup: "randzeiten",
			SelectionRule:  enrollmentModels.SelectionRuleAtMostOne,
			SortOrder:      1,
		},
		automaticOfferingID: {
			ID:                        automaticOfferingID,
			Name:                      "Spätbetreuung",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon"},
			AutoAddTriggerOfferingIDs: []int64{manualOfferingID},
			SelectionGroup:            "randzeiten",
			SelectionRule:             enrollmentModels.SelectionRuleAtMostOne,
			SortOrder:                 2,
		},
	}
	children := []SubmitChild{{
		OfferingIDs:  []int64{manualOfferingID},
		OfferingDays: []SubmitOfferingDays{{OfferingID: manualOfferingID, SelectedDays: []string{"mon"}}},
	}}

	_, err := materializeChildrenOfferingSelections(children, openByID, enrollmentModels.PhaseCareOfferingSelectionOptional)

	requireSelectionError(t, err, selection.ErrCareOfferingRule, enrollment.ErrCareOfferingRule)
}

func TestMaterializeAndValidateChildrenOfferingSelectionsIgnoresAutoAddedOfferingForExactlyOne(t *testing.T) {
	t.Parallel()

	manualOfferingID := int64(1101)
	automaticOfferingID := int64(1102)
	openByID := map[int64]*enrollmentModels.CareOffering{
		manualOfferingID: {
			ID:             manualOfferingID,
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "wed"},
			SortOrder:      1,
		},
		automaticOfferingID: {
			ID:                        automaticOfferingID,
			Name:                      "Randstunde",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "wed"},
			AutoAddTriggerOfferingIDs: []int64{manualOfferingID},
			SortOrder:                 2,
		},
	}
	children := []SubmitChild{{
		OfferingIDs:  []int64{manualOfferingID},
		OfferingDays: []SubmitOfferingDays{{OfferingID: manualOfferingID, SelectedDays: []string{"mon", "wed"}}},
	}}

	selections, err := materializeChildrenOfferingSelections(children, openByID, enrollmentModels.PhaseCareOfferingSelectionExactlyOne)

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
	openByID := map[int64]*enrollmentModels.CareOffering{
		manualOfferingID: {
			ID:             manualOfferingID,
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon"},
			SortOrder:      1,
		},
		automaticOfferingID: {
			ID:                        automaticOfferingID,
			Name:                      "Randstunde",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
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

	_, err := materializeChildrenOfferingSelections(children, openByID, enrollmentModels.PhaseCareOfferingSelectionExactlyOne)

	requireSelectionError(t, err, selection.ErrCareOfferingExactlyOneRequired, enrollment.ErrCareOfferingExactlyOneRequired)
}

func TestOrdinarySaveStillEnforcesRequiredCare(t *testing.T) {
	t.Parallel()
	care := &enrollmentModels.CareOffering{
		ID: 1, Name: "Ganztag", CountsAsCare: true,
		IsRequired: true, DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon", "tue"},
	}
	lunch := &enrollmentModels.CareOffering{
		ID: 2, Name: "Mittagessen", CountsAsCare: false,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	catalog := map[int64]*enrollmentModels.CareOffering{care.ID: care, lunch.ID: lunch}
	children := []SubmitChild{{OfferingIDs: []int64{lunch.ID}}}

	_, err := materializeChildrenOfferingSelections(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionAtLeastOne,
	)
	require.Error(t, err, "ordinary saves must still enforce required care and minimum selection")
}
