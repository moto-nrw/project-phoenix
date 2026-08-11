package enrollment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

func TestMaterializeCareOfferingsEnforcesAvailabilityPerChild(t *testing.T) {
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
	_, err := materializeAndValidateChildrenOfferingSelections(children, catalog, enrollmentModels.PhaseCareOfferingSelectionOptional)
	require.NoError(t, err)

	children[1].OfferingIDs = []int64{10}
	_, err = materializeAndValidateChildrenOfferingSelections(children, catalog, enrollmentModels.PhaseCareOfferingSelectionOptional)
	require.ErrorIs(t, err, ErrCareOfferingUnavailable)
}

func TestChangeRequestCareOfferingsUseEachChildsGradeIndependently(t *testing.T) {
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

	selections, err := materializeAndValidateChangeRequestChildrenOfferingSelections(
		[]SubmitChild{
			{TargetGradeLevel: &grade2, OfferingIDs: []int64{10}},
			{TargetGradeLevel: &grade3, OfferingIDs: []int64{20}},
		},
		[]*enrollmentModels.RequestChild{{}, {}},
		[]map[int64]*enrollmentModels.CareOffering{catalog, catalog},
		enrollmentModels.PhaseCareOfferingSelectionOptional,
	)
	require.NoError(t, err)
	require.Equal(t, int64(10), selections[0][0].OfferingID)
	require.Equal(t, int64(20), selections[1][0].OfferingID)
}

func TestMaterializeCareOfferingsMissingGradeRejectsPositiveAndNegativeRules(t *testing.T) {
	for _, operator := range []string{enrollmentModels.AvailabilityOperatorIn, enrollmentModels.AvailabilityOperatorNotIn} {
		t.Run(operator, func(t *testing.T) {
			offering := &enrollmentModels.CareOffering{AvailabilityRule: testGradeAvailabilityRule(operator, 1, 2)}
			offering.ID = 10
			_, err := materializeAndValidateChildrenOfferingSelections(
				[]SubmitChild{{OfferingIDs: []int64{10}}},
				map[int64]*enrollmentModels.CareOffering{10: offering},
				enrollmentModels.PhaseCareOfferingSelectionOptional,
			)
			require.ErrorIs(t, err, ErrCareOfferingUnavailable)
		})
	}
}

func TestConditionalRequiredAndSelectionModeOnlyApplyWhenAvailable(t *testing.T) {
	grade3 := int16(3)
	required := &enrollmentModels.CareOffering{
		IsRequired: true, AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	required.ID = 10
	catalog := map[int64]*enrollmentModels.CareOffering{10: required}

	_, err := materializeAndValidateChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade3}}, catalog, enrollmentModels.PhaseCareOfferingSelectionExactlyOne,
	)
	require.NoError(t, err)

	grade2 := int16(2)
	_, err = materializeAndValidateChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade2}}, catalog, enrollmentModels.PhaseCareOfferingSelectionExactlyOne,
	)
	require.ErrorIs(t, err, ErrRequiredCareOfferingMissing)
}

func TestSelectionModeStillRejectsAnEmptyCatalog(t *testing.T) {
	_, err := materializeAndValidateChildrenOfferingSelections(
		[]SubmitChild{{}},
		map[int64]*enrollmentModels.CareOffering{},
		enrollmentModels.PhaseCareOfferingSelectionExactlyOne,
	)
	require.ErrorIs(t, err, ErrCareOfferingExactlyOneRequired)
}

func TestSelectionGroupsIgnoreUnavailableOfferings(t *testing.T) {
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

	_, err := materializeAndValidateChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade3, OfferingIDs: []int64{20}}},
		catalog,
		enrollmentModels.PhaseCareOfferingSelectionOptional,
	)
	require.NoError(t, err)

	_, err = materializeAndValidateChildrenOfferingSelections(
		[]SubmitChild{{TargetGradeLevel: &grade3}},
		map[int64]*enrollmentModels.CareOffering{10: conditional},
		enrollmentModels.PhaseCareOfferingSelectionOptional,
	)
	require.NoError(t, err)
}

func TestUnavailableOfferingCannotBeAutoAdded(t *testing.T) {
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
	selections, err := materializeAndValidateChildrenOfferingSelections(children, map[int64]*enrollmentModels.CareOffering{10: trigger, 20: target}, enrollmentModels.PhaseCareOfferingSelectionOptional)
	require.NoError(t, err)
	require.Len(t, selections[0], 1)
	require.Equal(t, int64(10), selections[0][0].OfferingID)
}

