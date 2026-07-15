// Package enrollment holds domain entities for the parent-enrollment
// feature. The schema is split across several tables - form_schemas
// (versioned form definitions), requests (parent submissions),
// request_children (per-child decisions), and care_offerings +
// request_child_offerings.
package enrollment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// FormFieldType enumerates the field types admins can place in the
// custom-fields portion of a form schema. Mirrors the settings registry
// vocabulary so the auto-generated frontend renderer can be reused.
//
// "textarea" is the only enrollment-specific addition (handy for
// open-ended fields like allergy notes). "email" / "phone" are NOT
// included - guardian email + phone are core fields with their own
// columns on enrollment.requests.
type FormFieldType string

const (
	FormFieldBoolean  FormFieldType = "boolean"
	FormFieldNumber   FormFieldType = "number"
	FormFieldText     FormFieldType = "text"
	FormFieldTextarea FormFieldType = "textarea"
	FormFieldDate     FormFieldType = "date"
	FormFieldSelect   FormFieldType = "select"

	// FormFieldInfo is a read-only text block (optional heading + plain
	// description) shown to parents. It collects no answer: the
	// submission + decision services ignore it, and it can never be
	// required or carry options/target/validation. Use it for
	// instructions, notes, or disclaimers placed between questions. Like
	// any field it may carry a VisibleWhen condition.
	FormFieldInfo FormFieldType = "information"

	// Structured types — admins cannot define their internal shape;
	// the renderer + decision service know how to interpret them.
	// Each pairs with a specific FormField.Target (see ReservedTargets).
	FormFieldPhoneList        FormFieldType = "phone_list"         // 0..N labelled phone numbers
	FormFieldWeekdaySchedule  FormFieldType = "weekday_schedule"   // mon..fri → HH:MM, optional per day
	FormFieldWeekdayBoolean   FormFieldType = "weekday_boolean"    // mon..fri → bool, optional per day
	FormFieldWeekdayMode      FormFieldType = "weekday_mode"       // mon..fri → alone/bus/pickup, optional per day
	FormFieldWeekdayMultiMode FormFieldType = "weekday_multi_mode" // mon..fri → []alone/bus/pickup, optional per day
	FormFieldContactList      FormFieldType = "contact_list"       // 0..N people (name + phones + flags)
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
}

// FormFieldOption is a single static option on a select-type field.
type FormFieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// FormFieldValidation mirrors the settings registry's validation shape so
// the frontend renderer can apply min/max/pattern hints without a separate
// schema. All fields optional.
type FormFieldValidation struct {
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Pattern *string  `json:"pattern,omitempty"`
}

// Visibility condition sources. A field with a VisibleWhen condition is
// only shown to parents when the source value matches.
const (
	// ConditionSourceField references another custom field in the same
	// schema by key (must be a boolean or select field).
	ConditionSourceField = "field"
	// ConditionSourceGradeLevel references the per-child core field
	// target_grade_level. Only valid on per-child fields.
	ConditionSourceGradeLevel = "grade_level"
	// ConditionSourceCareOffering references the child's selected care
	// offerings by name (Value is an offering name, matched
	// case-insensitively). Names are used rather than ids because form
	// templates are not bound to a phase and offering ids differ per
	// phase. Only valid on per-child fields, paired with the "includes"
	// operator. Evaluated client-side only — the backend never resolves
	// this source (care offerings are per-child; the submit flow's
	// validation checks guardian-level fields only).
	ConditionSourceCareOffering = "care_offering"
)

// Visibility condition operators.
const (
	ConditionOpEquals    = "eq"        // source value equals Value
	ConditionOpNotEquals = "neq"       // source value differs from Value
	ConditionOpNotEmpty  = "not_empty" // source has any non-empty value
	ConditionOpIncludes  = "includes"  // multi-value source contains Value (care offering only)
)

