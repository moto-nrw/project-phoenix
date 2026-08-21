package schedule

import (
	"testing"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTemplateTargetGradeLimit(t *testing.T) {
	t.Parallel()

	gradeFive := int16(5)
	gradeSix := int16(6)
	legacy := &activitiesModel.Group{
		TargetGroupType:  activitiesModel.TargetGroupTypeJahrgang,
		TargetGradeLevel: &gradeFive,
	}

	t.Run("rejects a new above-cap Jahrgang", func(t *testing.T) {
		err := ValidateTemplateTargetGradeLimit(
			4, nil, activitiesModel.TargetGroupTypeJahrgang, &gradeFive,
		)
		require.ErrorIs(t, err, ErrTemplateTargetGradeExceedsLimit)
	})

	t.Run("allows an unchanged legacy above-cap Jahrgang", func(t *testing.T) {
		require.NoError(t, ValidateTemplateTargetGradeLimit(
			4, legacy, activitiesModel.TargetGroupTypeJahrgang, &gradeFive,
		))
	})

	t.Run("rejects changing one above-cap Jahrgang to another", func(t *testing.T) {
		err := ValidateTemplateTargetGradeLimit(
			4, legacy, activitiesModel.TargetGroupTypeJahrgang, &gradeSix,
		)
		require.ErrorIs(t, err, ErrTemplateTargetGradeExceedsLimit)
	})

	t.Run("does not apply the cap to other target types", func(t *testing.T) {
		require.NoError(t, ValidateTemplateTargetGradeLimit(
			4, legacy, activitiesModel.TargetGroupTypeNone, nil,
		))
	})

	t.Run("rejects an invalid resolved cap", func(t *testing.T) {
		err := ValidateTemplateTargetGradeLimit(
			0, nil, activitiesModel.TargetGroupTypeNone, nil,
		)
		assert.ErrorContains(t, err, "grade level max must be between")
	})
}

func TestValidateTemplateTargetsGradeLimitPreservesAllExistingGrades(t *testing.T) {
	t.Parallel()

	gradeFive := int16(5)
	gradeSix := int16(6)
	gradeSeven := int16(7)
	existing := &activitiesModel.Group{
		TargetGroupType:  activitiesModel.TargetGroupTypeJahrgang,
		TargetGradeLevel: &gradeFive,
	}
	existingTargets := []*activitiesModel.GroupTarget{
		{TargetGroupType: activitiesModel.TargetGroupTypeJahrgang, TargetGradeLevel: &gradeFive},
		{TargetGroupType: activitiesModel.TargetGroupTypeJahrgang, TargetGradeLevel: &gradeSix},
	}

	t.Run("allows every unchanged above-cap target", func(t *testing.T) {
		require.NoError(t, ValidateTemplateTargetsGradeLimit(
			4,
			existing,
			existingTargets,
			existingTargets,
		))
	})

	t.Run("rejects an additional above-cap target", func(t *testing.T) {
		requested := append([]*activitiesModel.GroupTarget{}, existingTargets...)
		requested = append(requested, &activitiesModel.GroupTarget{
			TargetGroupType:  activitiesModel.TargetGroupTypeJahrgang,
			TargetGradeLevel: &gradeSeven,
		})
		err := ValidateTemplateTargetsGradeLimit(4, existing, existingTargets, requested)
		require.ErrorIs(t, err, ErrTemplateTargetGradeExceedsLimit)
	})
}
