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

// TestValidateAccompaniedCompanionNote pins the submit-side coupling gate
// (#1694): a child whose visible departure field allows the accompanied ("Mit
// anderem Kind") mode must carry a non-blank companion note, so a stale or
// scripted submit can never be persisted into an un-approvable state.
func TestValidateAccompaniedCompanionNote(t *testing.T) {
	svc := &requestService{}
	grade := gradePtr(2)
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{{
		Key:         "allowed_modes",
		Type:        enrollmentModels.FormFieldWeekdayMultiMode,
		Target:      enrollmentModels.TargetStudentAllowedDepartureModes,
		AppliesToCh: true,
	}}}
	withChild := func(custom map[string]any) SubmitRequest {
		return SubmitRequest{Children: []SubmitChild{{
			FirstName:        "Comp",
			LastName:         "Child",
			TargetGradeLevel: grade,
			CustomData:       custom,
		}}}
	}

	t.Run("accompanied without note is rejected", func(t *testing.T) {
		err := svc.validateAccompaniedCompanionNote(schema, withChild(map[string]any{
			"allowed_modes": map[string]any{"mon": []any{"accompanied"}},
		}), nil)
		require.ErrorIs(t, err, ErrInvalidSubmission)
	})

	t.Run("accompanied with a blank note is rejected", func(t *testing.T) {
		err := svc.validateAccompaniedCompanionNote(schema, withChild(map[string]any{
			"allowed_modes": map[string]any{"mon": []any{"accompanied"}},
			enrollmentModels.TargetStudentDepartureCompanionNote: "   ",
		}), nil)
		require.ErrorIs(t, err, ErrInvalidSubmission)
	})

	t.Run("accompanied with a note is accepted", func(t *testing.T) {
		err := svc.validateAccompaniedCompanionNote(schema, withChild(map[string]any{
			"allowed_modes": map[string]any{"mon": []any{"accompanied"}},
			enrollmentModels.TargetStudentDepartureCompanionNote: "Geschwisterkind Mia",
		}), nil)
		require.NoError(t, err)
	})

	t.Run("non-accompanied plan needs no note", func(t *testing.T) {
		err := svc.validateAccompaniedCompanionNote(schema, withChild(map[string]any{
			"allowed_modes": map[string]any{"mon": []any{"bus"}},
		}), nil)
		require.NoError(t, err)
	})
}

// ---- customValueSatisfiesRequired ---------------------------------------

