package enrollment

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/users"
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

	t.Run("oversized note is bounded before storage", func(t *testing.T) {
		schema := schemaWith(enrollmentModels.TargetStudentAllowedDepartureModes)
		// A client that bypasses the React maxLength must not get an unbounded
		// note persisted into custom_data (a storage/echo surface before
		// approval). It is capped at MaxDepartureCompanionNoteLen runes (#1694).
		oversized := strings.Repeat("ä", users.MaxDepartureCompanionNoteLen+50)
		values := map[string]any{
			"dep":   map[string]any{"mon": []any{"accompanied"}},
			noteKey: oversized,
		}
		out := sanitizeVisibleAnswers(schema, true, values, ctxFor(schema, values))
		stored, ok := out[noteKey].(string)
		require.True(t, ok, "stored note must be a string")
		assert.Equal(t, users.MaxDepartureCompanionNoteLen, utf8.RuneCountInString(stored),
			"oversized note must be truncated to the rune cap")
	})
}

func TestWeekdaySchedule_RequiredHandling(t *testing.T) {
	pickup := enrollmentModels.FormField{
		Key:    "dismissal",
		Type:   enrollmentModels.FormFieldWeekdaySchedule,
		Target: enrollmentModels.TargetSchedulePickup,
	}

	assert.True(t, customAnswerSatisfiesRequiredWeekdaySchedule(pickup, map[string]any{}, map[string]bool{}, true),
		"no selected care days means there is nothing to schedule yet (mirrors the hidden form input)")
	assert.False(t, customAnswerSatisfiesRequiredWeekdaySchedule(pickup, map[string]any{}, map[string]bool{"mon": true}, true),
		"missing answer fails when a care day exists")
	assert.False(t, customAnswerSatisfiesRequiredWeekdaySchedule(
		pickup,
		map[string]any{"dismissal": map[string]any{"mon": "", "tue": ""}},
		map[string]bool{"mon": true, "tue": true},
		true,
	), "an all-blank schedule on care days is not answered")
	assert.False(t, customAnswerSatisfiesRequiredWeekdaySchedule(
		pickup,
		map[string]any{"dismissal": map[string]any{"fri": "15:00"}},
		map[string]bool{"mon": true},
		true,
	), "a time on a non-care day does not satisfy a care day requirement")

	// Constrained (care days derived from selected offerings): every care day
	// needs a time -- a partially filled schedule is not answered.
	assert.False(t, customAnswerSatisfiesRequiredWeekdaySchedule(
		pickup,
		map[string]any{"dismissal": map[string]any{"mon": "15:00"}},
		map[string]bool{"mon": true, "tue": true},
		true,
	), "constrained: an unfilled care day (tue) is not answered")
	assert.True(t, customAnswerSatisfiesRequiredWeekdaySchedule(
		pickup,
		map[string]any{"dismissal": map[string]any{"mon": "15:00", "tue": "16:00"}},
		map[string]bool{"mon": true, "tue": true},
		true,
	), "constrained: every care day filled is answered")

	// Unconstrained (no offerings, all weekdays shown): at least one day with a
	// time suffices, mirroring the lenient client path for offering-less phases.
	assert.True(t, customAnswerSatisfiesRequiredWeekdaySchedule(
		pickup,
		map[string]any{"dismissal": map[string]any{"mon": "15:00"}},
		map[string]bool{"mon": true, "tue": true},
		false,
	), "unconstrained: at least one day with a time is accepted")
	assert.False(t, customAnswerSatisfiesRequiredWeekdaySchedule(
		pickup,
		map[string]any{"dismissal": map[string]any{"mon": "", "tue": ""}},
		map[string]bool{"mon": true, "tue": true},
		false,
	), "unconstrained: an all-blank schedule is still not answered")

	// A malformed answer (not a schedule object) fails to decode and counts as
	// unanswered for a required field.
	assert.False(t, customAnswerSatisfiesRequiredWeekdaySchedule(
		pickup,
		map[string]any{"dismissal": "not-a-schedule"},
		map[string]bool{"mon": true},
		true,
	), "a non-decodable answer is not a valid schedule")
}

