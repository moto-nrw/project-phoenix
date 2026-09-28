package application

import (
	"fmt"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// The submission-time field checks behind the public form: required fields,
// the "mit wem" note of an accompanied departure, fixed pickup times and the
// single departure mode of restricted grades. They mirror the client checks
// in enrollment-form.tsx, so a stale or scripted client cannot bypass them.

// validateRequiredCustomFields enforces the required core and custom fields,
// skipping fields hidden by their condition and information blocks.
func validateRequiredCustomFields(schema *enrollment.FormSchema, req SubmitRequest, openByID map[int64]*enrollmentModels.CareOffering) error {
	if schema == nil {
		return nil
	}
	// Core fields render from request columns, so their requirement comes
	// from the schema's core_requirements.
	if schema.CoreRequirements.Required(enrollment.CoreRequirementGuardianPhone) {
		if req.GuardianPhone == nil || strings.TrimSpace(*req.GuardianPhone) == "" {
			return fmt.Errorf("%w: guardian phone is required", enrollment.ErrInvalidSubmission)
		}
	}
	byKey := buildFieldsByKey(schema)
	guardianCtx := fieldVisibilityContext{guardianAnswers: req.CustomData, fieldsByKey: byKey}
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if f.AppliesToCh || f.Type == enrollment.FormFieldInfo || !f.Required || !fieldVisible(f, guardianCtx) {
			continue
		}
		if !customAnswerSatisfiesRequired(*f, req.CustomData) {
			return fmt.Errorf("%w: field %q is required", enrollment.ErrInvalidSubmission, f.Key)
		}
	}
	for idx := range req.Children {
		if err := validateRequiredChildFields(schema, req.CustomData, idx, req.Children[idx], openByID, byKey); err != nil {
			return err
		}
	}
	return nil
}

func validateRequiredChildFields(schema *enrollment.FormSchema, guardianAnswers map[string]any, idx int, child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering, byKey map[string]*enrollment.FormField) error {
	childCtx := childVisibilityContext(guardianAnswers, child, openByID, byKey)
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if !f.AppliesToCh || f.Type == enrollment.FormFieldInfo || !f.Required || !fieldVisible(f, childCtx) {
			continue
		}
		if !childAnswerSatisfiesRequired(*f, child, openByID) {
			return fmt.Errorf("%w: child %d field %q is required", enrollment.ErrInvalidSubmission, idx, f.Key)
		}
	}
	return nil
}

// childAnswerSatisfiesRequired checks one required per-child field. The
// day-scoped fields are scoped to the child's care days exactly like the form
// renders them; a no-offerings phase renders every weekday.
func childAnswerSatisfiesRequired(f enrollment.FormField, child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) bool {
	switch f.Type {
	case enrollment.FormFieldWeekdayMultiMode:
		return customAnswerSatisfiesRequiredWeekdayMultiMode(f, child.CustomData, relevantCareDaysForChild(child, openByID))
	case enrollment.FormFieldWeekdaySchedule:
		return customAnswerSatisfiesRequiredWeekdaySchedule(f, child.CustomData, relevantCareDaysForChild(child, openByID), len(openByID) > 0)
	default:
		return customAnswerSatisfiesRequired(f, child.CustomData)
	}
}

// validateAccompaniedCompanionNote requires the "mit wem" note for a child
// whose visible departure field allows the accompanied mode. Without it the
// request would store fine and then block its approval (#1694).
func validateAccompaniedCompanionNote(schema *enrollment.FormSchema, req SubmitRequest, openByID map[int64]*enrollmentModels.CareOffering) error {
	if schema == nil {
		return nil
	}
	byKey := buildFieldsByKey(schema)
	for idx := range req.Children {
		child := req.Children[idx]
		if !childDepartureAllowsAccompanied(schema, child, childVisibilityContext(req.CustomData, child, openByID, byKey)) {
			continue
		}
		if strings.TrimSpace(stringValue(child.CustomData[enrollment.TargetStudentDepartureCompanionNote])) == "" {
			return fmt.Errorf("%w: child %d accompanied departure requires a companion note", enrollment.ErrInvalidSubmission, idx)
		}
	}
	return nil
}

// childDepartureAllowsAccompanied reports whether a visible per-child
// departure field allows the accompanied mode. A decode error counts as not
// accompanied; the approval rejects a malformed value.
func childDepartureAllowsAccompanied(schema *enrollment.FormSchema, child SubmitChild, ctx fieldVisibilityContext) bool {
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if !f.AppliesToCh || !fieldVisible(f, ctx) {
			continue
		}
		switch f.Target {
		case enrollment.TargetStudentAllowedDepartureModes:
			if modes, err := decodeAllowedDepartureModes(child.CustomData[f.Key]); err == nil && modes.HasMode(departure.DepartureAccompanied) {
				return true
			}
		case enrollment.TargetStudentDeparture:
			if days, err := decodeDepartureDays(child.CustomData[f.Key]); err == nil && days.HasMode(departure.DepartureAccompanied) {
				return true
			}
		}
	}
	return false
}

