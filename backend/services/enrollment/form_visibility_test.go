package enrollment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// Server-side visibility evaluation + required-custom-field enforcement.
// These mirror the client-side checks in enrollment-form.tsx and the
// shared evaluator in enrollment-field-visibility.ts so a stale or
// scripted submit can't bypass a visible required field.

func gradePtr(v int16) *int16 { return &v }

// ---- answerEmpty ---------------------------------------------------------

func TestAnswerEmpty(t *testing.T) {
	text := &enrollmentModels.FormField{Type: enrollmentModels.FormFieldText}
	boolean := &enrollmentModels.FormField{Type: enrollmentModels.FormFieldBoolean}
	phones := &enrollmentModels.FormField{Type: enrollmentModels.FormFieldPhoneList}
	contacts := &enrollmentModels.FormField{Type: enrollmentModels.FormFieldContactList}
	sched := &enrollmentModels.FormField{Type: enrollmentModels.FormFieldWeekdaySchedule}

	assert.True(t, answerEmpty(text, nil), "nil is empty")
	assert.True(t, answerEmpty(text, ""), "empty string is empty")
	assert.True(t, answerEmpty(text, "   "), "blank string is empty")
	assert.False(t, answerEmpty(text, "x"), "non-blank string is an answer")
	assert.False(t, answerEmpty(text, float64(0)), "number 0 is an answer")

	assert.False(t, answerEmpty(boolean, true), "bool true is an answer")
	assert.False(t, answerEmpty(boolean, false), "bool false is still an answer")
	assert.True(t, answerEmpty(boolean, nil), "unanswered bool is empty")

	// Structured types: empty when there is no entry.
	assert.True(t, answerEmpty(phones, nil), "nil phone list is empty")
	assert.True(t, answerEmpty(phones, []any{}), "empty phone list is empty")
	assert.False(t, answerEmpty(phones, []any{map[string]any{"phone_number": "012"}}), "one phone is an answer")
	assert.True(t, answerEmpty(contacts, []any{}), "empty contact list is empty")
	assert.False(t, answerEmpty(contacts, []any{map[string]any{"first_name": "A"}}), "one contact is an answer")
	assert.True(t, answerEmpty(sched, map[string]any{"mon": "", "tue": ""}), "all-blank schedule is empty")
	assert.False(t, answerEmpty(sched, map[string]any{"mon": "08:00"}), "a filled day is an answer")
	assert.True(t, answerEmpty(sched, nil), "nil schedule is empty")
}

// ---- fieldVisible --------------------------------------------------------

func TestFieldVisible_NoCondition(t *testing.T) {
	f := &enrollmentModels.FormField{Key: "x", Type: enrollmentModels.FormFieldText}
	assert.True(t, fieldVisible(f, fieldVisibilityContext{}))
}

func TestFieldVisible_GuardianFieldSource(t *testing.T) {
	controller := &enrollmentModels.FormField{Key: "has_allergy", Type: enrollmentModels.FormFieldBoolean}
	dependent := &enrollmentModels.FormField{
		Key:  "which",
		Type: enrollmentModels.FormFieldText,
		VisibleWhen: &enrollmentModels.VisibilityCondition{
			Source: enrollmentModels.ConditionSourceField, Field: "has_allergy",
			Operator: enrollmentModels.ConditionOpEquals, Value: true,
		},
	}
	byKey := map[string]*enrollmentModels.FormField{"has_allergy": controller, "which": dependent}

	assert.True(t, fieldVisible(dependent, fieldVisibilityContext{
		guardianAnswers: map[string]any{"has_allergy": true}, fieldsByKey: byKey,
	}))
	assert.False(t, fieldVisible(dependent, fieldVisibilityContext{
		guardianAnswers: map[string]any{"has_allergy": false}, fieldsByKey: byKey,
	}))
}

func TestFieldVisible_ChildScopeControllerReadFromChildAnswers(t *testing.T) {
	controller := &enrollmentModels.FormField{Key: "child_flag", Type: enrollmentModels.FormFieldBoolean, AppliesToCh: true}
	dependent := &enrollmentModels.FormField{
		Key: "child_detail", Type: enrollmentModels.FormFieldText, AppliesToCh: true,
		VisibleWhen: &enrollmentModels.VisibilityCondition{
			Source: enrollmentModels.ConditionSourceField, Field: "child_flag",
			Operator: enrollmentModels.ConditionOpEquals, Value: true,
		},
	}
	byKey := map[string]*enrollmentModels.FormField{"child_flag": controller, "child_detail": dependent}

	// The controller is per-child, so its answer must be read from
	// childAnswers, not guardianAnswers.
	assert.True(t, fieldVisible(dependent, fieldVisibilityContext{
		guardianAnswers: map[string]any{},
		childAnswers:    map[string]any{"child_flag": true},
		fieldsByKey:     byKey,
	}))
	assert.False(t, fieldVisible(dependent, fieldVisibilityContext{
		guardianAnswers: map[string]any{"child_flag": true}, // wrong scope
		childAnswers:    map[string]any{},
		fieldsByKey:     byKey,
	}))
}