func TestRelevantCareDaysForChild(t *testing.T) {
	fixed := &enrollmentModels.CareOffering{
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"tue", "thu"},
	}
	fixed.ID = 7
	openByID := map[int64]*enrollmentModels.CareOffering{7: fixed}

	// No offerings configured -> all weekdays (the form shows them all).
	assert.Equal(t, enrollmentModels.ValidWeekdays,
		relevantCareDaysForChild(SubmitChild{}, nil),
		"no offerings means every weekday is schedulable")

	// Offerings exist, child selected the Tue/Thu block -> just those days.
	assert.Equal(t, map[string]bool{"tue": true, "thu": true},
		relevantCareDaysForChild(SubmitChild{OfferingIDs: []int64{7}}, openByID))

	// Offerings exist, child selected none -> empty (the form hides the input).
	assert.Empty(t, relevantCareDaysForChild(SubmitChild{}, openByID))
}

func TestPruneChildScheduleAnswers(t *testing.T) {
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "dismissal", Type: enrollmentModels.FormFieldWeekdaySchedule,
			Target: enrollmentModels.TargetSchedulePickup, AppliesToCh: true},
	}}

	// Tue/Thu care: a smuggled Friday entry is stripped, Tue kept.
	answers := map[string]any{"dismissal": map[string]any{"tue": "15:00", "fri": "15:00"}}
	pruneChildScheduleAnswers(schema, answers, map[string]bool{"tue": true, "thu": true})
	assert.Equal(t, map[string]any{"tue": "15:00"}, answers["dismissal"])

	// No offerings (all weekdays schedulable): mon-fri kept, weekend dropped.
	answers2 := map[string]any{"dismissal": map[string]any{"mon": "15:00", "sat": "09:00"}}
	pruneChildScheduleAnswers(schema, answers2, enrollmentModels.ValidWeekdays)
	assert.Equal(t, map[string]any{"mon": "15:00"}, answers2["dismissal"])
}

func TestValidateRequiredCustomFields_ChildRequiredScheduleNoOfferingsEnforced(t *testing.T) {
	// P2: a phase with NO care offerings renders all weekdays for a required
	// schedule, so the server must still enforce it (no empty-care-days
	// exemption). openByID is empty in this configuration.
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "dismissal", Label: "Abholzeiten", Type: enrollmentModels.FormFieldWeekdaySchedule,
			Target: enrollmentModels.TargetSchedulePickup, Required: true, AppliesToCh: true},
	}}

	req := SubmitRequest{Children: []SubmitChild{{CustomData: map[string]any{}}}}
	err := s.validateRequiredCustomFields(schema, req, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dismissal")

	req2 := SubmitRequest{Children: []SubmitChild{{CustomData: map[string]any{"dismissal": map[string]any{"mon": "15:00"}}}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req2, nil))
}

