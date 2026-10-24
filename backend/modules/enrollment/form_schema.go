package enrollment

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// validFormFieldTypes is the set of accepted FormFieldType values.
var validFormFieldTypes = map[FormFieldType]bool{
	FormFieldBoolean:          true,
	FormFieldNumber:           true,
	FormFieldText:             true,
	FormFieldTextarea:         true,
	FormFieldDate:             true,
	FormFieldSelect:           true,
	FormFieldInfo:             true,
	FormFieldPhoneList:        true,
	FormFieldWeekdaySchedule:  true,
	FormFieldWeekdayBoolean:   true,
	FormFieldWeekdayMode:      true,
	FormFieldWeekdayMultiMode: true,
	FormFieldContactList:      true,
	FormFieldFile:             true,
}

// Validate checks a condition's shape in isolation. Cross-field
// reference + scope checks (does the controlling field exist, is it a
// boolean/select, same scope) happen in FormSchema.Validate, which can
// see sibling fields. appliesToChild is the owning field's scope; the
// grade_level / care_offering sources are per-child only.
func (c *VisibilityCondition) Validate(appliesToChild bool) error {
	if err := c.validateOperatorValue(); err != nil {
		return err
	}
	return c.validateSource(appliesToChild)
}

// validateOperatorValue checks the operator is known and that a value-taking
// operator carries a scalar value.
func (c *VisibilityCondition) validateOperatorValue() error {
	switch c.Operator {
	case ConditionOpEquals, ConditionOpNotEquals, ConditionOpNotEmpty, ConditionOpIncludes:
	default:
		return fmt.Errorf("unknown visibility operator %q", c.Operator)
	}

	needsValue := c.Operator != ConditionOpNotEmpty
	if needsValue && (c.Value == nil || c.Value == "") {
		return fmt.Errorf("visibility operator %q requires a value", c.Operator)
	}
	// The condition value must be a scalar. JSON decodes arrays as []any and
	// objects as map[string]any; comparing two such values with == at
	// evaluation time panics (uncomparable types), so reject them here with a
	// clean validation error instead of letting a crafted schema crash submit.
	if needsValue {
		switch c.Value.(type) {
		case string, bool,
			float64, float32,
			int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			// scalar — ok
		default:
			return fmt.Errorf(
				"visibility condition value must be a string, number, or boolean, got %T",
				c.Value,
			)
		}
	}
	return nil
}

// validateSource checks the condition source against the operator and the
// owning field's scope.
func (c *VisibilityCondition) validateSource(appliesToChild bool) error {
	switch c.Source {
	case ConditionSourceField:
		if strings.TrimSpace(c.Field) == "" {
			return errors.New("field visibility condition requires the controlling field key")
		}
		if c.Operator == ConditionOpIncludes {
			return errors.New("the 'includes' operator is only valid for care offering conditions")
		}
	case ConditionSourceGradeLevel:
		if !appliesToChild {
			return errors.New("grade level conditions are only valid on per-child fields")
		}
		if c.Operator == ConditionOpIncludes {
			return errors.New("the 'includes' operator is only valid for care offering conditions")
		}
	case ConditionSourceCareOffering:
		if !appliesToChild {
			return errors.New("care offering conditions are only valid on per-child fields")
		}
		if c.Operator != ConditionOpIncludes {
			return errors.New("care offering conditions must use the 'includes' operator")
		}
	default:
		return fmt.Errorf("unknown visibility source %q", c.Source)
	}
	return nil
}

var validCoreRequirements = map[string]bool{
	CoreRequirementGuardianPhone: true,
}

// Validate rejects unknown core requirement keys so typos in the admin API
// do not silently create requirements the public form cannot enforce.
func (c CoreRequirements) Validate() error {
	for key := range c {
		if !validCoreRequirements[key] {
			return fmt.Errorf("unknown core requirement %q", key)
		}
	}
	return nil
}

// Required reports whether the given core field must be answered.
func (c CoreRequirements) Required(key string) bool {
	if c == nil {
		return false
	}
	return c[key]
}

