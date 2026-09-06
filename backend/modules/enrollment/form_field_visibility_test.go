package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These cover the two features added on top of the original form schema:
// read-only information blocks (FormFieldInfo) and per-field visibility
// conditions (VisibleWhen). Both are validated at save time so a broken
// definition can never reach a parent submission.

// ---- Information fields --------------------------------------------------

func TestFormField_Validate_InfoHappyPath(t *testing.T) {
	t.Parallel()

	f := FormField{
		Key:     "infotext_1",
		Type:    FormFieldInfo,
		Label:   "", // heading is optional for info blocks
		Content: "Bitte den Allergiepass mitbringen.",
	}
	assert.NoError(t, f.Validate())
}

func TestFormField_Validate_InfoRequiresContent(t *testing.T) {
	t.Parallel()

	f := FormField{Key: "infotext_1", Type: FormFieldInfo, Content: "   "}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "content")
}

func TestFormField_Validate_InfoCannotBeRequired(t *testing.T) {
	t.Parallel()

	f := FormField{Key: "infotext_1", Type: FormFieldInfo, Content: "Hinweis", Required: true}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestFormField_Validate_InfoRejectsOptions(t *testing.T) {
	t.Parallel()

	f := FormField{
		Key:     "infotext_1",
		Type:    FormFieldInfo,
		Content: "Hinweis",
		Options: []FormFieldOption{{Label: "A", Value: "a"}},
	}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "options")
}

func TestFormField_Validate_InfoRejectsTarget(t *testing.T) {
	t.Parallel()

	f := FormField{
		Key:     "infotext_1",
		Type:    FormFieldInfo,
		Content: "Hinweis",
		Target:  TargetStudentHealthInfo,
	}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target")
}

func TestFormField_Validate_QuestionRejectsContent(t *testing.T) {
	t.Parallel()

	// Only information fields may carry content; a normal question with
	// content set is a programming error.
	f := FormField{Key: "allergies", Label: "Allergien", Type: FormFieldText, Content: "oops"}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "content")
}

// ---- VisibilityCondition shape ------------------------------------------

func TestVisibilityCondition_Validate_FieldEq(t *testing.T) {
	t.Parallel()

	c := &VisibilityCondition{Source: ConditionSourceField, Field: "has_allergy", Operator: ConditionOpEquals, Value: true}
	assert.NoError(t, c.Validate(false))
}

func TestVisibilityCondition_Validate_NotEmptyNeedsNoValue(t *testing.T) {
	t.Parallel()

	c := &VisibilityCondition{Source: ConditionSourceField, Field: "has_allergy", Operator: ConditionOpNotEmpty}
	assert.NoError(t, c.Validate(false))
}

func TestVisibilityCondition_Validate_EqRequiresValue(t *testing.T) {
	t.Parallel()

	c := &VisibilityCondition{Source: ConditionSourceField, Field: "x", Operator: ConditionOpEquals}
	err := c.Validate(false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "value")
}

func TestVisibilityCondition_Validate_UnknownOperator(t *testing.T) {
	t.Parallel()

	c := &VisibilityCondition{Source: ConditionSourceField, Field: "x", Operator: "between", Value: 1}
	require.Error(t, c.Validate(false))
}