func TestValidateRequiredCustomFields_ChildRequiredMultiModeNoOfferingsEnforced(t *testing.T) {
	// Reviewer blocker: a phase with NO care offerings renders all weekdays for
	// a required weekday_multi_mode field, so the server must still enforce it.
	// selectedCareDays would be empty here and wrongly exempt the field, letting
	// a scripted client skip a field the public form requires. openByID is empty.
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "allowed_departure", Label: "Abholarten", Type: enrollmentModels.FormFieldWeekdayMultiMode,
			Target: enrollmentModels.TargetStudentAllowedDepartureModes, Required: true, AppliesToCh: true},
	}}

	// Missing answer -> must error (every weekday is relevant and required).
	req := SubmitRequest{Children: []SubmitChild{{CustomData: map[string]any{}}}}
	err := s.validateRequiredCustomFields(schema, req, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allowed_departure")

	// A partial answer (only some weekdays) is still rejected: with no
	// offerings every weekday must carry at least one mode.
	partial := map[string]any{"allowed_departure": map[string]any{"mon": []any{"bus"}}}
	req2 := SubmitRequest{Children: []SubmitChild{{CustomData: partial}}}
	require.Error(t, s.validateRequiredCustomFields(schema, req2, nil))

	// A mode on every weekday -> satisfied.
	full := map[string]any{"allowed_departure": map[string]any{
		"mon": []any{"bus"}, "tue": []any{"bus"}, "wed": []any{"bus"},
		"thu": []any{"bus"}, "fri": []any{"bus"},
	}}
	req3 := SubmitRequest{Children: []SubmitChild{{CustomData: full}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req3, nil))
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

func TestValidateRequiredCustomFields_ChildRequiredScheduleNoCareDaysExempt(t *testing.T) {
	// The reviewer case: optional care + a required per-child weekday_schedule.
	// The public form only renders the child's care weekdays, so a child in no
	// care has no fillable input. The server must mirror that and not reject the
	// otherwise-complete submit it let through.
	s := &requestService{}
	lunch := &enrollmentModels.CareOffering{
		Name:           "Mittagessen",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon", "tue"},
	}
	lunch.ID = 42
	openByID := map[int64]*enrollmentModels.CareOffering{42: lunch}

	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "dismissal", Label: "Abholzeiten", Type: enrollmentModels.FormFieldWeekdaySchedule,
			Target: enrollmentModels.TargetSchedulePickup, Required: true, AppliesToCh: true},
	}}

	// No care offering selected → no care days → required schedule is exempt.
	req := SubmitRequest{Children: []SubmitChild{{CustomData: map[string]any{}}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req, openByID))

	// Care days present (mon/tue) but no schedule answer → required → error.
	req2 := SubmitRequest{Children: []SubmitChild{{OfferingIDs: []int64{42}, CustomData: map[string]any{}}}}
	err := s.validateRequiredCustomFields(schema, req2, openByID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dismissal")

	// Care days present (mon/tue) but only one day filled → still missing the
	// other care day → error. The shown days are the child's care days, so each
	// one needs a time.
	req3 := SubmitRequest{Children: []SubmitChild{{
		OfferingIDs: []int64{42},
		CustomData:  map[string]any{"dismissal": map[string]any{"mon": "15:00"}},
	}}}
	err = s.validateRequiredCustomFields(schema, req3, openByID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dismissal")

	// Every care day carries a time → satisfied.
	req4 := SubmitRequest{Children: []SubmitChild{{
		OfferingIDs: []int64{42},
		CustomData:  map[string]any{"dismissal": map[string]any{"mon": "15:00", "tue": "16:00"}},
	}}}
	assert.NoError(t, s.validateRequiredCustomFields(schema, req4, openByID))
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

func TestMergeEditableCustomData_PreservesLegacyKeysAndReplacesSchemaFields(t *testing.T) {
	schema := &enrollmentModels.FormSchema{
		Fields: []enrollmentModels.FormField{
			{Key: "visible", Type: enrollmentModels.FormFieldText},
			{Key: "hidden", Type: enrollmentModels.FormFieldText},
			{Key: "info", Type: enrollmentModels.FormFieldInfo},
		},
	}
	out := mergeEditableCustomData(
		map[string]any{
			"legacy":  "keep",
			"visible": "old",
			"hidden":  "old hidden answer",
			"info":    "stored before this became an info block",
		},
		map[string]any{"visible": "new"},
		schema,
		false,
	)

	assert.Equal(t, "keep", out["legacy"], "legacy keys outside the editable schema survive")
	assert.Equal(t, "new", out["visible"], "submitted visible fields replace stored values")
	assert.Equal(t, "stored before this became an info block", out["info"], "non-answer schema keys are preserved")
	_, hasHidden := out["hidden"]
	assert.False(t, hasHidden, "omitted schema answer fields are cleared")
}

func TestMergeEditableCustomData_TreatsCompanionNoteAsEditableWithDepartureField(t *testing.T) {
	schema := &enrollmentModels.FormSchema{
		Fields: []enrollmentModels.FormField{
			{
				Key:         "departure",
				Type:        enrollmentModels.FormFieldWeekdayMultiMode,
				AppliesToCh: true,
				Target:      enrollmentModels.TargetStudentAllowedDepartureModes,
			},
		},
	}
	out := mergeEditableCustomData(
		map[string]any{
			enrollmentModels.TargetStudentDepartureCompanionNote: "Mia",
			"legacy_child_note": "keep",
		},
		map[string]any{"departure": map[string]any{"mon": []any{"alone"}}},
		schema,
		true,
	)

	_, hasCompanion := out[enrollmentModels.TargetStudentDepartureCompanionNote]
	assert.False(t, hasCompanion, "stale companion notes are cleared with departure answers")
	assert.Equal(t, "keep", out["legacy_child_note"])
}

// ---- validateConstrainedSchedules ---------------------------------------

func pickupSchedField() enrollmentModels.FormField {
	return enrollmentModels.FormField{
		Key: "pickup_times", Label: "Abholzeiten",
		Type:        enrollmentModels.FormFieldWeekdaySchedule,
		AppliesToCh: true, Target: enrollmentModels.TargetSchedulePickup,
		AllowedTimes: []string{"14:45", "16:00"},
	}
}

func TestValidateConstrainedSchedules_NilSchema(t *testing.T) {
	s := &requestService{}
	assert.NoError(t, s.validateConstrainedSchedules(nil, SubmitRequest{}, nil))
}

func TestValidateConstrainedSchedules_AcceptsListedTimes(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{pickupSchedField()}}
	req := SubmitRequest{Children: []SubmitChild{
		{CustomData: map[string]any{"pickup_times": map[string]any{"mon": "14:45", "fri": "16:00"}}},
	}}
	assert.NoError(t, s.validateConstrainedSchedules(schema, req, nil))
}

func TestValidateConstrainedSchedules_RejectsOffListTime(t *testing.T) {
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{pickupSchedField()}}
	req := SubmitRequest{Children: []SubmitChild{
		{CustomData: map[string]any{"pickup_times": map[string]any{"mon": "15:00"}}},
	}}
	err := s.validateConstrainedSchedules(schema, req, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSubmission))
	assert.Contains(t, err.Error(), "child 0")
	assert.Contains(t, err.Error(), "not an allowed pickup time")
}