func TestCustomValueSatisfiesRequired(t *testing.T) {
	text := enrollmentModels.FormField{Type: enrollmentModels.FormFieldText}
	boolean := enrollmentModels.FormField{Type: enrollmentModels.FormFieldBoolean}
	phones := enrollmentModels.FormField{Type: enrollmentModels.FormFieldPhoneList}
	contacts := enrollmentModels.FormField{Type: enrollmentModels.FormFieldContactList}
	sched := enrollmentModels.FormField{Type: enrollmentModels.FormFieldWeekdaySchedule}

	// Plain text / number / boolean.
	assert.False(t, customValueSatisfiesRequired(text, nil))
	assert.False(t, customValueSatisfiesRequired(text, ""))
	assert.False(t, customValueSatisfiesRequired(text, "   "))
	assert.True(t, customValueSatisfiesRequired(text, "x"))
	assert.True(t, customValueSatisfiesRequired(boolean, true))
	assert.True(t, customValueSatisfiesRequired(boolean, false))
	assert.False(t, customValueSatisfiesRequired(boolean, nil))

	// Structured: a non-empty array is NOT enough — every entry must be
	// well-formed (this is the regression the reviewer flagged).
	assert.False(t, customValueSatisfiesRequired(phones, nil), "nil phone list")
	assert.False(t, customValueSatisfiesRequired(phones, []any{}), "empty phone list")
	assert.False(t, customValueSatisfiesRequired(phones, []any{map[string]any{}}), "phone_list:[{}] must be rejected")
	assert.False(t, customValueSatisfiesRequired(phones, []any{map[string]any{"phone_number": "   "}}), "blank phone number rejected")
	assert.True(t, customValueSatisfiesRequired(phones, []any{map[string]any{"phone_number": "012", "phone_type": "mobile"}}), "valid phone accepted")

	assert.False(t, customValueSatisfiesRequired(contacts, []any{}), "empty contact list")
	assert.False(t, customValueSatisfiesRequired(contacts, []any{map[string]any{"first_name": "A"}}), "contact without last name + contact rejected")
	assert.False(t, customValueSatisfiesRequired(contacts, []any{map[string]any{"first_name": "A", "last_name": "B"}}), "contact without email or phone rejected")
	assert.True(t, customValueSatisfiesRequired(contacts, []any{map[string]any{"first_name": "A", "last_name": "B", "email": "a@b.test"}}), "valid contact accepted")

	// Schedule: needs at least one filled day.
	assert.False(t, customValueSatisfiesRequired(sched, map[string]any{"mon": "", "tue": ""}), "all-blank schedule rejected")
	assert.True(t, customValueSatisfiesRequired(sched, map[string]any{"mon": "08:00"}), "a filled day accepted")
	assert.False(t, customValueSatisfiesRequired(sched, nil), "nil schedule rejected")

	// weekday_boolean (Buskind): needs at least one selected day.
	busDays := enrollmentModels.FormField{Type: enrollmentModels.FormFieldWeekdayBoolean, Target: enrollmentModels.TargetStudentBus}
	assert.False(t, customValueSatisfiesRequired(busDays, nil), "nil bus days rejected")
	assert.False(t, customValueSatisfiesRequired(busDays, map[string]any{}), "empty bus days rejected")
	assert.False(t, customValueSatisfiesRequired(busDays, map[string]any{"mon": false}), "all-false bus days rejected")
	assert.True(t, customValueSatisfiesRequired(busDays, map[string]any{"mon": true}), "a selected bus day accepted")

	// weekday_boolean (Abholregelung): an empty map is the valid "Geht
	// alleine nach Hause" answer, so a required pickup field is satisfied
	// even with no day selected. A malformed value is still rejected.
	pickupDays := enrollmentModels.FormField{Type: enrollmentModels.FormFieldWeekdayBoolean, Target: enrollmentModels.TargetStudentPickupStatus}
	assert.True(t, customValueSatisfiesRequired(pickupDays, nil), "nil pickup days accepted (goes alone)")
	assert.True(t, customValueSatisfiesRequired(pickupDays, map[string]any{}), "empty pickup days accepted (goes alone)")
	assert.True(t, customValueSatisfiesRequired(pickupDays, map[string]any{"mon": true}), "selected pickup day accepted")
	assert.False(t, customValueSatisfiesRequired(pickupDays, map[string]any{"sat": true}), "invalid weekday rejected")
}

// customAnswerSatisfiesRequired adds presence-awareness on top of the value
// shape check: for a required pickup field an explicit empty map is the valid
// "Geht alleine nach Hause" answer, but a missing key (parent never touched the
// picker) must fail. Covers the three distinct states the change hinges on.
func TestCustomAnswerSatisfiesRequired_PickupPresence(t *testing.T) {
	pickup := enrollmentModels.FormField{
		Key:    "pickup",
		Type:   enrollmentModels.FormFieldWeekdayBoolean,
		Target: enrollmentModels.TargetStudentPickupStatus,
	}

	// 1) Missing key — unanswered required pickup is rejected.
	assert.False(t, customAnswerSatisfiesRequired(pickup, map[string]any{}),
		"missing pickup answer must fail required")
	assert.False(t, customAnswerSatisfiesRequired(pickup, map[string]any{"other": true}),
		"unrelated keys do not satisfy a missing pickup answer")

	// 2) Explicit empty map — the valid "goes home alone" answer is accepted.
	assert.True(t, customAnswerSatisfiesRequired(pickup, map[string]any{"pickup": map[string]any{}}),
		"explicit empty pickup map is a valid answer")

	// 3) One or more selected weekdays — accepted.
	assert.True(t, customAnswerSatisfiesRequired(pickup, map[string]any{"pickup": map[string]any{"mon": true}}),
		"selected pickup day is a valid answer")

	// A malformed value is still rejected even when present.
	assert.False(t, customAnswerSatisfiesRequired(pickup, map[string]any{"pickup": map[string]any{"sat": true}}),
		"invalid weekday rejected")

	// Non-pickup weekday_boolean (Buskind) still requires a selected day even
	// when present, and a missing key fails as before.
	bus := enrollmentModels.FormField{
		Key:    "bus",
		Type:   enrollmentModels.FormFieldWeekdayBoolean,
		Target: enrollmentModels.TargetStudentBus,
	}
	assert.False(t, customAnswerSatisfiesRequired(bus, map[string]any{}),
		"missing bus answer fails required")
	assert.False(t, customAnswerSatisfiesRequired(bus, map[string]any{"bus": map[string]any{}}),
		"empty bus map fails required (needs a selected day)")
	assert.True(t, customAnswerSatisfiesRequired(bus, map[string]any{"bus": map[string]any{"mon": true}}),
		"selected bus day accepted")
}