func TestFieldVisible_GradeLevel(t *testing.T) {
	f := &enrollmentModels.FormField{
		Key: "g", Type: enrollmentModels.FormFieldText, AppliesToCh: true,
		VisibleWhen: &enrollmentModels.VisibilityCondition{
			Source:   enrollmentModels.ConditionSourceGradeLevel,
			Operator: enrollmentModels.ConditionOpEquals, Value: float64(1),
		},
	}
	assert.True(t, fieldVisible(f, fieldVisibilityContext{gradeLevel: gradePtr(1)}))
	assert.False(t, fieldVisible(f, fieldVisibilityContext{gradeLevel: gradePtr(2)}))
	assert.False(t, fieldVisible(f, fieldVisibilityContext{}), "no grade → not equal")
}

func TestFieldVisible_CareOfferingByName(t *testing.T) {
	f := &enrollmentModels.FormField{
		Key: "lunch_note", Type: enrollmentModels.FormFieldText, AppliesToCh: true,
		VisibleWhen: &enrollmentModels.VisibilityCondition{
			Source:   enrollmentModels.ConditionSourceCareOffering,
			Operator: enrollmentModels.ConditionOpIncludes, Value: "Mittagessen",
		},
	}
	assert.True(t, fieldVisible(f, fieldVisibilityContext{
		offeringNames: map[string]bool{"mittagessen": true},
	}), "case-insensitive name match")
	assert.False(t, fieldVisible(f, fieldVisibilityContext{
		offeringNames: map[string]bool{"regelbetreuung": true},
	}))
}

// ---- validateRequiredCustomFields ---------------------------------------

func TestValidateRequiredCustomFields_NilSchema(t *testing.T) {
	s := &requestService{}
	assert.NoError(t, s.validateRequiredCustomFields(nil, SubmitRequest{}, nil))
}

func TestValidateRequiredCustomFields_GuardianRequiredMissing(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "emergency", Label: "Notfallkontakt", Type: enrollmentModels.FormFieldText, Required: true},
	}}
	req := SubmitRequest{CustomData: map[string]any{}, Children: []SubmitChild{{}}}
	err := s.validateRequiredCustomFields(schema, req, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSubmission))
	assert.Contains(t, err.Error(), "emergency")
}

func TestValidateRequiredCustomFields_GuardianRequiredPresent(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "emergency", Label: "Notfallkontakt", Type: enrollmentModels.FormFieldText, Required: true},
	}}
	req := SubmitRequest{CustomData: map[string]any{"emergency": "Oma 0123"}, Children: []SubmitChild{{}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req, nil))
}

func TestValidateRequiredCustomFields_HiddenGuardianFieldExempt(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "has_allergy", Label: "Allergie?", Type: enrollmentModels.FormFieldBoolean},
		{Key: "which", Label: "Welche?", Type: enrollmentModels.FormFieldText, Required: true,
			VisibleWhen: &enrollmentModels.VisibilityCondition{
				Source: enrollmentModels.ConditionSourceField, Field: "has_allergy",
				Operator: enrollmentModels.ConditionOpEquals, Value: true,
			}},
	}}
	// has_allergy = false → "which" is hidden → its required flag is moot.
	req := SubmitRequest{CustomData: map[string]any{"has_allergy": false}, Children: []SubmitChild{{}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req, nil))

	// has_allergy = true → "which" is visible and required → must error.
	req2 := SubmitRequest{CustomData: map[string]any{"has_allergy": true}, Children: []SubmitChild{{}}}
	err := s.validateRequiredCustomFields(schema, req2, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "which")
}

func TestValidateRequiredCustomFields_ChildRequiredMissing(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "diet", Label: "Ernährung", Type: enrollmentModels.FormFieldText, Required: true, AppliesToCh: true},
	}}
	req := SubmitRequest{Children: []SubmitChild{
		{CustomData: map[string]any{"diet": "vegetarisch"}},
		{CustomData: map[string]any{}}, // second child missing → error names index 1
	}}
	err := s.validateRequiredCustomFields(schema, req, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "child 1")
	assert.Contains(t, err.Error(), "diet")
}