func TestValidateConstrainedSchedules_OffListWrapsPickupSentinel(t *testing.T) {
	// The off-list rejection must carry the specific ErrPickupTimeNotAllowed
	// identity (so the handler can attach a stable code) while still being
	// part of the broad ErrInvalidSubmission category.
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{pickupSchedField()}}
	req := SubmitRequest{Children: []SubmitChild{
		{CustomData: map[string]any{"pickup_times": map[string]any{"mon": "15:00"}}},
	}}
	err := s.validateConstrainedSchedules(schema, req, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPickupTimeNotAllowed))
	assert.True(t, errors.Is(err, ErrInvalidSubmission))
}

func TestValidateConstrainedSchedules_NoRestrictionSkips(t *testing.T) {
	// AllowedTimes empty → free entry → any well-formed time passes.
	s := &requestService{}
	field := pickupSchedField()
	field.AllowedTimes = nil
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{field}}
	req := SubmitRequest{Children: []SubmitChild{
		{CustomData: map[string]any{"pickup_times": map[string]any{"mon": "15:00"}}},
	}}
	assert.NoError(t, s.validateConstrainedSchedules(schema, req, nil))
}

func TestValidateConstrainedSchedules_HiddenFieldExempt(t *testing.T) {
	// A field hidden by its show-if condition is skipped — its answer is
	// dropped before persistence, so an off-list value must not block submit.
	s := &requestService{}
	field := pickupSchedField()
	field.VisibleWhen = &enrollmentModels.VisibilityCondition{
		Source: enrollmentModels.ConditionSourceField, Field: "needs_pickup",
		Operator: enrollmentModels.ConditionOpEquals, Value: true,
	}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{
		{Key: "needs_pickup", Label: "Wird abgeholt?", Type: enrollmentModels.FormFieldBoolean, AppliesToCh: true},
		field,
	}}
	req := SubmitRequest{Children: []SubmitChild{
		{CustomData: map[string]any{"needs_pickup": false, "pickup_times": map[string]any{"mon": "15:00"}}},
	}}
	assert.NoError(t, s.validateConstrainedSchedules(schema, req, nil))
}