// VisibilityCondition makes a field appear only when a controlling value
// matches. It mirrors the settings registry's Dependency shape but adds
// enrollment-specific sources for the built-in grade level and the
// child's selected care offerings, plus an "includes" operator for the
// multi-value offering check. nil means "always visible".
type VisibilityCondition struct {
	Source   string `json:"source"`          // ConditionSource* constant
	Field    string `json:"field,omitempty"` // controlling custom field key (Source == "field")
	Operator string `json:"operator"`        // ConditionOp* constant
	Value    any    `json:"value,omitempty"` // compared value (required except for not_empty)
}

// Validate checks a condition's shape in isolation. Cross-field
// reference + scope checks (does the controlling field exist, is it a
// boolean/select, same scope) happen in FormSchema.Validate, which can
// see sibling fields. appliesToChild is the owning field's scope; the
// grade_level / care_offering sources are per-child only.
func (c *VisibilityCondition) Validate(appliesToChild bool) error {
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

// FormField is one custom-field definition. Stored as a JSON object inside
// FormSchema.Fields.
//
// Target is the optional link to a Stammdaten column / association table.
// When empty, the field's value stays in request[_children].custom_data and
// is never auto-migrated. When set to one of the ReservedTargets entries,
// the decision service copies the value onto the appropriate downstream
// record on approval (Student column, guardian_phone_numbers row, schedule
// row, additional students_guardians row, etc.). Type is constrained by
// Target — see ReservedTargets for the allowed (target, type) pairs.
type FormField struct {
	Key      string            `json:"key"`
	Label    string            `json:"label"`
	Type     FormFieldType     `json:"type"`
	Required bool              `json:"required,omitempty"`
	HelpText string            `json:"help_text,omitempty"`
	Content  string            `json:"content,omitempty"` // body text for FormFieldInfo blocks (plain text, newlines preserved); empty for all other types
	Options  []FormFieldOption `json:"options,omitempty"`
	// AllowedTimes constrains a FormFieldWeekdaySchedule field to a fixed
	// list of HH:MM pickup times. Empty/absent = free time entry (the
	// historical behaviour). When set, every per-weekday time a parent
	// submits must be a member of this list. Only valid on a
	// weekday_schedule field; rejected on every other type.
	AllowedTimes []string             `json:"allowed_times,omitempty"`
	Validation   *FormFieldValidation `json:"validation,omitempty"`
	SortOrder    int                  `json:"sort_order"`
	AppliesToCh  bool                 `json:"applies_to_child,omitempty"` // false (default) = guardian-level field; true = per-child field
	Target       string               `json:"target,omitempty"`           // "" = free custom field; otherwise one of ReservedTargets
	VisibleWhen  *VisibilityCondition `json:"visible_when,omitempty"`     // nil = always visible; otherwise show only when the condition matches
}

const CoreRequirementGuardianPhone = "guardian_phone"

var validCoreRequirements = map[string]bool{
	CoreRequirementGuardianPhone: true,
}

// CoreRequirements stores per-template required-state for core fields
// that are rendered from dedicated request/request_child columns rather
// than from FormSchema.Fields. A missing or false value means optional.
type CoreRequirements map[string]bool

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

// ReservedTargets maps each known target to the FormFieldType it requires
// and a hint whether the field is naturally per-child (applies_to_child).
// Admins pick from this list in the editor; the editor locks the type to
// match. The decision service uses the same table to dispatch values onto
// downstream records during approval.
type ReservedTarget struct {
	Type           FormFieldType
	AppliesToChild bool
	Label          string // German default — admin may override per field
}

const (
	TargetStudentHealthInfo = "student.health_info"
	TargetStudentExtraInfo  = "student.extra_info"
	// TargetStudentDeparture is the canonical unified target: per weekday, how
	// the child leaves (alone/bus/pickup). It supersedes the separate Buskind
	// (TargetStudentBusDays/TargetStudentBus) and Abholregelung
	// (TargetStudentPickupStatus) targets, which are kept as legacy aliases so
	// older saved schemas keep working; all of them dispatch onto
	// student.departure_days, the single source of truth (#1610).
	TargetStudentDeparture             = "student.departure"
	TargetStudentAllowedDepartureModes = "student.allowed_departure_modes"
	// TargetStudentBusDays is the legacy Buskind target. TargetStudentBus
	// ("student.bus") is kept as a legacy alias so older saved schemas keep
	// working; both dispatch onto student.departure_days as bus days.
	TargetStudentBusDays      = "student.bus_days"
	TargetStudentBus          = "student.bus"
	TargetStudentPickupStatus = "student.pickup_status"
	// TargetStudentDepartureCompanionNote captures the free-text "mit wem" for
	// the accompanied departure mode (sibling, friend, named person). It is a
	// coupled companion to TargetStudentAllowedDepartureModes, NOT an
	// independently admin-pickable field: it rides on a reserved child
	// custom-data key under this string and is applied (accompanied-gated) by
	// the decision service. It is intentionally absent from ReservedTargets so
	// a schema cannot declare it as a standalone field whose answer would be
	// silently dropped on approval (#1694).
	TargetStudentDepartureCompanionNote = "student.departure_companion_note"
	TargetSchedulePickup                = "schedule.pickup"
	TargetScheduleArrival               = "schedule.arrival"
	TargetStudentContacts               = "student.contacts"
)

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

// formFieldKeyMaxLength caps the length of a custom form field key.
const formFieldKeyMaxLength = 64

// Validate checks a single field. Used by the form schema service when
// admins create / update a schema version.
func (f *FormField) Validate() error {
	f.Key = strings.TrimSpace(f.Key)
	if f.Key == "" {
		return errors.New("form field key is required")
	}
	if len(f.Key) > formFieldKeyMaxLength {
		return errors.New("form field key must be at most 64 characters")
	}
	for _, r := range f.Key {
		if !keyAllowedRunes(r) {
			return fmt.Errorf("form field key %q must be lowercase letters, digits, or underscores", f.Key)
		}
	}
	if CoreFieldKeys[f.Key] {
		return fmt.Errorf("form field key %q is reserved for a core field", f.Key)
	}

	if !validFormFieldTypes[f.Type] {
		return fmt.Errorf("unknown form field type %q", f.Type)
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
			return fmt.Errorf("form field %q visibility: %w", f.Key, err)
		}
	}

	return nil
}