// ReservedTargets is the canonical list of admin-pickable targets.
//
// Photo consent and the guardian's phone number are intentionally NOT
// in this list: both are already collected by the public base form
// (consent_flags.photo at the AGB block, guardian_phone in the
// guardian section). The decision service stamps them onto the
// downstream records automatically — admins shouldn't add duplicate
// Zusatzfeld entries for fields the parent already sees by default.
//
// Keep in sync with the frontend editor's reserved-targets picker.
var ReservedTargets = map[string]ReservedTarget{
	TargetStudentHealthInfo:            {Type: FormFieldTextarea, AppliesToChild: true, Label: "Gesundheitsinformationen"},
	TargetStudentExtraInfo:             {Type: FormFieldTextarea, AppliesToChild: true, Label: "Hinweise an die Betreuung"},
	TargetStudentAllowedDepartureModes: {Type: FormFieldWeekdayMultiMode, AppliesToChild: true, Label: "Erlaubte Heimwege"},
	TargetStudentDeparture:             {Type: FormFieldWeekdayMode, AppliesToChild: true, Label: "Geh- und Abholregelung"},
	TargetStudentBusDays:               {Type: FormFieldWeekdayBoolean, AppliesToChild: true, Label: "Buskind"},
	TargetStudentBus:                   {Type: FormFieldWeekdayBoolean, AppliesToChild: true, Label: "Buskind"},
	TargetStudentPickupStatus:          {Type: FormFieldWeekdayBoolean, AppliesToChild: true, Label: "Abholregelung"},
	TargetSchedulePickup:               {Type: FormFieldWeekdaySchedule, AppliesToChild: true, Label: "Abholzeiten"},
	TargetScheduleArrival:              {Type: FormFieldWeekdaySchedule, AppliesToChild: true, Label: "Ankunftszeiten"},
	TargetStudentContacts:              {Type: FormFieldContactList, AppliesToChild: true, Label: "Weitere Kontakte / Abholberechtigte / Notfallkontakte"},
}

// CoreFieldKeys are reserved - the form_schemas.fields JSONB MUST NOT
// declare a custom field with one of these keys. The frontend already
// renders these from dedicated columns on enrollment.requests / .request_children.
var CoreFieldKeys = map[string]bool{
	"guardian_first_name": true,
	"guardian_last_name":  true,
	"guardian_email":      true,
	"guardian_phone":      true,
	"first_name":          true,
	"last_name":           true,
	"date_of_birth":       true,
	"target_grade_level":  true,
}

// keyPattern allows lowercase letters, numbers, and underscores. Mirrors
// the settings registry key shape so admins build muscle memory on one
// convention.
var keyAllowedRunes = func(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
}

// Validate checks a single field. Used by the form schema service when
// admins create / update a schema version.
func (f *FormField) Validate() error {
	f.Key = strings.TrimSpace(f.Key)
	if f.Key == "" {
		return invalidInput(CodeFormFieldKeyInvalid, "key", errors.New("form field key is required"))
	}
	if len(f.Key) > formFieldKeyMaxLength {
		return invalidInput(CodeFormFieldKeyInvalid, "key", errors.New("form field key must be at most 64 characters"))
	}
	for _, r := range f.Key {
		if !keyAllowedRunes(r) {
			return invalidInputf(CodeFormFieldKeyInvalid, "key", "form field key %q must be lowercase letters, digits, or underscores", f.Key)
		}
	}
	if CoreFieldKeys[f.Key] {
		return invalidInputf(CodeFormFieldKeyInvalid, "key", "form field key %q is reserved for a core field", f.Key)
	}

	if err := f.validateType(); err != nil {
		return err
	}

	f.Label = strings.TrimSpace(f.Label)
	f.Content = strings.TrimSpace(f.Content)

	if f.Type == FormFieldInfo {
		if err := f.validateInfo(); err != nil {
			return err
		}
	} else if err := f.validateQuestion(); err != nil {
		return err
	}

	if f.VisibleWhen != nil {
		if err := f.VisibleWhen.Validate(f.AppliesToCh); err != nil {
			return invalidInput(CodeFormFieldVisibilityInvalid, "visible_when", fmt.Errorf("form field %q visibility: %w", f.Key, err))
		}
	}

	return f.normalizeTranslations()
}