func TestValidateConstrainedSchedules_OffListTimeOnNonCareDayIgnored(t *testing.T) {
	// Reviewer blocker: the allowed-times gate must only inspect the child's
	// schedulable (care) days. An off-list pickup time on a weekday the child
	// has no care on is stripped before persistence (pruneChildScheduleAnswers),
	// so rejecting it here would block an otherwise valid submit.
	s := &requestService{}
	schema := &enrollmentModels.FormSchema{Fields: []enrollmentModels.FormField{pickupSchedField()}}

	fixed := &enrollmentModels.CareOffering{
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"tue", "thu"},
	}
	fixed.ID = 7
	openByID := map[int64]*enrollmentModels.CareOffering{7: fixed}

	// tue (care day) carries an allowed time; mon (non-care day) carries an
	// off-list one -> mon is out of scope, so the submit is accepted.
	ok := SubmitRequest{Children: []SubmitChild{{
		OfferingIDs: []int64{7},
		CustomData:  map[string]any{"pickup_times": map[string]any{"tue": "14:45", "mon": "15:00"}},
	}}}
	assert.NoError(t, s.validateConstrainedSchedules(schema, ok, openByID))

	// An off-list time on a CARE day is still rejected.
	bad := SubmitRequest{Children: []SubmitChild{{
		OfferingIDs: []int64{7},
		CustomData:  map[string]any{"pickup_times": map[string]any{"tue": "15:00"}},
	}}}
	err := s.validateConstrainedSchedules(schema, bad, openByID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPickupTimeNotAllowed))
}

func TestMatchExistingChildrenBySubmittedIdentityNeverFallsBackToPosition(t *testing.T) {
	existing := []*enrollmentModels.RequestChild{
		{FirstName: "Anna", LastName: "Beispiel", DateOfBirth: timezone.NewDate(2018, 4, 15)},
		{FirstName: "Ben", LastName: "Beispiel", DateOfBirth: timezone.NewDate(2019, 8, 1)},
	}
	existing[0].ID = 11
	existing[1].ID = 12

	matches := matchExistingChildrenBySubmittedIdentity(existing, []SubmitChild{
		{ID: 12, FirstName: "Ben", LastName: "Beispiel", DateOfBirth: timezone.NewDate(2019, 8, 1)},
		{FirstName: "Clara", LastName: "Beispiel", DateOfBirth: timezone.NewDate(2020, 9, 2)},
	})
	require.Len(t, matches, 2)
	assert.Same(t, existing[1], matches[0])
	assert.Nil(t, matches[1], "new child must not inherit the removed child's hidden state")
}

func TestMatchExistingChildrenBySubmittedIdentityAllowsUniqueIDLessReorder(t *testing.T) {
	existing := []*enrollmentModels.RequestChild{
		{FirstName: "Anna", LastName: "Beispiel", DateOfBirth: timezone.NewDate(2018, 4, 15)},
		{FirstName: "Ben", LastName: "Beispiel", DateOfBirth: timezone.NewDate(2019, 8, 1)},
	}
	existing[0].ID = 11
	existing[1].ID = 12

	matches := matchExistingChildrenBySubmittedIdentity(existing, []SubmitChild{
		{FirstName: " ben ", LastName: "BEISPIEL", DateOfBirth: timezone.NewDate(2019, 8, 1)},
		{FirstName: "ANNA", LastName: "Beispiel", DateOfBirth: timezone.NewDate(2018, 4, 15)},
	})
	require.Len(t, matches, 2)
	assert.Same(t, existing[1], matches[0])
	assert.Same(t, existing[0], matches[1])
}