// validateConstrainedSchedules enforces the fixed pickup times of visible
// weekday_schedule fields and the single departure mode of restricted grades
// (#2381). A per-child schedule is checked only on the child's schedulable
// days, because the other entries are pruned before persistence.
// existingChildren, when given, lets an unchanged answer accepted before a
// single-mode rule stand.
func validateConstrainedSchedules(schema *enrollment.FormSchema, req SubmitRequest, openByID map[int64]*enrollmentModels.CareOffering, existingChildren ...[]*RequestChild) error {
	if schema == nil {
		return nil
	}
	byKey := buildFieldsByKey(schema)
	var existingBySubmittedChild []*RequestChild
	if len(existingChildren) > 0 {
		existingBySubmittedChild = matchExistingChildrenBySubmittedIdentity(existingChildren[0], req.Children)
	}
	guardianCtx := fieldVisibilityContext{guardianAnswers: req.CustomData, fieldsByKey: byKey}
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if f.Target != enrollment.TargetSchedulePickup || len(f.AllowedTimes) == 0 || f.AppliesToCh || !fieldVisible(f, guardianCtx) {
			continue
		}
		if err := checkAllowedPickupTimes(f, req.CustomData, -1, nil); err != nil {
			return err
		}
	}
	for idx := range req.Children {
		var existing *RequestChild
		if idx < len(existingBySubmittedChild) {
			existing = existingBySubmittedChild[idx]
		}
		if err := validateChildConstrainedSchedules(schema, req.CustomData, idx, req.Children[idx], existing, openByID, byKey); err != nil {
			return err
		}
	}
	return nil
}

func validateChildConstrainedSchedules(schema *enrollment.FormSchema, guardianAnswers map[string]any, idx int, child SubmitChild, existing *RequestChild, openByID map[int64]*enrollmentModels.CareOffering, byKey map[string]*enrollment.FormField) error {
	childCtx := childVisibilityContext(guardianAnswers, child, openByID, byKey)
	scheduleDays := relevantCareDaysForChild(child, openByID)
	for i := range schema.Fields {
		f := &schema.Fields[i]
		if !f.AppliesToCh || !fieldVisible(f, childCtx) {
			continue
		}
		if f.Target == enrollment.TargetSchedulePickup && len(f.AllowedTimes) > 0 {
			if err := checkAllowedPickupTimes(f, child.CustomData, idx, scheduleDays); err != nil {
				return err
			}
		}
		if f.Target != enrollment.TargetStudentAllowedDepartureModes || !f.SingleModeAppliesTo(child.TargetGradeLevel) ||
			unchangedSingleModeDepartureAnswer(existing, child, f.Key) {
			continue
		}
		if err := checkSingleModeDeparture(f, child.CustomData, idx); err != nil {
			return err
		}
	}
	return nil
}

// checkAllowedPickupTimes checks one schedule answer against the field's
// fixed times. scheduleDays scopes the checked weekdays; nil checks all.
func checkAllowedPickupTimes(f *enrollment.FormField, answers map[string]any, childIdx int, scheduleDays map[string]bool) error {
	raw, ok := answers[f.Key]
	if !ok || raw == nil {
		return nil
	}
	var sched enrollment.WeekdaySchedule
	if err := decodeStructured(raw, &sched); err != nil {
		if childIdx >= 0 {
			return fmt.Errorf("%w: child %d field %q: invalid schedule", enrollment.ErrInvalidSubmission, childIdx, f.Key)
		}
		return fmt.Errorf("%w: field %q: invalid schedule", enrollment.ErrInvalidSubmission, f.Key)
	}
	if scheduleDays != nil {
		scoped := make(enrollment.WeekdaySchedule, len(sched))
		for day, t := range sched {
			if scheduleDays[day] {
				scoped[day] = t
			}
		}
		sched = scoped
	}
	if err := sched.ValidateAllowed(f.AllowedTimes); err != nil {
		if childIdx >= 0 {
			return fmt.Errorf("%w: child %d field %q: %v", enrollment.ErrPickupTimeNotAllowed, childIdx, f.Key, err)
		}
		return fmt.Errorf("%w: field %q: %v", enrollment.ErrPickupTimeNotAllowed, f.Key, err)
	}
	return nil
}

// unchangedSingleModeDepartureAnswer preserves a multi-select answer accepted
// before a single-mode rule was configured. A changed grade still
// revalidates, because it can newly put the child under the rule.
func unchangedSingleModeDepartureAnswer(existing *RequestChild, submitted SubmitChild, key string) bool {
	if existing == nil || !sameGradeLevel(existing.TargetGradeLevel, submitted.TargetGradeLevel) {
		return false
	}
	return jsonEqual(existing.CustomData[key], submitted.CustomData[key])
}

func sameGradeLevel(left, right *int16) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