// validateInfo enforces the constraints specific to read-only
// information blocks: they carry body text instead of collecting an
// answer, so the answer-oriented attributes must all be empty. The
// label (heading) is optional.
func (f *FormField) validateInfo() error {
	if err := ValidateFormAttachments(f.Attachments); err != nil {
		return err
	}
	if f.Content == "" {
		return invalidInputf(CodeFormFieldContentRequired, "content", "information field %q requires content text", f.Key)
	}
	if f.Required {
		return fmt.Errorf("information field %q cannot be required", f.Key)
	}
	if len(f.Options) > 0 {
		return fmt.Errorf("information field %q must not declare options", f.Key)
	}
	if f.Target != "" {
		return fmt.Errorf("information field %q must not declare a target", f.Key)
	}
	if f.Validation != nil {
		return fmt.Errorf("information field %q must not declare validation", f.Key)
	}
	if len(f.AllowedTimes) > 0 {
		return fmt.Errorf("information field %q must not declare allowed_times", f.Key)
	}
	if len(f.SingleModeGrades) > 0 {
		return fmt.Errorf("information field %q must not declare single_mode_grades", f.Key)
	}
	return nil
}

// SingleModeAppliesTo reports whether this field's Heimweg-Beschränkung
// restricts the given target grade level. A nil grade (grade collection off,
// or a legacy request without one) is never restricted — the rule keys on the
// declared target grade and silently falls back to multi-select without it.
func (f *FormField) SingleModeAppliesTo(grade *int16) bool {
	if grade == nil {
		return false
	}
	for _, g := range f.SingleModeGrades {
		if g == int(*grade) {
			return true
		}
	}
	return false
}

// validateQuestion enforces the constraints for answer-collecting fields
// (every type except FormFieldInfo).
func (f *FormField) validateQuestion() error {
	if len(f.Attachments) > 0 {
		return invalidInputf(CodeFormAttachmentInvalid, "attachments", "only information fields may declare attachments")
	}
	if f.Label == "" {
		return invalidInput(CodeFormFieldLabelRequired, "label", errors.New("form field label is required"))
	}
	if f.Content != "" {
		return fmt.Errorf("only information fields may declare content (field %q)", f.Key)
	}

	if f.Type == FormFieldSelect && len(f.Options) == 0 {
		return invalidInputf(CodeFormFieldOptionsInvalid, "options", "select field %q must declare at least one option", f.Key)
	}
	if f.Type != FormFieldSelect && len(f.Options) > 0 {
		return invalidInputf(CodeFormFieldOptionsInvalid, "options", "non-select field %q must not declare options", f.Key)
	}
	if err := f.validateAllowedTimes(); err != nil {
		return err
	}
	if err := f.validateSingleModeGrades(); err != nil {
		return err
	}
	return f.validateTarget()
}

// validateAllowedTimes checks and normalizes the fixed pickup times.
func (f *FormField) validateAllowedTimes() error {
	// AllowedTimes (fixed pickup times) is only meaningful on the pickup
	// schedule field; everywhere else it would silently do nothing, so
	// reject it. In particular the arrival schedule (schedule.arrival)
	// stays free-entry by design. Each entry must be a unique HH:MM
	// value; normalize (trim) in place so the stored schema is canonical.
	if len(f.AllowedTimes) > 0 {
		if f.Target != TargetSchedulePickup {
			return fmt.Errorf("field %q: allowed_times is only valid on the pickup-times field", f.Key)
		}
		seen := make(map[string]bool, len(f.AllowedTimes))
		for i, t := range f.AllowedTimes {
			t = strings.TrimSpace(t)
			if t == "" {
				return invalidInputf(CodeFormFieldAllowedTimesInvalid, "allowed_times", "field %q: allowed_times must not contain empty entries", f.Key)
			}
			if _, err := time.Parse("15:04", t); err != nil {
				return invalidInputf(CodeFormFieldAllowedTimesInvalid, "allowed_times", "field %q: allowed_times entry %q must be HH:MM", f.Key, t)
			}
			if seen[t] {
				return invalidInputf(CodeFormFieldAllowedTimesInvalid, "allowed_times", "field %q: duplicate allowed_times entry %q", f.Key, t)
			}
			seen[t] = true
			f.AllowedTimes[i] = t
		}
	}
	return nil
}