// The unified weekday_mode field behaves like pickup for required checks: an
// all-alone (empty) plan is a valid answer, but a never-touched field fails
// required, and malformed values are rejected (#1610).
func TestWeekdayMode_RequiredHandling(t *testing.T) {
	departure := enrollmentModels.FormField{
		Key:    "departure",
		Type:   enrollmentModels.FormFieldWeekdayMode,
		Target: enrollmentModels.TargetStudentDeparture,
	}

	// Value-shape check: a well-formed map (even empty) satisfies required.
	assert.True(t, customValueSatisfiesRequired(departure, map[string]any{}),
		"empty departure plan accepted (goes alone)")
	assert.True(t, customValueSatisfiesRequired(departure, map[string]any{"mon": "bus"}),
		"a set departure day accepted")
	assert.False(t, customValueSatisfiesRequired(departure, map[string]any{"mon": "taxi"}),
		"invalid mode rejected")

	// Presence check: missing key fails, explicit empty map passes.
	assert.False(t, customAnswerSatisfiesRequired(departure, map[string]any{}),
		"missing departure answer must fail required")
	assert.True(t, customAnswerSatisfiesRequired(departure, map[string]any{"departure": map[string]any{}}),
		"explicit empty departure map is a valid answer")
	assert.True(t, customAnswerSatisfiesRequired(departure, map[string]any{"departure": map[string]any{"wed": "pickup"}}),
		"selected departure mode accepted")
}

func TestWeekdayMultiMode_RequiredHandling(t *testing.T) {
	departure := enrollmentModels.FormField{
		Key:    "allowed_departure",
		Type:   enrollmentModels.FormFieldWeekdayMultiMode,
		Target: enrollmentModels.TargetStudentAllowedDepartureModes,
	}

	assert.True(t, customAnswerSatisfiesRequiredWeekdayMultiMode(departure, map[string]any{}, map[string]bool{}),
		"no selected care days means there is no required departure answer yet")
	assert.False(t, customAnswerSatisfiesRequiredWeekdayMultiMode(departure, map[string]any{}, map[string]bool{"mon": true}),
		"missing answer fails when a care day exists")
	assert.False(t, customAnswerSatisfiesRequiredWeekdayMultiMode(
		departure,
		map[string]any{"allowed_departure": map[string]any{"mon": []any{"bus"}}},
		map[string]bool{"mon": true, "tue": true},
	), "every selected care day needs at least one mode")
	assert.False(t, customAnswerSatisfiesRequiredWeekdayMultiMode(
		departure,
		map[string]any{"allowed_departure": map[string]any{"mon": []any{"bus"}, "fri": []any{"pickup"}}},
		map[string]bool{"mon": true},
	), "answers for non-care days are rejected")
	assert.True(t, customAnswerSatisfiesRequiredWeekdayMultiMode(
		departure,
		map[string]any{"allowed_departure": map[string]any{"mon": []any{"bus", "pickup"}, "tue": []any{"alone"}}},
		map[string]bool{"mon": true, "tue": true},
	), "selected care days with modes are accepted")
}