// checkSingleModeDeparture enforces at most one departure mode per weekday
// (#2381) on every submitted day; the answer is not care-day-pruned.
func checkSingleModeDeparture(f *enrollment.FormField, answers map[string]any, childIdx int) error {
	raw, ok := answers[f.Key]
	if !ok || raw == nil {
		return nil
	}
	var modes enrollment.WeekdayMultiMode
	if err := decodeStructured(raw, &modes); err != nil {
		return fmt.Errorf("%w: child %d field %q: invalid departure modes", enrollment.ErrInvalidSubmission, childIdx, f.Key)
	}
	if err := modes.ValidateSingleSelection(); err != nil {
		return fmt.Errorf("%w: child %d field %q: %v", enrollment.ErrDepartureModeLimitExceeded, childIdx, f.Key, err)
	}
	return nil
}

// customAnswerSatisfiesRequired reports whether a required field is
// answered. The pickup target and the departure fields accept an empty
// selection ("geht alleine"), so a never-touched field — no key at all —
// must still fail.
func customAnswerSatisfiesRequired(field enrollment.FormField, answers map[string]any) bool {
	value, present := answers[field.Key]
	pickupBoolean := field.Type == enrollment.FormFieldWeekdayBoolean && field.Target == enrollment.TargetStudentPickupStatus
	if (pickupBoolean || field.Type == enrollment.FormFieldWeekdayMode || field.Type == enrollment.FormFieldWeekdayMultiMode) && !present {
		return false
	}
	return customValueSatisfiesRequired(field, value)
}

func customAnswerSatisfiesRequiredWeekdayMultiMode(field enrollment.FormField, answers map[string]any, careDays map[string]bool) bool {
	if len(careDays) == 0 {
		return true
	}
	value, present := answers[field.Key]
	if !present {
		return false
	}
	var modes enrollment.WeekdayMultiMode
	if err := decodeStructured(value, &modes); err != nil || modes.Validate() != nil {
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

// customAnswerSatisfiesRequiredWeekdaySchedule mirrors the form's care-day
// scoped schedule: with care offerings every shown care day needs a time,
// without them at least one weekday does; a child without care days has no
// input to fill.
func customAnswerSatisfiesRequiredWeekdaySchedule(field enrollment.FormField, answers map[string]any, scheduleDays map[string]bool, careConstrained bool) bool {
	if len(scheduleDays) == 0 {
		return true
	}
	value, present := answers[field.Key]
	if !present {
		return false
	}
	var sched enrollment.WeekdaySchedule
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

// customValueSatisfiesRequired reports whether a required answer is present
// and well formed; structured entries are validated one by one, so [{}] never
// satisfies a required phone list.
func customValueSatisfiesRequired(field enrollment.FormField, value any) bool {
	switch field.Type {
	case enrollment.FormFieldBoolean:
		_, ok := value.(bool)
		return ok
	case enrollment.FormFieldPhoneList:
		return phoneListSatisfiesRequired(value)
	case enrollment.FormFieldContactList:
		return contactListSatisfiesRequired(value)
	case enrollment.FormFieldWeekdaySchedule:
		return scheduleHasAnyTime(value)
	case enrollment.FormFieldWeekdayBoolean:
		// An empty map is the valid "Geht alleine nach Hause" answer of the
		// pickup target; every other weekday_boolean needs a selected day.
		if field.Target == enrollment.TargetStudentPickupStatus {
			return weekdayBooleanWellFormed(value)
		}
		return weekdayBooleanHasAnySelected(value)
	case enrollment.FormFieldWeekdayMode:
		// An all-alone (empty) plan is a valid answer (#1610).
		return weekdayModeWellFormed(value)
	case enrollment.FormFieldWeekdayMultiMode:
		return weekdayMultiModeWellFormed(value)
	case enrollment.FormFieldNumber:
		return numberValueSatisfiesRequired(value)
	default:
		return stringValue(value) != ""
	}
}

func weekdayBooleanHasAnySelected(value any) bool {
	var days enrollment.WeekdayBoolean
	if err := decodeStructured(value, &days); err != nil || days.Validate() != nil {
		return false
	}
	return days.HasAny()
}

func weekdayBooleanWellFormed(value any) bool {
	var days enrollment.WeekdayBoolean
	if err := decodeStructured(value, &days); err != nil {
		return false
	}
	return days.Validate() == nil
}

func weekdayModeWellFormed(value any) bool {
	var modes enrollment.WeekdayMode
	if err := decodeStructured(value, &modes); err != nil {
		return false
	}
	return modes.Validate() == nil
}

func weekdayMultiModeWellFormed(value any) bool {
	var modes enrollment.WeekdayMultiMode
	if err := decodeStructured(value, &modes); err != nil {
		return false
	}
	return modes.Validate() == nil
}

func phoneListSatisfiesRequired(value any) bool {
	var entries []enrollment.PhoneEntry
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
	var entries []enrollment.ContactEntry
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