// validateSingleModeGrades checks and normalizes the Heimweg-Beschränkung
// grades.
func (f *FormField) validateSingleModeGrades() error {
	// SingleModeGrades (Heimweg-Beschränkung, #2381) only makes sense on
	// the allowed-departure-modes field; anywhere else it would silently
	// do nothing, so reject it. Grade bounds and dedup reuse the shared
	// grade-list normalization.
	if len(f.SingleModeGrades) > 0 {
		if f.Target != TargetStudentAllowedDepartureModes {
			return fmt.Errorf("field %q: single_mode_grades is only valid on the allowed-departure-modes field", f.Key)
		}
		if !f.AppliesToCh {
			return fmt.Errorf("field %q: single_mode_grades requires a child field", f.Key)
		}
		normalized, err := NormalizeGradeLevelList("single_mode_grades", f.SingleModeGrades)
		if err != nil {
			return fmt.Errorf("field %q: %w", f.Key, err)
		}
		f.SingleModeGrades = normalized
	}
	return nil
}

// validateTarget checks the structured-type and reserved-target pairing.
func (f *FormField) validateTarget() error {
	// Structured types are only meaningful when paired with their
	// canonical target. They have no free-form variant — admins can't
	// invent a "phone_list" with their own semantics.
	if isStructuredFieldType(f.Type) && f.Target == "" {
		return invalidInputf(CodeFormFieldTargetInvalid, "target", "field type %q must declare a target", f.Type)
	}

	// Target consistency: when set, the target dictates the type so
	// the decision service can dispatch without ambiguity. Admin
	// editor enforces this too; this check is the backstop. The Label
	// is intentionally NOT constrained — admins may rename the question
	// shown to parents while keeping the canonical target wiring.
	if f.Target != "" {
		spec, ok := ReservedTargets[f.Target]
		if !ok {
			return invalidInputf(CodeFormFieldTargetInvalid, "target", "unknown form field target %q", f.Target)
		}
		if spec.Type != f.Type {
			return invalidInputf(CodeFormFieldTargetInvalid, "target", "target %q requires type %q, got %q", f.Target, spec.Type, f.Type)
		}
	}

	return nil
}

// isStructuredFieldType reports whether the type's value is a JSON
// object/array with internal sub-fields (vs a plain scalar). The
// renderer and decision service treat these specially.
func isStructuredFieldType(t FormFieldType) bool {
	switch t {
	case FormFieldPhoneList, FormFieldWeekdaySchedule, FormFieldWeekdayBoolean, FormFieldWeekdayMode, FormFieldWeekdayMultiMode, FormFieldContactList:
		return true
	default:
		return false
	}
}

// ValidPhoneTypes mirrors the CHECK constraint on
// users.guardian_phone_numbers.phone_type.
var ValidPhoneTypes = map[string]bool{
	"mobile": true,
	"home":   true,
	"work":   true,
	"other":  true,
}

// Validate enforces the shape required to round-trip to
// users.guardian_phone_numbers.
func (p *PhoneEntry) Validate() error {
	p.PhoneNumber = strings.TrimSpace(p.PhoneNumber)
	if p.PhoneNumber == "" {
		return errors.New("phone_number is required")
	}
	if p.PhoneType == "" {
		p.PhoneType = "other"
	}
	if !ValidPhoneTypes[p.PhoneType] {
		return fmt.Errorf("phone_type %q must be mobile/home/work/other", p.PhoneType)
	}
	return nil
}