func TestUnknownOfferingStillUsesClosedError(t *testing.T) {
	_, err := materializeAndValidateChildrenOfferingSelections(
		[]SubmitChild{{OfferingIDs: []int64{999}}}, map[int64]*enrollmentModels.CareOffering{}, enrollmentModels.PhaseCareOfferingSelectionOptional,
	)
	require.True(t, errors.Is(err, ErrCareOfferingClosed))
}

func testGradeAvailabilityRule(operator string, values ...int) *enrollmentModels.CareOfferingAvailabilityRule {
	return &enrollmentModels.CareOfferingAvailabilityRule{
		Match: enrollmentModels.AvailabilityMatchAll,
		Conditions: []enrollmentModels.CareOfferingAvailabilityCondition{{
			Source: enrollmentModels.AvailabilitySourceGradeLevel, Operator: operator, Value: values,
		}},
	}
}

// Bestandsschutz for the admin offering-adjustment path (#2186). A rule
// tightened after a child was booked must not revoke the booking, and must
// not make every later correction for that child unsaveable.
func TestGrandfatheredOfferingsSurviveATightenedAvailabilityRule(t *testing.T) {
	grade := func(value int16) *int16 { return &value }
	restricted := &enrollmentModels.CareOffering{
		Name:             "Randstunde",
		DaysOfWeekMode:   enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:    []string{"mon"},
		AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	restricted.ID = 10
	open := &enrollmentModels.CareOffering{
		Name: "Ganztag", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	open.ID = 20
	catalog := map[int64]*enrollmentModels.CareOffering{10: restricted, 20: open}

	// Without the exemption this is the production bug: a grade-3 child who
	// already holds the grade-1-2 offering cannot be saved at all.
	children := []SubmitChild{{TargetGradeLevel: grade(3), OfferingIDs: []int64{10, 20}}}
	_, err := materializeAndValidateChildrenOfferingSelections(children, catalog, enrollmentModels.PhaseCareOfferingSelectionOptional)
	require.ErrorIs(t, err, ErrCareOfferingUnavailable)

	children = []SubmitChild{{TargetGradeLevel: grade(3), OfferingIDs: []int64{10, 20}}}
	selections, err := materializeAndValidateChildrenOfferingSelectionsGrandfathering(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionOptional,
		map[int64]bool{10: true},
	)
	require.NoError(t, err)

	// The booking must SURVIVE, not merely pass validation: materialization
	// only emits offerings present in the available set, so a bypass that
	// skipped the error alone would still silently drop the booking.
	kept := make([]int64, 0, len(selections[0]))
	for _, selection := range selections[0] {
		kept = append(kept, selection.OfferingID)
	}
	require.ElementsMatch(t, []int64{10, 20}, kept)
}

func TestGrandfatheringDoesNotLetANewBlockedOfferingBeAdded(t *testing.T) {
	grade := func(value int16) *int16 { return &value }
	restricted := &enrollmentModels.CareOffering{
		Name:             "Randstunde",
		DaysOfWeekMode:   enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:    []string{"mon"},
		AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	restricted.ID = 10
	alsoRestricted := &enrollmentModels.CareOffering{
		Name:             "Frühbetreuung",
		DaysOfWeekMode:   enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:    []string{"mon"},
		AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	alsoRestricted.ID = 30
	catalog := map[int64]*enrollmentModels.CareOffering{10: restricted, 30: alsoRestricted}

	// Offering 10 is held (grandfathered); 30 is a NEW blocked pick.
	children := []SubmitChild{{TargetGradeLevel: grade(3), OfferingIDs: []int64{10, 30}}}
	_, err := materializeAndValidateChildrenOfferingSelectionsGrandfathering(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionOptional,
		map[int64]bool{10: true},
	)
	require.ErrorIs(t, err, ErrCareOfferingUnavailable)
}

func TestGrandfatheringIgnoresIDsOutsideTheCatalog(t *testing.T) {
	grade := func(value int16) *int16 { return &value }
	restricted := &enrollmentModels.CareOffering{
		Name:             "Randstunde",
		DaysOfWeekMode:   enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:    []string{"mon"},
		AvailabilityRule: testGradeAvailabilityRule(enrollmentModels.AvailabilityOperatorIn, 1, 2),
	}
	restricted.ID = 10
	catalog := map[int64]*enrollmentModels.CareOffering{10: restricted}

	// A stale id must not be smuggled past the closed-catalog check.
	children := []SubmitChild{{TargetGradeLevel: grade(3), OfferingIDs: []int64{999}}}
	_, err := materializeAndValidateChildrenOfferingSelectionsGrandfathering(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionOptional,
		map[int64]bool{999: true},
	)
	require.ErrorIs(t, err, ErrCareOfferingClosed)
}
