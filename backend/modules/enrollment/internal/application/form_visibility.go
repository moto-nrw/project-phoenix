package application

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// The server-side counterpart of the frontend visibility evaluator
// (frontend/src/lib/enrollment-field-visibility.ts). A required field the
// parent could see must carry an answer; a field hidden by its condition is
// exempt, and its answer is dropped before persistence. Keep the two
// implementations in sync.

// fieldVisibilityContext carries the answers a condition is evaluated
// against. childAnswers, gradeLevel and offeringNames are only set for a
// per-child field.
type fieldVisibilityContext struct {
	guardianAnswers map[string]any
	childAnswers    map[string]any
	gradeLevel      *int16
	offeringNames   map[string]bool // lower-cased selected offering names
	fieldsByKey     map[string]*enrollment.FormField
}

func lowerTrim(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// fieldVisible reports whether a field is shown given the current answers.
func fieldVisible(field *enrollment.FormField, ctx fieldVisibilityContext) bool {
	return fieldVisibleGuarded(field, ctx, map[string]bool{})
}

// fieldVisibleGuarded tracks the fields already on the dependency chain, so a
// cyclic visible_when configuration (A→B→A) cannot recurse forever; a field on
// a cycle is hidden.
func fieldVisibleGuarded(field *enrollment.FormField, ctx fieldVisibilityContext, seen map[string]bool) bool {
	c := field.VisibleWhen
	if c == nil {
		return true
	}
	if seen[field.Key] {
		return false
	}
	seen[field.Key] = true
	switch c.Source {
	case enrollment.ConditionSourceField:
		return fieldConditionMet(c, ctx, seen)
	case enrollment.ConditionSourceGradeLevel:
		var actual any
		if ctx.gradeLevel != nil {
			actual = int(*ctx.gradeLevel)
		}
		return matchScalar(c.Operator, actual, c.Value)
	case enrollment.ConditionSourceCareOffering:
		return ctx.offeringNames[lowerTrim(fmt.Sprintf("%v", c.Value))]
	default:
		return true
	}
}

// fieldConditionMet evaluates a field-sourced condition. A hidden controller
// hides its dependents regardless of the operator; a guardian-level
// controller is read from the guardian answers even for a per-child field.
func fieldConditionMet(c *enrollment.VisibilityCondition, ctx fieldVisibilityContext, seen map[string]bool) bool {
	controller := ctx.fieldsByKey[c.Field]
	if controller != nil && !fieldVisibleGuarded(controller, ctx, seen) {
		return false
	}
	answers := ctx.guardianAnswers
	if controller != nil && controller.AppliesToCh {
		answers = ctx.childAnswers
	}
	return matchScalar(c.Operator, answers[c.Field], c.Value)
}

// matchScalar evaluates a scalar comparison operator; "includes" is handled by
// the care-offering source and counts as visible here.
func matchScalar(operator string, actual, expected any) bool {
	switch operator {
	case enrollment.ConditionOpNotEmpty:
		return actual != nil && fmt.Sprintf("%v", actual) != ""
	case enrollment.ConditionOpEquals:
		return conditionValuesEqual(actual, expected)
	case enrollment.ConditionOpNotEquals:
		return !conditionValuesEqual(actual, expected)
	default:
		return true
	}
}

// conditionValuesEqual compares tolerant of the JSON round trip (bool vs
// "true", number vs string).
func conditionValuesEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// buildFieldsByKey indexes a schema's fields by key; nil for a nil schema.
func buildFieldsByKey(schema *enrollment.FormSchema) map[string]*enrollment.FormField {
	if schema == nil {
		return nil
	}
	byKey := make(map[string]*enrollment.FormField, len(schema.Fields))
	for i := range schema.Fields {
		byKey[schema.Fields[i].Key] = &schema.Fields[i]
	}
	return byKey
}

// childVisibilityContext is the evaluation context of one child's fields.
func childVisibilityContext(guardianAnswers map[string]any, child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering, byKey map[string]*enrollment.FormField) fieldVisibilityContext {
	return fieldVisibilityContext{
		guardianAnswers: guardianAnswers,
		childAnswers:    child.CustomData,
		gradeLevel:      child.TargetGradeLevel,
		offeringNames:   selectedOfferingNames(child, openByID),
		fieldsByKey:     byKey,
	}
}

// sanitizeVisibleAnswers keeps only the answers of visible, answer-collecting
// fields of the given scope. Answers to hidden fields and undeclared keys are
// dropped, so a stale or manipulated client cannot smuggle a value into
// custom data — and, through a field target, into the student on approval.
func sanitizeVisibleAnswers(schema *enrollment.FormSchema, appliesToChild bool, values map[string]any, ctx fieldVisibilityContext) map[string]any {
	out := make(map[string]any)
	if schema == nil {
		return out
	}
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if f.AppliesToCh != appliesToChild || f.Type == enrollment.FormFieldInfo || !fieldVisible(f, ctx) {
			continue
		}
		if v, ok := values[f.Key]; ok {
			out[f.Key] = v
		}
	}
	// The coupled "mit wem" note (#1694) rides on a reserved key next to a
	// departure field. Keep it only when a visible departure field allows the
	// accompanied mode, bounded to the rune limit the approval applies.
	if note, ok := values[enrollment.TargetStudentDepartureCompanionNote]; ok && companionNoteAllowed(schema, appliesToChild, values, ctx) {
		out[enrollment.TargetStudentDepartureCompanionNote] = truncateRunes(stringValue(note), departure.MaxDepartureCompanionNoteLen)
	}
	return out
}