// ValidWeekdayModes is the set of accepted WeekdayMode values.
var ValidWeekdayModes = map[string]bool{
	WeekdayModeAlone:       true,
	WeekdayModeBus:         true,
	WeekdayModePickup:      true,
	WeekdayModeAccompanied: true,
}

// Validate checks every entry is a known weekday with a known mode.
func (w WeekdayMode) Validate() error {
	for day, mode := range w {
		if !ValidWeekdays[day] {
			return fmt.Errorf("weekday %q must be one of mon/tue/wed/thu/fri", day)
		}
		if !ValidWeekdayModes[mode] {
			return fmt.Errorf("weekday %q mode %q must be one of alone/bus/pickup/accompanied", day, mode)
		}
	}
	return nil
}

func (w WeekdayMultiMode) Validate() error {
	for day, modes := range w {
		if !ValidWeekdays[day] {
			return fmt.Errorf("weekday %q must be one of mon/tue/wed/thu/fri", day)
		}
		if len(modes) == 0 {
			return fmt.Errorf("weekday %q must contain at least one departure mode", day)
		}
		seen := map[string]bool{}
		for _, mode := range modes {
			if !ValidWeekdayModes[mode] {
				return fmt.Errorf("weekday %q mode %q must be one of alone/bus/pickup/accompanied", day, mode)
			}
			if seen[mode] {
				return fmt.Errorf("weekday %q contains duplicate mode %q", day, mode)
			}
			seen[mode] = true
		}
	}
	return nil
}

// ValidateSingleSelection checks the plan is well-formed AND every weekday
// carries at most one departure mode. Backs the server-side Heimweg-
// Beschränkung (#2381): the public form renders a single-choice picker for
// restricted grades, but a scripted or stale client could still POST several
// modes per day. Callers must only invoke this for children whose target
// grade is restricted (FormField.SingleModeAppliesTo).
func (w WeekdayMultiMode) ValidateSingleSelection() error {
	if err := w.Validate(); err != nil {
		return err
	}
	for day, modes := range w {
		if len(modes) > 1 {
			return fmt.Errorf("weekday %q allows only one departure mode, got %d", day, len(modes))
		}
	}
	return nil
}

// ValidWeekdays mirrors the keys WeekdaySchedule accepts; aligns
// with how schedule.student_pickup_schedules / arrival_schedules
// encode their per-day rows.
var ValidWeekdays = map[string]bool{
	"mon": true, "tue": true, "wed": true, "thu": true, "fri": true,
}

// Validate checks every entry is a known weekday with a HH:MM time.
func (w WeekdaySchedule) Validate() error {
	for day, hhmm := range w {
		if !ValidWeekdays[day] {
			return fmt.Errorf("weekday %q must be one of mon/tue/wed/thu/fri", day)
		}
		hhmm = strings.TrimSpace(hhmm)
		if hhmm == "" {
			continue
		}
		if _, err := time.Parse("15:04", hhmm); err != nil {
			return fmt.Errorf("weekday %q time %q must be HH:MM", day, hhmm)
		}
	}
	return nil
}

// ValidateAllowed checks the schedule is well-formed AND every set time is a
// member of allowed (the field's configured fixed pickup times). Empty days
// stay valid ("kein fester Eintrag"). Callers must skip this when allowed is
// empty — an empty allowed list means "no restriction", not "nothing
// permitted". Backs the server-side enforcement so a scripted client can't
// bypass the fixed-times dropdown the public form renders.
func (w WeekdaySchedule) ValidateAllowed(allowed []string) error {
	if err := w.Validate(); err != nil {
		return err
	}
	set := make(map[string]bool, len(allowed))
	for _, t := range allowed {
		set[strings.TrimSpace(t)] = true
	}
	for day, hhmm := range w {
		hhmm = strings.TrimSpace(hhmm)
		if hhmm == "" {
			continue
		}
		if !set[hhmm] {
			return fmt.Errorf("weekday %q time %q is not an allowed pickup time", day, hhmm)
		}
	}
	return nil
}