// validateInfo enforces the constraints specific to read-only
// information blocks: they carry body text instead of collecting an
// answer, so the answer-oriented attributes must all be empty. The
// label (heading) is optional.
func (f *FormField) validateInfo() error {
	if f.Content == "" {
		return fmt.Errorf("information field %q requires content text", f.Key)
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
	return nil
}

// validateQuestion enforces the constraints for answer-collecting fields
// (every type except FormFieldInfo).
func (f *FormField) validateQuestion() error {
	if f.Label == "" {
		return errors.New("form field label is required")
	}
	if f.Content != "" {
		return fmt.Errorf("only information fields may declare content (field %q)", f.Key)
	}

	if f.Type == FormFieldSelect && len(f.Options) == 0 {
		return fmt.Errorf("select field %q must declare at least one option", f.Key)
	}
	if f.Type != FormFieldSelect && len(f.Options) > 0 {
		return fmt.Errorf("non-select field %q must not declare options", f.Key)
	}

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
				return fmt.Errorf("field %q: allowed_times must not contain empty entries", f.Key)
			}
			if _, err := time.Parse("15:04", t); err != nil {
				return fmt.Errorf("field %q: allowed_times entry %q must be HH:MM", f.Key, t)
			}
			if seen[t] {
				return fmt.Errorf("field %q: duplicate allowed_times entry %q", f.Key, t)
			}
			seen[t] = true
			f.AllowedTimes[i] = t
		}
	}

	// Structured types are only meaningful when paired with their
	// canonical target. They have no free-form variant — admins can't
	// invent a "phone_list" with their own semantics.
	if isStructuredFieldType(f.Type) && f.Target == "" {
		return fmt.Errorf("field type %q must declare a target", f.Type)
	}

	// Target consistency: when set, the target dictates the type so
	// the decision service can dispatch without ambiguity. Admin
	// editor enforces this too; this check is the backstop. The Label
	// is intentionally NOT constrained — admins may rename the question
	// shown to parents while keeping the canonical target wiring.
	if f.Target != "" {
		spec, ok := ReservedTargets[f.Target]
		if !ok {
			return fmt.Errorf("unknown form field target %q", f.Target)
		}
		if spec.Type != f.Type {
			return fmt.Errorf("target %q requires type %q, got %q", f.Target, spec.Type, f.Type)
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

// PhoneEntry is one row of a FormFieldPhoneList submission. The
// decision service inserts one users.guardian_phone_numbers row per
// entry. PhoneType is validated against the enum from migration
// 1.7.6 (mobile/home/work/other).
type PhoneEntry struct {
	PhoneNumber string `json:"phone_number"`
	PhoneType   string `json:"phone_type"`
	Label       string `json:"label,omitempty"`
	IsPrimary   bool   `json:"is_primary,omitempty"`
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

// WeekdaySchedule is the value of a FormFieldWeekdaySchedule field.
// Keys are weekday names (mon/tue/wed/thu/fri); values are HH:MM
// strings. Missing or empty days are skipped on approval.
type WeekdaySchedule map[string]string

// WeekdayBoolean is the value of a FormFieldWeekdayBoolean field.
// Keys are weekday names (mon/tue/wed/thu/fri); true means selected.
type WeekdayBoolean map[string]bool

// Weekday departure modes for a FormFieldWeekdayMode value. They mirror
// users.DepartureMode without importing the users package, keeping the
// enrollment model layer self-contained.
const (
	WeekdayModeAlone       = "alone"
	WeekdayModeBus         = "bus"
	WeekdayModePickup      = "pickup"
	WeekdayModeAccompanied = "accompanied"
)

// ValidWeekdayModes is the set of accepted WeekdayMode values.
var ValidWeekdayModes = map[string]bool{
	WeekdayModeAlone:       true,
	WeekdayModeBus:         true,
	WeekdayModePickup:      true,
	WeekdayModeAccompanied: true,
}

// WeekdayMode is the value of a FormFieldWeekdayMode field. Keys are weekday
// names (mon/tue/wed/thu/fri); values are alone/bus/pickup. A missing day (or
// "alone") means the child goes home alone that day.
type WeekdayMode map[string]string

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

// WeekdayMultiMode is the value of a FormFieldWeekdayMultiMode field. Each
// weekday contains every allowed departure mode for that care day.
type WeekdayMultiMode map[string][]string

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

// ContactEntry is one row of a FormFieldContactList submission. The
// shape mirrors GuardianImportData (services/import/student.go) so
// the decision service can run it through the same dedup-by-email
// path the CSV importer uses.
type ContactEntry struct {
	FirstName          string       `json:"first_name"`
	LastName           string       `json:"last_name"`
	Email              string       `json:"email,omitempty"`
	RelationshipType   string       `json:"relationship_type,omitempty"`
	PhoneNumbers       []PhoneEntry `json:"phone_numbers,omitempty"`
	CanPickup          bool         `json:"can_pickup,omitempty"`
	IsEmergencyContact bool         `json:"is_emergency_contact,omitempty"`
	EmergencyPriority  int          `json:"emergency_priority,omitempty"`
	AddressStreet      string       `json:"address_street,omitempty"`
	AddressCity        string       `json:"address_city,omitempty"`
	AddressPostalCode  string       `json:"address_postal_code,omitempty"`
	Notes              string       `json:"notes,omitempty"`
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

const (
	LegalBlockKindTerms         = "terms"
	LegalBlockKindPrivacyNotice = "privacy_notice"
	LegalBlockKindNotice        = "notice"
	LegalBlockKindConsent       = "consent"

	LegalBlockSourceStandard = "standard"
	LegalBlockSourceCustom   = "custom"

	LegalBlockDisplayModeText = "text"
	LegalBlockDisplayModePDF  = "pdf"
)

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

// FormLegalBlock is one consent/legal row owned by a form template.
// Standard blocks originate from tenant settings and may be overridden or
// disabled on the template. Custom blocks are stored only on the template.
type FormLegalBlock struct {
	Key         string `json:"key"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Label       string `json:"label"`
	Text        string `json:"text"`
	Required    bool   `json:"required"`
	Enabled     bool   `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
	Source      string `json:"source,omitempty"`
	DisplayMode string `json:"display_mode,omitempty"`
	DocumentURL string `json:"document_url,omitempty"`
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

	if b.Key == "" {
		return errors.New("legal block key is required")
	}
	if len(b.Key) > formFieldKeyMaxLength {
		return errors.New("legal block key must be at most 64 characters")
	}
	for _, r := range b.Key {
		if !keyAllowedRunes(r) {
			return fmt.Errorf("legal block key %q must be lowercase letters, digits, or underscores", b.Key)
		}
	}
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
		return fmt.Errorf("custom legal block key %q is reserved for a standard block", b.Key)
	}
	if !validLegalBlockKinds[b.Kind] {
		return fmt.Errorf("legal block %q has unknown kind %q", b.Key, b.Kind)
	}
	if b.Enabled && b.Label == "" {
		return fmt.Errorf("enabled legal block %q requires a label", b.Key)
	}
	if b.Enabled && b.Title == "" {
		return fmt.Errorf("enabled legal block %q requires a title", b.Key)
	}
	if b.Kind == LegalBlockKindNotice && b.Required {
		return fmt.Errorf("notice legal block %q cannot be required", b.Key)
	}
	if b.DisplayMode == "" {
		b.DisplayMode = LegalBlockDisplayModeText
	}
	if b.DisplayMode != LegalBlockDisplayModeText && b.DisplayMode != LegalBlockDisplayModePDF {
		return fmt.Errorf("legal block %q has unknown display mode %q", b.Key, b.DisplayMode)
	}
	if b.DisplayMode == LegalBlockDisplayModePDF {
		if b.Key != ConsentKeyAGB || b.Source != LegalBlockSourceStandard {
			return fmt.Errorf("legal block %q cannot use PDF display mode", b.Key)
		}
		if b.Enabled && b.DocumentURL == "" {
			return fmt.Errorf("enabled legal block %q requires a PDF document", b.Key)
		}
	}
	return nil
}

// FormSchema is a row in enrollment.form_schemas. Each save creates a new
// version; submissions pin to a specific schema_id so editing the active
// schema doesn't break already-submitted requests. Name groups versions
// of the same logical schema (e.g. "Schuljahr", "Ferienbetreuung") so
// admins can maintain multiple parallel form variants per tenant.
type FormSchema struct {
	base.Model `bun:"schema:enrollment,table:form_schemas"`
	base.TenantModel
	Name             string           `bun:"name,notnull,default:''" json:"name"`
	Version          int              `bun:"version,notnull" json:"version"`
	Fields           []FormField      `bun:"fields,type:jsonb,notnull,default:'[]'" json:"fields"`
	CoreRequirements CoreRequirements `bun:"core_requirements,type:jsonb,notnull,default:'{}'" json:"core_requirements"`
	LegalBlocks      []FormLegalBlock `bun:"legal_blocks,type:jsonb,notnull,default:'[]'" json:"legal_blocks"`
	IsActive         bool             `bun:"is_active,notnull,default:false" json:"is_active"`
	CreatedBy        int64            `bun:"created_by,notnull" json:"created_by"`
}

// Validate checks fields for duplicate keys + per-field validity.
func (s *FormSchema) Validate() error {
	if s.Name == "" {
		return errors.New("form schema name is required")
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
	// NOTE: a template may deliberately enable blocks while keeping
	// data_processing disabled — pilot schools without a standalone
	// Datenschutzinformation run their consent via the Elternbrief/AGB
	// block. The editor surfaces a non-blocking hint instead of a hard
	// validation here.
	legalByKey := make(map[string]bool, len(s.LegalBlocks))
	for i := range s.LegalBlocks {
		if err := s.LegalBlocks[i].Validate(); err != nil {
			return fmt.Errorf("legal block %d: %w", i, err)
		}
		if legalByKey[s.LegalBlocks[i].Key] {
			return fmt.Errorf("duplicate legal block key %q", s.LegalBlocks[i].Key)
		}
		legalByKey[s.LegalBlocks[i].Key] = true
	}
	byKey := make(map[string]*FormField, len(s.Fields))
	seenTarget := make(map[string]string, len(s.Fields))
	for i := range s.Fields {
		if err := s.Fields[i].Validate(); err != nil {
			return fmt.Errorf("field %d: %w", i, err)
		}
		if _, dup := byKey[s.Fields[i].Key]; dup {
			return fmt.Errorf("duplicate form field key %q", s.Fields[i].Key)
		}
		byKey[s.Fields[i].Key] = &s.Fields[i]
		// A reserved target writes one specific Stammdaten column on approval, so
		// two fields pointing at the same target produce an undefined last-field-wins
		// outcome — the safety-sensitive departure plan in particular can silently
		// collapse back into self-goer semantics (#1694). Free custom fields (empty
		// target) may repeat. Keep in sync with the editor's reserved-targets picker.
		if t := s.Fields[i].Target; t != "" {
			if firstKey, dup := seenTarget[t]; dup {
				return fmt.Errorf("duplicate field target %q on fields %q and %q", t, firstKey, s.Fields[i].Key)
			}
			seenTarget[t] = s.Fields[i].Key
		}
	}

	// Second pass: validate cross-field visibility references now that
	// every key is known. A "field" condition must point at an existing
	// boolean/select field that the owning field can actually observe.
	for i := range s.Fields {
		if err := validateFieldVisibility(&s.Fields[i], byKey); err != nil {
			return err
		}
	}
	return nil
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
		return fmt.Errorf("field %q cannot depend on itself", f.Key)
	}
	controller, ok := byKey[ref]
	if !ok {
		return fmt.Errorf("field %q depends on unknown field %q", f.Key, ref)
	}
	if controller.Type != FormFieldBoolean && controller.Type != FormFieldSelect {
		return fmt.Errorf("field %q can only depend on a yes/no or selection field, but %q is %q", f.Key, ref, controller.Type)
	}
	if !f.AppliesToCh && controller.AppliesToCh {
		return fmt.Errorf("parent-level field %q cannot depend on per-child field %q", f.Key, ref)
	}
	return nil
}

// FormSchemaRepository describes the DB operations the schema service needs.
type FormSchemaRepository interface {
	Create(ctx context.Context, schema *FormSchema) error
	FindByID(ctx context.Context, id int64) (*FormSchema, error)
	FindActive(ctx context.Context) (*FormSchema, error)
	ListByTenant(ctx context.Context) ([]*FormSchema, error)
	NextVersion(ctx context.Context) (int, error)
	NextVersionForName(ctx context.Context, name string) (int, error)
	// ExistsByName reports whether any version row already carries name
	// for the tenant in context. Used to reject a rename onto an existing
	// logical schema before touching the unique index.
	ExistsByName(ctx context.Context, name string) (bool, error)
	DeactivatePrevious(ctx context.Context) error
	UpdateActiveFlag(ctx context.Context, id int64, isActive bool) error
	// RenameByName renames every version of a logical schema in one
	// atomic update, so the whole version lineage keeps a shared name.
	// Callers must guard against colliding with an existing name first.
	RenameByName(ctx context.Context, oldName, newName string) error
	DeleteByName(ctx context.Context, name string) error
	HasLegalDocumentReference(ctx context.Context, storedURL, publicURL string) (bool, error)
	// LockLineages serializes lineage mutations (publish, rename, delete)
	// for the tenant in context against one another, so a rename and a
	// concurrent publish cannot split a version lineage. Transaction-scoped;
	// the caller must already be inside the request's tenant transaction.
	LockLineages(ctx context.Context) error
}

// SubmissionData is the reduced shape used by the legacy schema-service
// validation helper. The full public submit flow validates the request model
// directly.
type SubmissionData struct {
	GuardianFields map[string]any `json:"guardian_fields"`
	ChildFields    map[string]any `json:"child_fields"` // keyed by child index -> field map
}

// _ keeps `time` imported even when the model doesn't reference time.Time
// directly (the base.Model embeds CreatedAt/UpdatedAt).
var _ = time.Time{}