// companionNoteAllowed reports whether a visible departure field of the
// scope submits a plan allowing the accompanied mode. On a decode error the
// note is kept and the approval, the authoritative guard, decides.
func companionNoteAllowed(schema *enrollment.FormSchema, appliesToChild bool, values map[string]any, ctx fieldVisibilityContext) bool {
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if f.AppliesToCh != appliesToChild || !fieldVisible(f, ctx) {
			continue
		}
		switch f.Target {
		case enrollment.TargetStudentAllowedDepartureModes:
			if modes, err := decodeAllowedDepartureModes(values[f.Key]); err != nil || modes.HasMode(departure.DepartureAccompanied) {
				return true
			}
		case enrollment.TargetStudentDeparture:
			if days, err := decodeDepartureDays(values[f.Key]); err != nil || days.HasMode(departure.DepartureAccompanied) {
				return true
			}
		}
	}
	return false
}

// mergeEditableCustomData keeps stored keys the current schema cannot edit
// and lets the sanitized submitted answers replace the editable ones, so a
// full edit snapshot never wipes data the reopened form cannot render.
func mergeEditableCustomData(existing, submitted map[string]any, schema *enrollment.FormSchema, appliesToChild bool) map[string]any {
	out := make(map[string]any)
	editable := editableCustomDataKeys(schema, appliesToChild)
	for key, value := range existing {
		if !editable[key] {
			out[key] = value
		}
	}
	for key, value := range submitted {
		out[key] = value
	}
	return out
}

func existingChildCustomDataBySubmittedIdentity(existing []*RequestChild, submitted []SubmitChild) []map[string]any {
	out := make([]map[string]any, len(submitted))
	for i, child := range matchExistingChildrenBySubmittedIdentity(existing, submitted) {
		if child != nil {
			out[i] = child.CustomData
		}
	}
	return out
}

// matchExistingChildrenBySubmittedIdentity pairs replacement children with
// the stored ones: a persisted ID is authoritative; an ID-less child matches
// only when its normalized name and birthday are unique on both sides.
func matchExistingChildrenBySubmittedIdentity(existing []*RequestChild, submitted []SubmitChild) []*RequestChild {
	matches := make([]*RequestChild, len(submitted))
	used := matchChildrenByID(existing, submitted, matches)
	existingByIdentity := make(map[string][]*RequestChild)
	for _, child := range existing {
		if child == nil || child.ID <= 0 || used[child.ID] {
			continue
		}
		key := requestChildIdentityKey(child.FirstName, child.LastName, child.DateOfBirth.String())
		existingByIdentity[key] = append(existingByIdentity[key], child)
	}
	incomingByIdentity := make(map[string][]int)
	for i, incoming := range submitted {
		if matches[i] != nil || incoming.ID > 0 {
			continue
		}
		key := requestChildIdentityKey(incoming.FirstName, incoming.LastName, incoming.DateOfBirth.String())
		incomingByIdentity[key] = append(incomingByIdentity[key], i)
	}
	for key, indexes := range incomingByIdentity {
		candidates := existingByIdentity[key]
		if len(indexes) == 1 && len(candidates) == 1 {
			matches[indexes[0]] = candidates[0]
		}
	}
	return matches
}