func (w WeekdayBoolean) Validate() error {
	for day := range w {
		if !ValidWeekdays[day] {
			return fmt.Errorf("weekday %q must be one of mon/tue/wed/thu/fri", day)
		}
	}
	return nil
}

func (w WeekdayBoolean) HasAny() bool {
	for day, selected := range w {
		if !ValidWeekdays[day] {
			continue
		}
		if selected {
			return true
		}
	}
	return false
}

// Validate enforces guardian_profiles.CHECK (email OR ≥1 phone) plus
// per-phone validity. Relationship type is admitted as free text;
// the decision service maps it onto the enum via the same helper the
// CSV importer uses.
func (c *ContactEntry) Validate() error {
	c.FirstName = strings.TrimSpace(c.FirstName)
	c.LastName = strings.TrimSpace(c.LastName)
	if c.FirstName == "" || c.LastName == "" {
		return errors.New("contact first_name and last_name are required")
	}
	c.Email = strings.TrimSpace(c.Email)
	hasPhone := false
	for i := range c.PhoneNumbers {
		if err := c.PhoneNumbers[i].Validate(); err != nil {
			return fmt.Errorf("contact %s %s: %w", c.FirstName, c.LastName, err)
		}
		hasPhone = true
	}
	if c.Email == "" && !hasPhone {
		return errors.New("contact requires at least one of email or phone_numbers")
	}
	if c.EmergencyPriority < 0 {
		return errors.New("emergency_priority must be non-negative")
	}
	return nil
}

var validLegalBlockKinds = map[string]bool{
	LegalBlockKindTerms:         true,
	LegalBlockKindPrivacyNotice: true,
	LegalBlockKindNotice:        true,
	LegalBlockKindConsent:       true,
}

var standardLegalBlockKeys = map[string]bool{
	ConsentKeyAGB:            true,
	ConsentKeyDataProcessing: true,
	ConsentKeyEmailContact:   true,
	ConsentKeyPhoto:          true,
}

func (b *FormLegalBlock) Validate() error {
	b.Key = strings.TrimSpace(b.Key)
	b.Kind = strings.TrimSpace(b.Kind)
	b.Title = strings.TrimSpace(b.Title)
	b.Label = strings.TrimSpace(b.Label)
	b.Text = strings.TrimSpace(b.Text)
	b.Source = strings.TrimSpace(b.Source)
	b.DisplayMode = strings.TrimSpace(b.DisplayMode)
	b.DocumentURL = strings.TrimSpace(b.DocumentURL)

	validators := []func() error{
		b.validateKey,
		b.validateSource,
		b.validateContent,
		b.validateDisplayMode,
		func() error { return ValidateFormAttachments(b.Attachments) },
	}
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	var err error
	b.Translations, err = b.Translations.Normalize(TranslationAttrTitle, TranslationAttrLabel, TranslationAttrText)
	if err != nil {
		return fmt.Errorf("legal block %q: %w", b.Key, err)
	}
	return nil
}

func (b *FormLegalBlock) validateKey() error {
	if b.Key == "" {
		return invalidInput(CodeLegalBlockKeyInvalid, "key", errors.New("legal block key is required"))
	}
	if len(b.Key) > formFieldKeyMaxLength {
		return invalidInput(CodeLegalBlockKeyInvalid, "key", errors.New("legal block key must be at most 64 characters"))
	}
	for _, r := range b.Key {
		if !keyAllowedRunes(r) {
			return invalidInputf(CodeLegalBlockKeyInvalid, "key", "legal block key %q must be lowercase letters, digits, or underscores", b.Key)
		}
	}
	return nil
}

