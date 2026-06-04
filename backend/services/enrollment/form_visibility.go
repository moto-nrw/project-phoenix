package enrollment

import (
	"fmt"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// This file holds the server-side counterpart of the frontend visibility
// evaluator (frontend/src/lib/enrollment-field-visibility.ts). It powers
// defense-in-depth enforcement of required custom fields in
// requestService.Submit: a required field that the parent could actually
// see (its condition passed) must carry an answer, while a field hidden
// by its condition is exempt. Keep the two implementations in sync.

// fieldVisibilityContext carries the answers a condition is evaluated
// against. childAnswers / gradeLevel / offeringNames are only populated
// when evaluating a per-child field.
type fieldVisibilityContext struct {
	guardianAnswers map[string]any
	childAnswers    map[string]any
	gradeLevel      *int16
	offeringNames   map[string]bool // lowercased selected offering names
	fieldsByKey     map[string]*enrollmentModels.FormField
}

// fieldVisible reports whether a field is shown given the current answers.
func fieldVisible(field *enrollmentModels.FormField, ctx fieldVisibilityContext) bool {
	c := field.VisibleWhen
	if c == nil {
		return true
	}
	switch c.Source {
	case enrollmentModels.ConditionSourceField:
		// Resolve which answer map holds the controlling field: a
		// guardian-level controller is read from guardianAnswers even
		// when the owning field is per-child.
		answers := ctx.guardianAnswers
		if controller := ctx.fieldsByKey[c.Field]; controller != nil && controller.AppliesToCh {
			answers = ctx.childAnswers
		}
		return matchScalar(c.Operator, answers[c.Field], c.Value)
	case enrollmentModels.ConditionSourceGradeLevel:
		var actual any
		if ctx.gradeLevel != nil {
			actual = int(*ctx.gradeLevel)
		}
		return matchScalar(c.Operator, actual, c.Value)
	case enrollmentModels.ConditionSourceCareOffering:
		want := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", c.Value)))
		return ctx.offeringNames[want]
	default:
		return true
	}
}

// matchScalar evaluates a scalar comparison operator. The "includes"
// operator is care-offering-specific and handled by fieldVisible, so it
// falls through to "visible" here.
func matchScalar(operator string, actual, expected any) bool {
	switch operator {
	case enrollmentModels.ConditionOpNotEmpty:
		return actual != nil && fmt.Sprintf("%v", actual) != ""
	case enrollmentModels.ConditionOpEquals:
		return conditionValuesEqual(actual, expected)
	case enrollmentModels.ConditionOpNotEquals:
		return !conditionValuesEqual(actual, expected)
	default:
		return true
	}
}

// answerEmpty reports whether a custom-field answer is missing. A present
// boolean (true or false) counts as answered; list/schedule fields need
// at least one entry; everything else must be a non-blank value. Mirrors
// the client-side isAnswerEmpty in enrollment-form.tsx.
func answerEmpty(field *enrollmentModels.FormField, v any) bool {
	switch field.Type {
	case enrollmentModels.FormFieldPhoneList, enrollmentModels.FormFieldContactList:
		arr, ok := v.([]any)
		return !ok || len(arr) == 0
	case enrollmentModels.FormFieldWeekdaySchedule:
		m, ok := v.(map[string]any)
		if !ok {
			return true
		}
		for _, val := range m {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				return false
			}
		}
		return true
	default:
		switch x := v.(type) {
		case nil:
			return true
		case string:
			return strings.TrimSpace(x) == ""
		case bool:
			return false
		default:
			return fmt.Sprintf("%v", v) == ""
		}
	}
}

// selectedOfferingNames resolves a child's selected offering ids to a set
// of lowercased names for care-offering condition matching.
func selectedOfferingNames(child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) map[string]bool {
	names := make(map[string]bool, len(child.OfferingIDs))
	for _, id := range child.OfferingIDs {
		if o, ok := openByID[id]; ok {
			names[strings.ToLower(strings.TrimSpace(o.Name))] = true
		}
	}
	return names
}

// buildFieldsByKey indexes a schema's fields by key for condition
// resolution. Returns nil for a nil schema.
func buildFieldsByKey(schema *enrollmentModels.FormSchema) map[string]*enrollmentModels.FormField {
	if schema == nil {
		return nil
	}
	byKey := make(map[string]*enrollmentModels.FormField, len(schema.Fields))
	for i := range schema.Fields {
		byKey[schema.Fields[i].Key] = &schema.Fields[i]
	}
	return byKey
}

// sanitizeVisibleAnswers returns a new map containing only the answers for
// schema fields of the given scope (guardian vs per-child) that are visible
// (their show-if condition passes) and actually collect an answer (not
// information blocks). Answers to hidden fields AND keys not declared in the
// schema are dropped.
//
// This is the persistence-time counterpart of the client-side
// visibleAnswerData: a stale or manipulated client must not be able to smuggle
// a value for a field the parent never saw. Such a value would otherwise be
// stored in custom_data and, if the field carries a Target, written into
// student data by the decision service on approval.
func sanitizeVisibleAnswers(
	schema *enrollmentModels.FormSchema,
	appliesToChild bool,
	values map[string]any,
	ctx fieldVisibilityContext,
) map[string]any {
	out := make(map[string]any)
	if schema == nil {
		return out
	}
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if f.AppliesToCh != appliesToChild || f.Type == enrollmentModels.FormFieldInfo {
			continue
		}
		if !fieldVisible(f, ctx) {
			continue
		}
		if v, ok := values[f.Key]; ok {
			out[f.Key] = v
		}
	}
	return out
}

// validateRequiredCustomFields enforces the schema's required custom
// fields against the submission, skipping fields hidden by a visibility
// condition and information blocks. Mirrors the client-side check in
// enrollment-form.tsx so a stale or scripted client can't bypass it.
func (s *requestService) validateRequiredCustomFields(
	schema *enrollmentModels.FormSchema,
	req SubmitRequest,
	openByID map[int64]*enrollmentModels.CareOffering,
) error {
	if schema == nil {
		return nil
	}

	byKey := make(map[string]*enrollmentModels.FormField, len(schema.Fields))
	for i := range schema.Fields {
		byKey[schema.Fields[i].Key] = &schema.Fields[i]
	}

	guardianCtx := fieldVisibilityContext{
		guardianAnswers: req.CustomData,
		fieldsByKey:     byKey,
	}
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if f.AppliesToCh || f.Type == enrollmentModels.FormFieldInfo || !f.Required {
			continue
		}
		if !fieldVisible(f, guardianCtx) {
			continue
		}
		if answerEmpty(f, req.CustomData[f.Key]) {
			return fmt.Errorf("%w: field %q is required", ErrInvalidSubmission, f.Key)
		}
	}

	for idx := range req.Children {
		child := req.Children[idx]
		childCtx := fieldVisibilityContext{
			guardianAnswers: req.CustomData,
			childAnswers:    child.CustomData,
			gradeLevel:      child.TargetGradeLevel,
			offeringNames:   selectedOfferingNames(child, openByID),
			fieldsByKey:     byKey,
		}
		for i := range schema.Fields {
			f := &schema.Fields[i]
			if !f.AppliesToCh || f.Type == enrollmentModels.FormFieldInfo || !f.Required {
				continue
			}
			if !fieldVisible(f, childCtx) {
				continue
			}
			if answerEmpty(f, child.CustomData[f.Key]) {
				return fmt.Errorf("%w: child %d field %q is required", ErrInvalidSubmission, idx, f.Key)
			}
		}
	}
	return nil
}