// TestSanitizeVisibleAnswers_CompanionNote pins the persistence-time companion
// note coupling (#1694): sanitizeVisibleAnswers must keep the reserved "mit wem"
// note whenever a visible per-child departure field allows the accompanied mode,
// for BOTH the unified allowed-modes target and the legacy student.departure
// target. Dropping it for the legacy target would let an accepted submission
// (validateAccompaniedCompanionNote also accepts the legacy path) become
// un-approvable, since the decision service would decode an accompanied
// departure with no note and studentRepo.Update would reject it.
func TestSanitizeVisibleAnswers_CompanionNote(t *testing.T) {
	const note = "Geschwisterkind Mia (1b)"
	noteKey := enrollmentModels.TargetStudentDepartureCompanionNote

	schemaWith := func(target string) *enrollmentModels.FormSchema {
		fieldType := enrollmentModels.FormFieldWeekdayMultiMode
		if target == enrollmentModels.TargetStudentDeparture {
			fieldType = enrollmentModels.FormFieldWeekdayMode
		}
		return &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{{
			Key:         "dep",
			Type:        fieldType,
			Target:      target,
			AppliesToCh: true,
		}}}
	}
	ctxFor := func(schema *enrollmentModels.FormSchema, values map[string]any) fieldVisibilityContext {
		return fieldVisibilityContext{
			childAnswers: values,
			fieldsByKey:  buildFieldsByKey(schema),
		}
	}

	t.Run("legacy departure target keeps note when accompanied", func(t *testing.T) {
		schema := schemaWith(enrollmentModels.TargetStudentDeparture)
		values := map[string]any{
			"dep":   map[string]any{"mon": "accompanied"},
			noteKey: note,
		}
		out := sanitizeVisibleAnswers(schema, true, values, ctxFor(schema, values))
		assert.Equal(t, note, out[noteKey], "note must survive for legacy student.departure accompanied plan")
	})

	t.Run("legacy departure target drops note when not accompanied", func(t *testing.T) {
		schema := schemaWith(enrollmentModels.TargetStudentDeparture)
		values := map[string]any{
			"dep":   map[string]any{"mon": "bus"},
			noteKey: note,
		}
		out := sanitizeVisibleAnswers(schema, true, values, ctxFor(schema, values))
		_, ok := out[noteKey]
		assert.False(t, ok, "orphan note must be dropped when no accompanied day")
	})

	t.Run("unified allowed-modes target keeps note when accompanied", func(t *testing.T) {
		schema := schemaWith(enrollmentModels.TargetStudentAllowedDepartureModes)
		values := map[string]any{
			"dep":   map[string]any{"mon": []any{"accompanied"}},
			noteKey: note,
		}
		out := sanitizeVisibleAnswers(schema, true, values, ctxFor(schema, values))
		assert.Equal(t, note, out[noteKey], "note must survive for unified accompanied plan")
	})
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

func TestFieldVisible_HiddenControllerCollapsesNeqDependent(t *testing.T) {
	// has_extra (boolean) controls a (select); c uses neq on a. When has_extra
	// is false, a is hidden — c must NOT stay visible via "nil != expected".
	hasExtra := &enrollmentModels.FormField{Key: "has_extra", Type: enrollmentModels.FormFieldBoolean}
	a := &enrollmentModels.FormField{
		Key: "a", Type: enrollmentModels.FormFieldSelect,
		VisibleWhen: &enrollmentModels.VisibilityCondition{
			Source: enrollmentModels.ConditionSourceField, Field: "has_extra",
			Operator: enrollmentModels.ConditionOpEquals, Value: true,
		},
	}
	c := &enrollmentModels.FormField{
		Key: "c", Type: enrollmentModels.FormFieldText,
		VisibleWhen: &enrollmentModels.VisibilityCondition{
			Source: enrollmentModels.ConditionSourceField, Field: "a",
			Operator: enrollmentModels.ConditionOpNotEquals, Value: "x",
		},
	}
	byKey := map[string]*enrollmentModels.FormField{"has_extra": hasExtra, "a": a, "c": c}

	// a visible (has_extra=true) and a != "x" → c visible.
	assert.True(t, fieldVisible(c, fieldVisibilityContext{
		guardianAnswers: map[string]any{"has_extra": true, "a": "y"}, fieldsByKey: byKey,
	}))
	// has_extra=false hides a → c must collapse to hidden despite the neq.
	assert.False(t, fieldVisible(c, fieldVisibilityContext{
		guardianAnswers: map[string]any{"has_extra": false, "a": "y"}, fieldsByKey: byKey,
	}))
}

