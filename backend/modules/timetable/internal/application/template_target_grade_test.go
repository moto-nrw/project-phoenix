package application

import (
	"testing"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ValidateTemplateTargetGradeLimit(
	gradeLevelMax int,
	existing *activityContract.Group,
	targetGroupType string,
	targetGradeLevel *int16,
) error {
	target := &activityContract.GroupTarget{
		TargetGroupType:  targetGroupType,
		TargetGradeLevel: targetGradeLevel,
	}
	return validateTemplateTargetsGradeLimit(testSchoolClassRules, gradeLevelMax, existing, nil, []*activityContract.GroupTarget{target})
}

func TestValidateTemplateTargetGradeLimit(t *testing.T) {
	t.Parallel()

	gradeFive := int16(5)
	gradeSix := int16(6)
	legacy := &activityContract.Group{
		TargetGroupType:  activityContract.TargetGroupTypeJahrgang,
		TargetGradeLevel: &gradeFive,
	}

	t.Run("rejects a new above-cap Jahrgang", func(t *testing.T) {
		err := ValidateTemplateTargetGradeLimit(
			4, nil, activityContract.TargetGroupTypeJahrgang, &gradeFive,
		)
		require.ErrorIs(t, err, timetable.ErrTemplateTargetGradeExceedsLimit)
	})

	t.Run("allows an unchanged legacy above-cap Jahrgang", func(t *testing.T) {
		require.NoError(t, ValidateTemplateTargetGradeLimit(
			4, legacy, activityContract.TargetGroupTypeJahrgang, &gradeFive,
		))
	})

	t.Run("rejects changing one above-cap Jahrgang to another", func(t *testing.T) {
		err := ValidateTemplateTargetGradeLimit(
			4, legacy, activityContract.TargetGroupTypeJahrgang, &gradeSix,
		)
		require.ErrorIs(t, err, timetable.ErrTemplateTargetGradeExceedsLimit)
	})

	t.Run("does not apply the cap to other target types", func(t *testing.T) {
		require.NoError(t, ValidateTemplateTargetGradeLimit(
			4, legacy, activityContract.TargetGroupTypeNone, nil,
		))
	})

	t.Run("rejects an invalid resolved cap", func(t *testing.T) {
		err := ValidateTemplateTargetGradeLimit(
			0, nil, activityContract.TargetGroupTypeNone, nil,
		)
		assert.ErrorContains(t, err, "grade level max must be between")
	})
}

func TestValidateTemplateTargetsGradeLimitPreservesAllExistingGrades(t *testing.T) {
	t.Parallel()

	gradeFive := int16(5)
	gradeSix := int16(6)
	gradeSeven := int16(7)
	existing := &activityContract.Group{
		TargetGroupType:  activityContract.TargetGroupTypeJahrgang,
		TargetGradeLevel: &gradeFive,
	}
	existingTargets := []*activityContract.GroupTarget{
		{TargetGroupType: activityContract.TargetGroupTypeJahrgang, TargetGradeLevel: &gradeFive},
		{TargetGroupType: activityContract.TargetGroupTypeJahrgang, TargetGradeLevel: &gradeSix},
	}

	t.Run("allows every unchanged above-cap target", func(t *testing.T) {
		require.NoError(t, validateTemplateTargetsGradeLimit(
			testSchoolClassRules,
			4,
			existing,
			existingTargets,
			existingTargets,
		))
	})

	t.Run("rejects an additional above-cap target", func(t *testing.T) {
		requested := append([]*activityContract.GroupTarget{}, existingTargets...)
		requested = append(requested, &activityContract.GroupTarget{
			TargetGroupType:  activityContract.TargetGroupTypeJahrgang,
			TargetGradeLevel: &gradeSeven,
		})
		err := validateTemplateTargetsGradeLimit(testSchoolClassRules, 4, existing, existingTargets, requested)
		require.ErrorIs(t, err, timetable.ErrTemplateTargetGradeExceedsLimit)
	})
}

// TestTemplateGradeLimitNamesTheCap pins that a Jahrgang above the school's
// maximum carries its code and the requested grade and the cap (#2516).
func TestTemplateGradeLimitNamesTheCap(t *testing.T) {
	t.Parallel()

	gradeSix := int16(6)
	err := ValidateTemplateTargetGradeLimit(4, nil, activityContract.TargetGroupTypeJahrgang, &gradeSix)
	require.ErrorIs(t, err, timetable.ErrTemplateTargetGradeExceedsLimit)
	coded, ok := timetable.AsCoded(err)
	require.True(t, ok)
	assert.Equal(t, timetable.CodeTemplateGradeAboveMax, coded.Code)
	assert.Equal(t, timetable.RefusalValues{Grade: 6, Max: 4}, coded.Values)
}