func TestValidateRequiredCustomFields_ChildHiddenByGradeExempt(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "g1_note", Label: "Hinweis Klasse 1", Type: enrollmentModels.FormFieldText, Required: true, AppliesToCh: true,
			VisibleWhen: &enrollmentModels.VisibilityCondition{
				Source:   enrollmentModels.ConditionSourceGradeLevel,
				Operator: enrollmentModels.ConditionOpEquals, Value: float64(1),
			}},
	}}
	// Grade 2 → field hidden → no answer required.
	req := SubmitRequest{Children: []SubmitChild{{TargetGradeLevel: gradePtr(2), CustomData: map[string]any{}}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req, nil))

	// Grade 1 → field visible and required → error.
	req2 := SubmitRequest{Children: []SubmitChild{{TargetGradeLevel: gradePtr(1), CustomData: map[string]any{}}}}
	require.Error(t, s.validateRequiredCustomFields(schema, req2, nil))
}

func TestValidateRequiredCustomFields_ChildHiddenByCareOfferingExempt(t *testing.T) {
	s := &requestService{}
	lunch := &enrollmentModels.CareOffering{Name: "Mittagessen"}
	lunch.ID = 42
	openByID := map[int64]*enrollmentModels.CareOffering{42: lunch}

	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "allergy_note", Label: "Allergie-Hinweis", Type: enrollmentModels.FormFieldText, Required: true, AppliesToCh: true,
			VisibleWhen: &enrollmentModels.VisibilityCondition{
				Source:   enrollmentModels.ConditionSourceCareOffering,
				Operator: enrollmentModels.ConditionOpIncludes, Value: "Mittagessen",
			}},
	}}

	// No lunch selected → field hidden → exempt.
	req := SubmitRequest{Children: []SubmitChild{{CustomData: map[string]any{}}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req, openByID))

	// Lunch selected → field visible + required + empty → error.
	req2 := SubmitRequest{Children: []SubmitChild{{OfferingIDs: []int64{42}, CustomData: map[string]any{}}}}
	require.Error(t, s.validateRequiredCustomFields(schema, req2, openByID))
}

func TestValidateRequiredCustomFields_InfoFieldNeverRequired(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		// An info block carries no answer; even if Required somehow set,
		// it must never block submit.
		{Key: "infotext_1", Type: enrollmentModels.FormFieldInfo, Content: "Hinweis", Required: true},
	}}
	req := SubmitRequest{CustomData: map[string]any{}, Children: []SubmitChild{{}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req, nil))
}

func TestValidateRequiredCustomFields_ChildFieldDependsOnGuardianAnswer(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "guardian_flag", Label: "Eltern-Flag", Type: enrollmentModels.FormFieldBoolean},
		{Key: "child_detail", Label: "Kind-Detail", Type: enrollmentModels.FormFieldText, Required: true, AppliesToCh: true,
			VisibleWhen: &enrollmentModels.VisibilityCondition{
				Source: enrollmentModels.ConditionSourceField, Field: "guardian_flag",
				Operator: enrollmentModels.ConditionOpEquals, Value: true,
			}},
	}}
	// Guardian flag off → child field hidden → exempt.
	req := SubmitRequest{
		CustomData: map[string]any{"guardian_flag": false},
		Children:   []SubmitChild{{CustomData: map[string]any{}}},
	}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req, nil))

	// Guardian flag on → child field visible + required + empty → error.
	req2 := SubmitRequest{
		CustomData: map[string]any{"guardian_flag": true},
		Children:   []SubmitChild{{CustomData: map[string]any{}}},
	}
	require.Error(t, s.validateRequiredCustomFields(schema, req2, nil))
}

func TestValidateRequiredCustomFields_StructuredSuggestedRequired(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "student_contacts", Label: "Kontakte", Type: enrollmentModels.FormFieldContactList,
			Target: enrollmentModels.TargetStudentContacts, Required: true, AppliesToCh: true},
	}}
	// No contact entered → required contact list is empty → error.
	req := SubmitRequest{Children: []SubmitChild{{CustomData: map[string]any{}}}}
	require.Error(t, s.validateRequiredCustomFields(schema, req, nil))

	// One contact → satisfied.
	req2 := SubmitRequest{Children: []SubmitChild{{CustomData: map[string]any{
		"student_contacts": []any{map[string]any{"first_name": "Oma", "last_name": "X"}},
	}}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req2, nil))
}