func TestVisibilityCondition_Validate_FieldSourceRequiresKey(t *testing.T) {
	t.Parallel()

	c := &VisibilityCondition{Source: ConditionSourceField, Operator: ConditionOpEquals, Value: true}
	err := c.Validate(false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field")
}

func TestVisibilityCondition_Validate_FieldSourceRejectsIncludes(t *testing.T) {
	t.Parallel()

	c := &VisibilityCondition{Source: ConditionSourceField, Field: "x", Operator: ConditionOpIncludes, Value: "a"}
	require.Error(t, c.Validate(false))
}

func TestVisibilityCondition_Validate_GradeLevelChildOnly(t *testing.T) {
	t.Parallel()

	c := &VisibilityCondition{Source: ConditionSourceGradeLevel, Operator: ConditionOpEquals, Value: 1}
	assert.NoError(t, c.Validate(true), "valid on a per-child field")
	require.Error(t, c.Validate(false), "rejected on a guardian-level field")
}

func TestVisibilityCondition_Validate_CareOfferingRequiresIncludes(t *testing.T) {
	t.Parallel()

	ok := &VisibilityCondition{Source: ConditionSourceCareOffering, Operator: ConditionOpIncludes, Value: "Mittagessen"}
	assert.NoError(t, ok.Validate(true))

	wrongOp := &VisibilityCondition{Source: ConditionSourceCareOffering, Operator: ConditionOpEquals, Value: "Mittagessen"}
	require.Error(t, wrongOp.Validate(true))

	guardian := &VisibilityCondition{Source: ConditionSourceCareOffering, Operator: ConditionOpIncludes, Value: "Mittagessen"}
	require.Error(t, guardian.Validate(false))
}

func TestVisibilityCondition_Validate_UnknownSource(t *testing.T) {
	t.Parallel()

	c := &VisibilityCondition{Source: "weather", Operator: ConditionOpEquals, Value: "sunny"}
	require.Error(t, c.Validate(true))
}

// ---- Cross-field references (FormSchema.Validate) ------------------------

func schemaWithFields(fields ...FormField) *FormSchema {
	return &FormSchema{Name: "Schuljahr", Version: 1, CreatedBy: 4321, Fields: fields}
}

func TestFormSchema_Validate_ConditionReferencesBooleanField(t *testing.T) {
	t.Parallel()

	s := schemaWithFields(
		FormField{Key: "has_allergy", Label: "Allergie?", Type: FormFieldBoolean, SortOrder: 0},
		FormField{Key: "which", Label: "Welche?", Type: FormFieldText, SortOrder: 1,
			VisibleWhen: &VisibilityCondition{Source: ConditionSourceField, Field: "has_allergy", Operator: ConditionOpEquals, Value: true}},
	)
	assert.NoError(t, s.Validate())
}

func TestFormSchema_Validate_ConditionUnknownReference(t *testing.T) {
	t.Parallel()

	s := schemaWithFields(
		FormField{Key: "which", Label: "Welche?", Type: FormFieldText, SortOrder: 0,
			VisibleWhen: &VisibilityCondition{Source: ConditionSourceField, Field: "ghost", Operator: ConditionOpEquals, Value: true}},
	)
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestFormSchema_Validate_ConditionSelfReference(t *testing.T) {
	t.Parallel()

	s := schemaWithFields(
		FormField{Key: "loop", Label: "Loop", Type: FormFieldSelect, SortOrder: 0,
			Options:     []FormFieldOption{{Label: "A", Value: "a"}},
			VisibleWhen: &VisibilityCondition{Source: ConditionSourceField, Field: "loop", Operator: ConditionOpEquals, Value: "a"}},
	)
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "itself")
}

func TestFormSchema_Validate_ConditionControllerMustBeBooleanOrSelect(t *testing.T) {
	t.Parallel()

	s := schemaWithFields(
		FormField{Key: "free_text", Label: "Freitext", Type: FormFieldText, SortOrder: 0},
		FormField{Key: "dependent", Label: "Abhängig", Type: FormFieldText, SortOrder: 1,
			VisibleWhen: &VisibilityCondition{Source: ConditionSourceField, Field: "free_text", Operator: ConditionOpEquals, Value: "x"}},
	)
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yes/no or selection")
}

func TestFormSchema_Validate_GuardianFieldCannotDependOnChildField(t *testing.T) {
	t.Parallel()

	s := schemaWithFields(
		FormField{Key: "child_flag", Label: "Kind-Flag", Type: FormFieldBoolean, SortOrder: 0, AppliesToCh: true},
		FormField{Key: "guardian_note", Label: "Eltern-Notiz", Type: FormFieldText, SortOrder: 1, AppliesToCh: false,
			VisibleWhen: &VisibilityCondition{Source: ConditionSourceField, Field: "child_flag", Operator: ConditionOpEquals, Value: true}},
	)
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "per-child")
}

func TestFormSchema_Validate_ChildFieldMayDependOnGuardianField(t *testing.T) {
	t.Parallel()

	s := schemaWithFields(
		FormField{Key: "guardian_flag", Label: "Eltern-Flag", Type: FormFieldBoolean, SortOrder: 0, AppliesToCh: false},
		FormField{Key: "child_note", Label: "Kind-Notiz", Type: FormFieldText, SortOrder: 1, AppliesToCh: true,
			VisibleWhen: &VisibilityCondition{Source: ConditionSourceField, Field: "guardian_flag", Operator: ConditionOpEquals, Value: true}},
	)
	assert.NoError(t, s.Validate())
}

func TestFormSchema_Validate_InfoFieldWithCondition(t *testing.T) {
	t.Parallel()

	s := schemaWithFields(
		FormField{Key: "has_allergy", Label: "Allergie?", Type: FormFieldBoolean, SortOrder: 0},
		FormField{Key: "infotext_1", Type: FormFieldInfo, Content: "Allergiepass mitbringen.", SortOrder: 1,
			VisibleWhen: &VisibilityCondition{Source: ConditionSourceField, Field: "has_allergy", Operator: ConditionOpEquals, Value: true}},
	)
	assert.NoError(t, s.Validate())
}