// matchChildrenByID pairs submitted children carrying a stored ID and
// reports the stored IDs it used.
func matchChildrenByID(existing []*RequestChild, submitted []SubmitChild, matches []*RequestChild) map[int64]bool {
	byID := make(map[int64]*RequestChild, len(existing))
	used := make(map[int64]bool, len(existing))
	for _, child := range existing {
		if child != nil && child.ID > 0 {
			byID[child.ID] = child
		}
	}
	for i, incoming := range submitted {
		if incoming.ID <= 0 {
			continue
		}
		if child := byID[incoming.ID]; child != nil && !used[child.ID] {
			matches[i] = child
			used[child.ID] = true
		}
	}
	return used
}

func requestChildIdentityKey(firstName, lastName, dateOfBirth string) string {
	return lowerTrim(firstName) + "\x00" + lowerTrim(lastName) + "\x00" + strings.TrimSpace(dateOfBirth)
}

// sameSubmittedIdentity reports whether an edited child still names the same
// person as the stored child it was paired to; a changed identity drops a
// stale existing-student pin (#1663).
func sameSubmittedIdentity(existing *RequestChild, submitted SubmitChild) bool {
	if existing == nil {
		return false
	}
	return requestChildIdentityKey(existing.FirstName, existing.LastName, existing.DateOfBirth.String()) ==
		requestChildIdentityKey(submitted.FirstName, submitted.LastName, submitted.DateOfBirth.String())
}

func editableCustomDataKeys(schema *enrollment.FormSchema, appliesToChild bool) map[string]bool {
	keys := make(map[string]bool)
	if schema == nil {
		return keys
	}
	for i := range schema.Fields {
		field := &schema.Fields[i]
		if field.AppliesToCh != appliesToChild || field.Type == enrollment.FormFieldInfo {
			continue
		}
		keys[field.Key] = true
		if appliesToChild && (field.Target == enrollment.TargetStudentAllowedDepartureModes || field.Target == enrollment.TargetStudentDeparture) {
			keys[enrollment.TargetStudentDepartureCompanionNote] = true
		}
	}
	return keys
}

// relevantCareDaysForChild returns the weekdays every per-child day-scoped
// field offers: all weekdays without care offerings, otherwise the child's
// selected care days. The result is read-only.
func relevantCareDaysForChild(child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) map[string]bool {
	if len(openByID) == 0 {
		return enrollment.ValidWeekdays
	}
	return selectedCareDays(child, openByID)
}

func selectedCareDays(child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) map[string]bool {
	picksByOffering := make(map[int64][]string, len(child.OfferingDays))
	for _, pick := range child.OfferingDays {
		picksByOffering[pick.OfferingID] = pick.SelectedDays
	}
	days := map[string]bool{}
	for _, id := range child.OfferingIDs {
		offering := openByID[id]
		if offering == nil {
			continue
		}
		selected := offering.AvailableDays
		if offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeParentChoice {
			selected = picksByOffering[id]
		}
		for _, day := range selected {
			if enrollment.ValidWeekdays[day] {
				days[day] = true
			}
		}
	}
	return days
}

// pruneChildScheduleAnswers strips per-child weekday_schedule entries for
// weekdays the child cannot schedule; the approval turns every stored weekday
// into a schedule row. Mutates answers in place.
func pruneChildScheduleAnswers(schema *enrollment.FormSchema, answers map[string]any, scheduleDays map[string]bool) {
	if schema == nil || answers == nil {
		return
	}
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if !f.AppliesToCh || f.Type != enrollment.FormFieldWeekdaySchedule {
			continue
		}
		raw, ok := answers[f.Key]
		if !ok || raw == nil {
			continue
		}
		var sched enrollment.WeekdaySchedule
		if err := decodeStructured(raw, &sched); err != nil {
			continue
		}
		answers[f.Key] = scheduleOnDays(sched, scheduleDays)
	}
}

// scheduleOnDays keeps the schedule entries of the given days.
func scheduleOnDays(sched enrollment.WeekdaySchedule, days map[string]bool) map[string]any {
	pruned := make(map[string]any, len(sched))
	for day, t := range sched {
		if days[day] {
			pruned[day] = t
		}
	}
	return pruned
}

// jsonEqual compares two values by their JSON encoding.
func jsonEqual(left, right any) bool {
	return reflect.DeepEqual(jsonNormalized(left), jsonNormalized(right))
}

func jsonNormalized(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return v
	}
	return out
}
