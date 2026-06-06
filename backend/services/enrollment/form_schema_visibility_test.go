package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// guardianFieldHidden decides whether a required guardian-level field is
// suppressed by its visibility condition, so ValidateSubmission doesn't
// reject a submission for a field the parent never saw.

func fieldWith(c *enrollmentModels.VisibilityCondition) enrollmentModels.FormField {
	return enrollmentModels.FormField{
		Key:         "which_allergy",
		Label:       "Welche Allergie?",
		Type:        enrollmentModels.FormFieldText,
		Required:    true,
		VisibleWhen: c,
	}
}

func TestGuardianFieldHidden_NoCondition(t *testing.T) {
	f := fieldWith(nil)
	assert.False(t, guardianFieldHidden(f, map[string]any{}))
}

func TestGuardianFieldHidden_EqMatchVisible(t *testing.T) {
	f := fieldWith(&enrollmentModels.VisibilityCondition{
		Source:   enrollmentModels.ConditionSourceField,
		Field:    "has_allergy",
		Operator: enrollmentModels.ConditionOpEquals,
		Value:    true,
	})
	assert.False(t, guardianFieldHidden(f, map[string]any{"has_allergy": true}),
		"condition met → field is shown, not hidden")
}

func TestGuardianFieldHidden_EqMismatchHidden(t *testing.T) {
	f := fieldWith(&enrollmentModels.VisibilityCondition{
		Source:   enrollmentModels.ConditionSourceField,
		Field:    "has_allergy",
		Operator: enrollmentModels.ConditionOpEquals,
		Value:    true,
	})
	assert.True(t, guardianFieldHidden(f, map[string]any{"has_allergy": false}),
		"condition not met → field is hidden")
}

func TestGuardianFieldHidden_BooleanRoundTrip(t *testing.T) {
	// A value arriving as the string "true" (JSON edge) still matches a
	// boolean condition value of true.
	f := fieldWith(&enrollmentModels.VisibilityCondition{
		Source:   enrollmentModels.ConditionSourceField,
		Field:    "has_allergy",
		Operator: enrollmentModels.ConditionOpEquals,
		Value:    true,
	})
	assert.False(t, guardianFieldHidden(f, map[string]any{"has_allergy": "true"}))
}

func TestGuardianFieldHidden_NotEqual(t *testing.T) {
	f := fieldWith(&enrollmentModels.VisibilityCondition{
		Source:   enrollmentModels.ConditionSourceField,
		Field:    "mode",
		Operator: enrollmentModels.ConditionOpNotEquals,
		Value:    "x",
	})
	assert.True(t, guardianFieldHidden(f, map[string]any{"mode": "x"}))
	assert.False(t, guardianFieldHidden(f, map[string]any{"mode": "y"}))
}

func TestGuardianFieldHidden_NotEmpty(t *testing.T) {
	f := fieldWith(&enrollmentModels.VisibilityCondition{
		Source:   enrollmentModels.ConditionSourceField,
		Field:    "mode",
		Operator: enrollmentModels.ConditionOpNotEmpty,
	})
	assert.True(t, guardianFieldHidden(f, map[string]any{}), "absent → hidden")
	assert.True(t, guardianFieldHidden(f, map[string]any{"mode": ""}), "empty → hidden")
	assert.False(t, guardianFieldHidden(f, map[string]any{"mode": "y"}), "present → shown")
}

func TestGuardianFieldHidden_NonFieldSourceNeverHidesAtGuardianLevel(t *testing.T) {
	// grade_level / care_offering are per-child sources; they can't be
	// evaluated against guardian answers, so a guardian field carrying
	// one is treated as always visible here (the model rejects such a
	// combination at save time anyway).
	f := fieldWith(&enrollmentModels.VisibilityCondition{
		Source:   enrollmentModels.ConditionSourceGradeLevel,
		Operator: enrollmentModels.ConditionOpEquals,
		Value:    1,
	})
	assert.False(t, guardianFieldHidden(f, map[string]any{}))
}