// validateSource defaults the source from the key and checks the pair.
func (b *FormLegalBlock) validateSource() error {
	if b.Source == "" {
		if standardLegalBlockKeys[b.Key] {
			b.Source = LegalBlockSourceStandard
		} else {
			b.Source = LegalBlockSourceCustom
		}
	}
	if b.Source != LegalBlockSourceStandard && b.Source != LegalBlockSourceCustom {
		return fmt.Errorf("legal block source %q must be standard or custom", b.Source)
	}
	if b.Source == LegalBlockSourceStandard && !standardLegalBlockKeys[b.Key] {
		return fmt.Errorf("standard legal block key %q is not recognized", b.Key)
	}
	if b.Source == LegalBlockSourceCustom && standardLegalBlockKeys[b.Key] {
		return invalidInputf(CodeLegalBlockKeyInvalid, "key", "custom legal block key %q is reserved for a standard block", b.Key)
	}
	return nil
}

// validateContent checks the kind, the texts an enabled block needs and the
// kinds that must not be required.
func (b *FormLegalBlock) validateContent() error {
	if !validLegalBlockKinds[b.Kind] {
		return fmt.Errorf("legal block %q has unknown kind %q", b.Key, b.Kind)
	}
	if b.Enabled && b.Label == "" {
		return invalidInputf(CodeLegalBlockIncomplete, "label", "enabled legal block %q requires a label", b.Key)
	}
	if b.Enabled && b.Title == "" {
		return invalidInputf(CodeLegalBlockIncomplete, "title", "enabled legal block %q requires a title", b.Key)
	}
	if b.Kind == LegalBlockKindNotice && b.Required {
		return invalidInputf(CodeLegalBlockRequiredNotAllowed, "required", "notice legal block %q cannot be required", b.Key)
	}
	// Consent is only valid when freely given (Art. 7 Abs. 4 DSGVO), so a
	// consent block must never gate the submit. A required checkbox is a
	// terms/acknowledgement block, not a consent — the editor converts
	// accordingly. Already-persisted schema versions are only read, never
	// re-validated, so legacy consent+required rows keep resolving.
	if b.Kind == LegalBlockKindConsent && b.Required {
		return invalidInputf(CodeLegalBlockRequiredNotAllowed, "required", "consent legal block %q cannot be required", b.Key)
	}
	return nil
}

// Validate checks fields for duplicate keys + per-field validity.
func (s *FormSchema) Validate() error {
	if s.Name == "" {
		return invalidInput(CodeSchemaNameRequired, "name", errors.New("form schema name is required"))
	}
	if s.Version <= 0 {
		return errors.New("form schema version must be positive")
	}
	if s.CreatedBy <= 0 {
		return errors.New("form schema created_by is required")
	}
	if s.CoreRequirements == nil {
		s.CoreRequirements = CoreRequirements{}
	}
	if err := s.CoreRequirements.Validate(); err != nil {
		return err
	}
	if err := s.validateLegalBlocks(); err != nil {
		return err
	}
	byKey, err := s.validateFields()
	if err != nil {
		return err
	}

	// Second pass: validate cross-field visibility references now that
	// every key is known. A "field" condition must point at an existing
	// boolean/select field that the owning field can actually observe.
	for i := range s.Fields {
		if err := validateFieldVisibility(&s.Fields[i], byKey); err != nil {
			return NestInput(err, fmt.Sprintf("fields.%d", i), "field %d", i)
		}
	}
	return nil
}

// validateLegalBlocks validates every legal block and rejects duplicate keys.
func (s *FormSchema) validateLegalBlocks() error {
	// NOTE: a template may deliberately enable blocks while keeping
	// data_processing disabled — pilot schools without a standalone
	// Datenschutzinformation run their consent via the Elternbrief/AGB
	// block. The editor surfaces a non-blocking hint instead of a hard
	// validation here.
	legalByKey := make(map[string]bool, len(s.LegalBlocks))
	for i := range s.LegalBlocks {
		if err := s.LegalBlocks[i].Validate(); err != nil {
			return NestInput(err, fmt.Sprintf("legal_blocks.%d", i), "legal block %d", i)
		}
		if legalByKey[s.LegalBlocks[i].Key] {
			return invalidInputf(CodeLegalBlockKeyDuplicate, fmt.Sprintf("legal_blocks.%d.key", i), "duplicate legal block key %q", s.LegalBlocks[i].Key)
		}
		legalByKey[s.LegalBlocks[i].Key] = true
	}
	return nil
}

