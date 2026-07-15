package enrollment

import (
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/users"
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
	return fieldVisibleGuarded(field, ctx, map[string]bool{})
}

// fieldVisibleGuarded evaluates visibility while tracking the fields already
// on the dependency chain, so a cyclic visible_when configuration (A→B→A)
// can't recurse forever. A field on a cycle is treated as hidden. Mirrors the
// frontend evaluator in enrollment-field-visibility.ts.
func fieldVisibleGuarded(field *enrollmentModels.FormField, ctx fieldVisibilityContext, seen map[string]bool) bool {
	c := field.VisibleWhen
	if c == nil {
		return true
	}
	if seen[field.Key] {
		return false
	}
	seen[field.Key] = true
	switch c.Source {
	case enrollmentModels.ConditionSourceField:
		controller := ctx.fieldsByKey[c.Field]
		// If the controlling field is itself hidden (its own condition
		// fails, recursively), this field collapses with it — hidden,
		// regardless of operator. Evaluating the operator against a nil/stale
		// controller value would wrongly keep a `neq` dependent visible
		// (nil != expected), so short-circuit to false here.
		if controller != nil && !fieldVisibleGuarded(controller, ctx, seen) {
			return false
		}
		// Resolve which answer map holds the controlling field: a
		// guardian-level controller is read from guardianAnswers even
		// when the owning field is per-child.
		answers := ctx.guardianAnswers
		if controller != nil && controller.AppliesToCh {
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
	// Preserve the coupled "mit wem" note (#1694): it rides on a reserved key
	// alongside a departure field rather than being its own schema field, so the
	// generic strip above would drop it. Keep it only when a visible per-child
	// departure field submits a plan that allows the accompanied mode — either
	// the unified allowed-departure-modes field OR the legacy student.departure
	// field. Mirroring childDepartureAllowsAccompanied (the validation path) is
	// what keeps an accepted submission approvable: validateAccompaniedCompanionNote
	// accepts a legacy departure submission as long as the note is present, so
	// dropping it here for that target would let approval decode an accompanied
	// departure with no note and have studentRepo.Update reject it. A
	// stale/crafted note for a child with no "Mit anderem Kind" day must still
	// not survive. The decision service is the authoritative guard, so on a
	// decode error we keep the note and let it decide.
	if note, ok := values[enrollmentModels.TargetStudentDepartureCompanionNote]; ok {
		keepNote := false
		for i := range schema.Fields {
			f := &schema.Fields[i]
			if f.AppliesToCh != appliesToChild || !fieldVisible(f, ctx) {
				continue
			}
			switch f.Target {
			case enrollmentModels.TargetStudentAllowedDepartureModes:
				if modes, err := decodeAllowedDepartureModes(values[f.Key]); err != nil ||
					modes.HasMode(users.DepartureAccompanied) {
					keepNote = true
				}
			case enrollmentModels.TargetStudentDeparture:
				if days, err := decodeDepartureDays(values[f.Key]); err != nil ||
					days.HasMode(users.DepartureAccompanied) {
					keepNote = true
				}
			}
		}
		if keepNote {
			// Bound the free-text note before it is persisted and echoed in admin
			// review/export. The custom_data value (and the student column) is
			// unbounded TEXT, so a client that bypasses the React maxLength could
			// otherwise store an arbitrarily large note here — a storage/echo
			// surface that survives until approval. Cap it at the same rune limit
			// the approval path applies (truncateRunes), so the stored request and
			// the approved student row carry the same value. Normalizing to a
			// string also prevents a non-string blob from riding the reserved key.
			out[enrollmentModels.TargetStudentDepartureCompanionNote] =
				strutil.TruncateRunes(stringValue(note), users.MaxDepartureCompanionNoteLen, "")
		}
	}
	return out
}

// mergeEditableCustomData keeps legacy/custom keys that are not editable by the
// current schema while letting sanitized submitted answers replace the editable
// field set. This prevents full edit/change-request snapshots from wiping data
// that the reopened form cannot render, without preserving stale answers for
// hidden schema fields.
func mergeEditableCustomData(
	existing map[string]any,
	submitted map[string]any,
	schema *enrollmentModels.FormSchema,
	appliesToChild bool,
) map[string]any {
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

func existingChildCustomDataBySubmittedIdentity(existing []*enrollmentModels.RequestChild, submitted []SubmitChild) []map[string]any {
	out := make([]map[string]any, len(submitted))
	for i, child := range matchExistingChildrenBySubmittedIdentity(existing, submitted) {
		if child != nil {
			out[i] = child.CustomData
		}
	}
	return out
}

// matchExistingChildrenBySubmittedIdentity returns a one-to-one carryover map
// for replacement edits. A persisted ID is authoritative. ID-less rows are
// matched only when the normalized (name, date-of-birth) identity is unique on
// both sides; ambiguous or genuinely new children stay unmatched. This avoids
// silently transferring hidden offerings or lifecycle metadata by array index.
func matchExistingChildrenBySubmittedIdentity(existing []*enrollmentModels.RequestChild, submitted []SubmitChild) []*enrollmentModels.RequestChild {
	matches := make([]*enrollmentModels.RequestChild, len(submitted))
	byID := make(map[int64]*enrollmentModels.RequestChild, len(existing))
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

	existingByIdentity := make(map[string][]*enrollmentModels.RequestChild)
	incomingByIdentity := make(map[string][]int)
	for _, child := range existing {
		if child == nil || child.ID <= 0 || used[child.ID] {
			continue
		}
		key := requestChildIdentityKey(child.FirstName, child.LastName, child.DateOfBirth.String())
		existingByIdentity[key] = append(existingByIdentity[key], child)
	}
	for i, incoming := range submitted {
		if matches[i] != nil || incoming.ID > 0 {
			continue
		}
		key := requestChildIdentityKey(incoming.FirstName, incoming.LastName, incoming.DateOfBirth.String())
		incomingByIdentity[key] = append(incomingByIdentity[key], i)
	}
	for key, indexes := range incomingByIdentity {
		candidates := existingByIdentity[key]
		if len(indexes) != 1 || len(candidates) != 1 {
			continue
		}
		matches[indexes[0]] = candidates[0]
	}
	return matches
}

func requestChildIdentityKey(firstName, lastName, dateOfBirth string) string {
	return strings.ToLower(strings.TrimSpace(firstName)) + "\x00" +
		strings.ToLower(strings.TrimSpace(lastName)) + "\x00" + strings.TrimSpace(dateOfBirth)
}

func editableCustomDataKeys(schema *enrollmentModels.FormSchema, appliesToChild bool) map[string]bool {
	keys := make(map[string]bool)
	if schema == nil {
		return keys
	}
	for i := range schema.Fields {
		field := &schema.Fields[i]
		if field.AppliesToCh != appliesToChild || field.Type == enrollmentModels.FormFieldInfo {
			continue
		}
		keys[field.Key] = true
		if appliesToChild {
			switch field.Target {
			case enrollmentModels.TargetStudentAllowedDepartureModes, enrollmentModels.TargetStudentDeparture:
				keys[enrollmentModels.TargetStudentDepartureCompanionNote] = true
			}
		}
	}
	return keys
}

// validateRequiredCustomFields is the single required-field gate for the
// public submit path. It enforces required core (built-in) fields and required
// custom fields, skipping any field hidden by a visibility condition and
// information blocks. Mirrors the client-side check in enrollment-form.tsx so a
// stale or scripted client can't bypass it — and so a field hidden by its
// show-if condition never blocks an otherwise valid submit.
func (s *requestService) validateRequiredCustomFields(
	schema *enrollmentModels.FormSchema,
	req SubmitRequest,
	openByID map[int64]*enrollmentModels.CareOffering,
) error {
	if schema == nil {
		return nil
	}

	// Core (built-in) required fields are rendered from dedicated request
	// columns rather than schema.Fields, so they are enforced here from the
	// schema's core_requirements.
	if schema.CoreRequirements.Required(enrollmentModels.CoreRequirementGuardianPhone) {
		if req.GuardianPhone == nil || strings.TrimSpace(*req.GuardianPhone) == "" {
			return fmt.Errorf("%w: guardian phone is required", ErrInvalidSubmission)
		}
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
		if !customAnswerSatisfiesRequired(*f, req.CustomData) {
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
			if f.Type == enrollmentModels.FormFieldWeekdayMultiMode {
				// Care-day scoped exactly like weekday_schedule: a no-offerings
				// phase renders all weekdays, so the required answer must cover
				// them all. selectedCareDays alone would be empty there and
				// wrongly exempt the field, letting a scripted client skip it.
				if !customAnswerSatisfiesRequiredWeekdayMultiMode(*f, child.CustomData, relevantCareDaysForChild(child, openByID)) {
					return fmt.Errorf("%w: child %d field %q is required", ErrInvalidSubmission, idx, f.Key)
				}
				continue
			}
			if f.Type == enrollmentModels.FormFieldWeekdaySchedule {
				if !customAnswerSatisfiesRequiredWeekdaySchedule(*f, child.CustomData, relevantCareDaysForChild(child, openByID), len(openByID) > 0) {
					return fmt.Errorf("%w: child %d field %q is required", ErrInvalidSubmission, idx, f.Key)
				}
				continue
			}
			if !customAnswerSatisfiesRequired(*f, child.CustomData) {
				return fmt.Errorf("%w: child %d field %q is required", ErrInvalidSubmission, idx, f.Key)
			}
		}
	}
	return nil
}

// validateAccompaniedCompanionNote enforces the coupled "mit wem" note at the
// public submit boundary: a child whose visible departure field actually allows
// the accompanied ("Mit anderem Kind") mode must carry a non-blank companion
// note on the reserved custom-data key. Without this gate a stale or scripted
// client could submit an accompanied plan with no note, which persists fine but
// then permanently blocks approval — studentRepo.Update rejects the model
// invariant. Mirrors the same defense-in-depth other submit-time field checks
// apply so a submittable request can never get stuck at approval (#1694).
func (s *requestService) validateAccompaniedCompanionNote(
	schema *enrollmentModels.FormSchema,
	req SubmitRequest,
	openByID map[int64]*enrollmentModels.CareOffering,
) error {
	if schema == nil {
		return nil
	}
	byKey := buildFieldsByKey(schema)
	for idx := range req.Children {
		child := req.Children[idx]
		childCtx := fieldVisibilityContext{
			guardianAnswers: req.CustomData,
			childAnswers:    child.CustomData,
			gradeLevel:      child.TargetGradeLevel,
			offeringNames:   selectedOfferingNames(child, openByID),
			fieldsByKey:     byKey,
		}
		if !childDepartureAllowsAccompanied(schema, child, childCtx) {
			continue
		}
		if strings.TrimSpace(stringValue(child.CustomData[enrollmentModels.TargetStudentDepartureCompanionNote])) == "" {
			return fmt.Errorf("%w: child %d accompanied departure requires a companion note", ErrInvalidSubmission, idx)
		}
	}
	return nil
}

// childDepartureAllowsAccompanied reports whether any visible per-child
// departure field (unified allowed modes or the legacy-exclusive departure
// field) submits a plan that allows the accompanied mode. On a decode error the
// field is treated as not-accompanied: the decision service is the authoritative
// guard and rejects a malformed value there.
func childDepartureAllowsAccompanied(
	schema *enrollmentModels.FormSchema,
	child SubmitChild,
	ctx fieldVisibilityContext,
) bool {
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if !f.AppliesToCh || !fieldVisible(f, ctx) {
			continue
		}
		switch f.Target {
		case enrollmentModels.TargetStudentAllowedDepartureModes:
			if modes, err := decodeAllowedDepartureModes(child.CustomData[f.Key]); err == nil &&
				modes.HasMode(users.DepartureAccompanied) {
				return true
			}
		case enrollmentModels.TargetStudentDeparture:
			if days, err := decodeDepartureDays(child.CustomData[f.Key]); err == nil &&
				days.HasMode(users.DepartureAccompanied) {
				return true
			}
		}
	}
	return false
}

// validateConstrainedSchedules enforces a weekday_schedule field's fixed
// pickup times (FormField.AllowedTimes) server-side. The public form renders a
// dropdown limited to these times, but a scripted or stale client could POST
// an off-list time, so every *visible* schedule answer is re-checked here.
// Empty AllowedTimes means "no restriction" (free time entry) and such fields
// are skipped. Hidden fields are skipped to match validateRequiredCustomFields
// — their answers are dropped before persistence anyway, so rejecting them
// would block an otherwise valid submit. For the same reason a per-child
// schedule is only checked on the child's schedulable weekdays
// (relevantCareDaysForChild): an off-list time on a non-care day is stripped by
// pruneChildScheduleAnswers before persistence, so the gate must not reject it
// and block the submit.
func (s *requestService) validateConstrainedSchedules(
	schema *enrollmentModels.FormSchema,
	req SubmitRequest,
	openByID map[int64]*enrollmentModels.CareOffering,
) error {
	if schema == nil {
		return nil
	}
	byKey := buildFieldsByKey(schema)

	// scheduleDays scopes which weekdays are inspected. A nil set means "all
	// days" (guardian-level fields, which are not care-day scoped); a per-child
	// field passes its schedulable days so non-care-day entries (pruned before
	// persistence) can't trip the allowed-times gate.
	check := func(f *enrollmentModels.FormField, answers map[string]any, childIdx int, scheduleDays map[string]bool) error {
		raw, ok := answers[f.Key]
		if !ok || raw == nil {
			return nil
		}
		var sched enrollmentModels.WeekdaySchedule
		if err := decodeStructured(raw, &sched); err != nil {
			if childIdx >= 0 {
				return fmt.Errorf("%w: child %d field %q: invalid schedule", ErrInvalidSubmission, childIdx, f.Key)
			}
			return fmt.Errorf("%w: field %q: invalid schedule", ErrInvalidSubmission, f.Key)
		}
		if scheduleDays != nil {
			scoped := make(enrollmentModels.WeekdaySchedule, len(sched))
			for day, t := range sched {
				if scheduleDays[day] {
					scoped[day] = t
				}
			}
			sched = scoped
		}
		if err := sched.ValidateAllowed(f.AllowedTimes); err != nil {
			if childIdx >= 0 {
				return fmt.Errorf("%w: child %d field %q: %v", ErrPickupTimeNotAllowed, childIdx, f.Key, err)
			}
			return fmt.Errorf("%w: field %q: %v", ErrPickupTimeNotAllowed, f.Key, err)
		}
		return nil
	}

	guardianCtx := fieldVisibilityContext{guardianAnswers: req.CustomData, fieldsByKey: byKey}
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if f.Target != enrollmentModels.TargetSchedulePickup || len(f.AllowedTimes) == 0 || f.AppliesToCh {
			continue
		}
		if !fieldVisible(f, guardianCtx) {
			continue
		}
		if err := check(f, req.CustomData, -1, nil); err != nil {
			return err
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
		scheduleDays := relevantCareDaysForChild(child, openByID)
		for i := range schema.Fields {
			f := &schema.Fields[i]
			if f.Target != enrollmentModels.TargetSchedulePickup || len(f.AllowedTimes) == 0 || !f.AppliesToCh {
				continue
			}
			if !fieldVisible(f, childCtx) {
				continue
			}
			if err := check(f, child.CustomData, idx, scheduleDays); err != nil {
				return err
			}
		}
	}
	return nil
}

// customAnswerSatisfiesRequired reports whether a required field is answered,
// pulling the value out of the answer map so it can distinguish a *missing*
// key from a present-but-empty value. This matters only for the pickup target:
// an explicit empty weekday map is the valid "Geht alleine nach Hause" answer,
// but a field the parent never touched (no key in custom_data) is unanswered
// and must fail a required check. Every other field type derives "missing"
// from the value shape alone, so the value lookup is sufficient for them.
func customAnswerSatisfiesRequired(field enrollmentModels.FormField, answers map[string]any) bool {
	value, present := answers[field.Key]
	// The pickup target and the unified departure field accept an empty
	// selection ("geht alleine") as a valid answer, so a required one must still
	// reject a never-touched field (no key in custom_data).
	pickupBoolean := field.Type == enrollmentModels.FormFieldWeekdayBoolean &&
		field.Target == enrollmentModels.TargetStudentPickupStatus
	if (pickupBoolean || field.Type == enrollmentModels.FormFieldWeekdayMode) && !present {
		return false
	}
	if field.Type == enrollmentModels.FormFieldWeekdayMultiMode && !present {
		return false
	}
	return customValueSatisfiesRequired(field, value)
}

func customAnswerSatisfiesRequiredWeekdayMultiMode(field enrollmentModels.FormField, answers map[string]any, careDays map[string]bool) bool {
	if len(careDays) == 0 {
		return true
	}
	value, present := answers[field.Key]
	if !present {
		return false
	}
	var modes enrollmentModels.WeekdayMultiMode
	if err := decodeStructured(value, &modes); err != nil {
		return false
	}
	if err := modes.Validate(); err != nil {
		return false
	}
	for day := range modes {
		if !careDays[day] {
			return false
		}
	}
	for day := range careDays {
		if len(modes[day]) == 0 {
			return false
		}
	}
	return true
}

// relevantCareDaysForChild returns the weekdays every per-child day-scoped
// field (weekday_schedule AND weekday_multi_mode) offers for this child. It is
// the single server-side mirror of relevantCareDaysForChild in
// enrollment-form.tsx, so all care-day-scoped paths share one rule and can't
// drift: with no care offerings configured the field is unconstrained (all
// weekdays), otherwise it is limited to the child's selected care days (empty
// when the child picked none -- the form hides the input). The result is
// read-only; callers must not mutate it (the no-offerings case returns the
// shared ValidWeekdays map).
func relevantCareDaysForChild(child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) map[string]bool {
	if len(openByID) == 0 {
		return enrollmentModels.ValidWeekdays
	}
	return selectedCareDays(child, openByID)
}

// customAnswerSatisfiesRequiredWeekdaySchedule mirrors the public form's
// care-day-scoped rendering of a per-child weekday_schedule field. scheduleDays
// is relevantCareDaysForChild: all weekdays when no offerings constrain the
// form (the field is shown and must be answered), the child's care days when
// offerings exist, or empty when offerings exist but the child selected none
// (the form hides the input, so a required schedule is vacuously satisfied).
//
// careConstrained reports whether scheduleDays come from selected care
// offerings (openByID non-empty), exactly like careDaysAreConstrained on the
// frontend. When constrained, the shown days ARE the child's care days, so each
// one must carry a time (an empty day is a missing answer, not a "keine
// Angabe"). When unconstrained (no offerings, all weekdays shown), at least one
// schedulable day must carry a time -- forcing every weekday would block a
// child who attends only some days. Both branches stay in lockstep with the
// client's customValueMissing so a stale or scripted client cannot persist an
// enrollment that is missing pickup times for required care days.
func customAnswerSatisfiesRequiredWeekdaySchedule(field enrollmentModels.FormField, answers map[string]any, scheduleDays map[string]bool, careConstrained bool) bool {
	if len(scheduleDays) == 0 {
		return true
	}
	value, present := answers[field.Key]
	if !present {
		return false
	}
	var sched enrollmentModels.WeekdaySchedule
	if err := decodeStructured(value, &sched); err != nil {
		return false
	}
	if careConstrained {
		for day := range scheduleDays {
			if strings.TrimSpace(sched[day]) == "" {
				return false
			}
		}
		return true
	}
	for day, t := range sched {
		if scheduleDays[day] && strings.TrimSpace(t) != "" {
			return true
		}
	}
	return false
}

// pruneChildScheduleAnswers strips per-child weekday_schedule entries for
// weekdays the child cannot schedule (non-care days, or weekend days a scripted
// client smuggled in), keeping the schedulable days as-is. It is the
// persistence-time counterpart of the public form's pruneWeekdayAnswers: the
// decision service turns every persisted weekday into a pickup/arrival schedule
// row, so an unpruned {"fri":"15:00"} for a Tue/Thu child would create a Friday
// pickup the parent never had access to. Mutates answers in place.
func pruneChildScheduleAnswers(schema *enrollmentModels.FormSchema, answers map[string]any, scheduleDays map[string]bool) {
	if schema == nil || answers == nil {
		return
	}
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if !f.AppliesToCh || f.Type != enrollmentModels.FormFieldWeekdaySchedule {
			continue
		}
		raw, ok := answers[f.Key]
		if !ok || raw == nil {
			continue
		}
		var sched enrollmentModels.WeekdaySchedule
		if err := decodeStructured(raw, &sched); err != nil {
			continue
		}
		pruned := make(map[string]any, len(sched))
		for day, t := range sched {
			if scheduleDays[day] {
				pruned[day] = t
			}
		}
		answers[f.Key] = pruned
	}
}

// customValueSatisfiesRequired reports whether a required field's answer is
// present AND well-formed. Unlike a plain "non-empty" check, structured fields
// are validated entry-by-entry (PhoneEntry/ContactEntry.Validate, schedule has
// a time) so a stale or scripted client can't satisfy a required phone_list
// with [{}] or a contact_list with nameless/contactless rows — which would
// otherwise pass submit and only fail later at approval/dispatch.
func customValueSatisfiesRequired(field enrollmentModels.FormField, value any) bool {
	switch field.Type {
	case enrollmentModels.FormFieldBoolean:
		_, ok := value.(bool)
		return ok
	case enrollmentModels.FormFieldPhoneList:
		return phoneListSatisfiesRequired(value)
	case enrollmentModels.FormFieldContactList:
		return contactListSatisfiesRequired(value)
	case enrollmentModels.FormFieldWeekdaySchedule:
		return scheduleHasAnyTime(value)
	case enrollmentModels.FormFieldWeekdayBoolean:
		// For the pickup target an empty map is the valid "Geht alleine nach
		// Hause" answer — the parent consciously selected no pickup days — so a
		// required Abholregelung is satisfied as long as the value is a
		// well-formed weekday map. Every other weekday_boolean field (e.g.
		// Buskind) still needs at least one selected day to count as answered.
		if field.Target == enrollmentModels.TargetStudentPickupStatus {
			return weekdayBooleanWellFormed(value)
		}
		return weekdayBooleanHasAnySelected(value)
	case enrollmentModels.FormFieldWeekdayMode:
		// An all-alone (empty) plan is a valid answer, so a well-formed map
		// satisfies a required Geh-/Abholregelung (#1610).
		return weekdayModeWellFormed(value)
	case enrollmentModels.FormFieldWeekdayMultiMode:
		return weekdayMultiModeWellFormed(value)
	case enrollmentModels.FormFieldNumber:
		return numberValueSatisfiesRequired(value)
	default:
		return stringValue(value) != ""
	}
}

func weekdayBooleanHasAnySelected(value any) bool {
	var days enrollmentModels.WeekdayBoolean
	if err := decodeStructured(value, &days); err != nil {
		return false
	}
	if err := days.Validate(); err != nil {
		return false
	}
	return days.HasAny()
}

// weekdayBooleanWellFormed reports whether the value decodes to a valid
// weekday map, without requiring any day to be selected. Used for the pickup
// target, where an empty map ("Geht alleine nach Hause") is a legitimate
// answer — a nil/absent value decodes to an empty map and is accepted too.
func weekdayBooleanWellFormed(value any) bool {
	var days enrollmentModels.WeekdayBoolean
	if err := decodeStructured(value, &days); err != nil {
		return false
	}
	return days.Validate() == nil
}

// weekdayModeWellFormed reports whether the value decodes to a valid per-day
// departure map, without requiring any non-alone day. Used for the unified
// departure target, where an all-alone (empty) plan is a legitimate answer.
func weekdayModeWellFormed(value any) bool {
	var modes enrollmentModels.WeekdayMode
	if err := decodeStructured(value, &modes); err != nil {
		return false
	}
	return modes.Validate() == nil
}

func weekdayMultiModeWellFormed(value any) bool {
	var modes enrollmentModels.WeekdayMultiMode
	if err := decodeStructured(value, &modes); err != nil {
		return false
	}
	return modes.Validate() == nil
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
			if enrollmentModels.ValidWeekdays[day] {
				days[day] = true
			}
		}
	}
	return days
}

func phoneListSatisfiesRequired(value any) bool {
	var entries []enrollmentModels.PhoneEntry
	if err := decodeStructured(value, &entries); err != nil || len(entries) == 0 {
		return false
	}
	for i := range entries {
		if err := entries[i].Validate(); err != nil {
			return false
		}
	}
	return true
}

func contactListSatisfiesRequired(value any) bool {
	var entries []enrollmentModels.ContactEntry
	if err := decodeStructured(value, &entries); err != nil || len(entries) == 0 {
		return false
	}
	for i := range entries {
		if err := entries[i].Validate(); err != nil {
			return false
		}
	}
	return true
}

func numberValueSatisfiesRequired(value any) bool {
	switch v := value.(type) {
	case float64, int:
		return true
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return false
	}
}

func scheduleHasAnyTime(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, v := range typed {
			if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
				return true
			}
		}
	case map[string]string:
		for _, v := range typed {
			if strings.TrimSpace(v) != "" {
				return true
			}
		}
	}
	return false
}