func TestFieldVisible_CyclicConfigurationIsHidden(t *testing.T) {
	a := &enrollmentModels.FormField{
		Key: "a", Type: enrollmentModels.FormFieldSelect,
		VisibleWhen: &enrollmentModels.VisibilityCondition{
			Source: enrollmentModels.ConditionSourceField, Field: "b",
			Operator: enrollmentModels.ConditionOpEquals, Value: "x",
		},
	}
	b := &enrollmentModels.FormField{
		Key: "b", Type: enrollmentModels.FormFieldSelect,
		VisibleWhen: &enrollmentModels.VisibilityCondition{
			Source: enrollmentModels.ConditionSourceField, Field: "a",
			Operator: enrollmentModels.ConditionOpEquals, Value: "x",
		},
	}
	byKey := map[string]*enrollmentModels.FormField{"a": a, "b": b}
	// Must not recurse forever; resolves to hidden.
	assert.False(t, fieldVisible(a, fieldVisibilityContext{
		guardianAnswers: map[string]any{"a": "x", "b": "x"}, fieldsByKey: byKey,
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

	// A contact with a name but no email/phone is malformed → still an error
	// (a non-empty array is not enough; every entry must be well-formed).
	reqMalformed := SubmitRequest{Children: []SubmitChild{{CustomData: map[string]any{
		"student_contacts": []any{map[string]any{"first_name": "Oma", "last_name": "X"}},
	}}}}
	require.Error(t, s.validateRequiredCustomFields(schema, reqMalformed, nil))

	// One complete contact (name + email) → satisfied.
	req2 := SubmitRequest{Children: []SubmitChild{{CustomData: map[string]any{
		"student_contacts": []any{map[string]any{
			"first_name": "Oma", "last_name": "X", "email": "oma@example.test",
		}},
	}}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req2, nil))
}

// ---- sanitizeVisibleAnswers ---------------------------------------------

func TestSanitizeVisibleAnswers_DropsHiddenAndUnknownKeys(t *testing.T) {
	schema := &enrollmentModels.FormSchema{
		Fields: []enrollmentModels.FormField{
			{Key: "has_allergy", Type: enrollmentModels.FormFieldBoolean},
			{
				Key:  "which", // hidden unless has_allergy == true
				Type: enrollmentModels.FormFieldText,
				VisibleWhen: &enrollmentModels.VisibilityCondition{
					Source: enrollmentModels.ConditionSourceField, Field: "has_allergy",
					Operator: enrollmentModels.ConditionOpEquals, Value: true,
				},
			},
			{Key: "info", Type: enrollmentModels.FormFieldInfo},
		},
	}
	byKey := buildFieldsByKey(schema)
	// has_allergy=false → "which" is hidden. A stale/manipulated client still
	// sends a value for it, plus an info-block value and an unknown key.
	raw := map[string]any{
		"has_allergy": false,
		"which":       "smuggled value with a target",
		"info":        "should never be collected",
		"unknown_key": "not in schema",
	}
	out := sanitizeVisibleAnswers(schema, false, raw,
		fieldVisibilityContext{guardianAnswers: raw, fieldsByKey: byKey})

	assert.Equal(t, false, out["has_allergy"], "visible field kept")
	_, hasWhich := out["which"]
	assert.False(t, hasWhich, "hidden field dropped")
	_, hasInfo := out["info"]
	assert.False(t, hasInfo, "information block dropped")
	_, hasUnknown := out["unknown_key"]
	assert.False(t, hasUnknown, "key not in schema dropped")
}

func TestSanitizeVisibleAnswers_KeepsValueWhenConditionPasses(t *testing.T) {
	schema := &enrollmentModels.FormSchema{
		Fields: []enrollmentModels.FormField{
			{Key: "has_allergy", Type: enrollmentModels.FormFieldBoolean},
			{
				Key:  "which",
				Type: enrollmentModels.FormFieldText,
				VisibleWhen: &enrollmentModels.VisibilityCondition{
					Source: enrollmentModels.ConditionSourceField, Field: "has_allergy",
					Operator: enrollmentModels.ConditionOpEquals, Value: true,
				},
			},
		},
	}
	byKey := buildFieldsByKey(schema)
	raw := map[string]any{"has_allergy": true, "which": "peanuts"}
	out := sanitizeVisibleAnswers(schema, false, raw,
		fieldVisibilityContext{guardianAnswers: raw, fieldsByKey: byKey})
	assert.Equal(t, "peanuts", out["which"], "visible conditional field kept")
}

func TestSanitizeVisibleAnswers_NilSchemaReturnsEmpty(t *testing.T) {
	out := sanitizeVisibleAnswers(nil, false, map[string]any{"x": "y"}, fieldVisibilityContext{})
	assert.Empty(t, out)
}