// validateFields validates every field, rejects duplicate keys and reserved
// targets, and returns the fields by key.
func (s *FormSchema) validateFields() (map[string]*FormField, error) {
	byKey := make(map[string]*FormField, len(s.Fields))
	seenTarget := make(map[string]string, len(s.Fields))
	for i := range s.Fields {
		if err := s.Fields[i].Validate(); err != nil {
			return nil, NestInput(err, fmt.Sprintf("fields.%d", i), "field %d", i)
		}
		if _, dup := byKey[s.Fields[i].Key]; dup {
			return nil, invalidInputf(CodeFormFieldKeyDuplicate, fmt.Sprintf("fields.%d.key", i), "duplicate form field key %q", s.Fields[i].Key)
		}
		byKey[s.Fields[i].Key] = &s.Fields[i]
		// A reserved target writes one specific Stammdaten column on approval, so
		// two fields pointing at the same target produce an undefined last-field-wins
		// outcome — the safety-sensitive departure plan in particular can silently
		// collapse back into self-goer semantics (#1694). Free custom fields (empty
		// target) may repeat. Keep in sync with the editor's reserved-targets picker.
		if t := s.Fields[i].Target; t != "" {
			if firstKey, dup := seenTarget[t]; dup {
				return nil, invalidInputf(CodeFormFieldTargetDuplicate, fmt.Sprintf("fields.%d.target", i), "duplicate field target %q on fields %q and %q", t, firstKey, s.Fields[i].Key)
			}
			seenTarget[t] = s.Fields[i].Key
		}
	}
	return byKey, nil
}

// validateFieldVisibility checks a field's "field"-sourced visibility
// condition against its sibling fields: the controlling field must
// exist, be a yes/no or selection field, not be the field itself, and
// be observable from the owning field's scope (a parent-level field
// cannot depend on a per-child answer).
func validateFieldVisibility(f *FormField, byKey map[string]*FormField) error {
	if f.VisibleWhen == nil || f.VisibleWhen.Source != ConditionSourceField {
		return nil
	}
	ref := strings.TrimSpace(f.VisibleWhen.Field)
	if ref == f.Key {
		return invalidInputf(CodeFormFieldVisibilityInvalid, "visible_when", "field %q cannot depend on itself", f.Key)
	}
	controller, ok := byKey[ref]
	if !ok {
		return invalidInputf(CodeFormFieldVisibilityInvalid, "visible_when", "field %q depends on unknown field %q", f.Key, ref)
	}
	if controller.Type != FormFieldBoolean && controller.Type != FormFieldSelect {
		return invalidInputf(CodeFormFieldVisibilityInvalid, "visible_when", "field %q can only depend on a yes/no or selection field, but %q is %q", f.Key, ref, controller.Type)
	}
	if !f.AppliesToCh && controller.AppliesToCh {
		return invalidInputf(CodeFormFieldVisibilityInvalid, "visible_when", "parent-level field %q cannot depend on per-child field %q", f.Key, ref)
	}
	return nil
}

// normalizeGradeLevelList validates the supported school grades and preserves
// the configured order while removing duplicates.
func NormalizeGradeLevelList(field string, levels []int) ([]int, error) {
	out := make([]int, 0, len(levels))
	seen := make(map[int]bool, len(levels))
	for _, level := range levels {
		if level < 1 || level > 13 {
			return nil, fmt.Errorf("%s contains invalid grade %d", field, level)
		}
		if !seen[level] {
			seen[level] = true
			out = append(out, level)
		}
	}
	return out, nil
}
