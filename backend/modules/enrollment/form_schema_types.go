package enrollment

import "time"

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

// FormFieldOption is a single static option on a select-type field.
type FormFieldOption struct {
	Label        string       `json:"label"`
	Value        string       `json:"value"`
	Translations Translations `json:"translations,omitempty"` // of Label (#3377)
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

// FormField is one custom-field definition. Stored as a JSON object inside
// FormSchema.Fields.
//
// Target is the optional link to a Stammdaten column / association table.
// When empty, the field's value stays in request[_children].custom_data and
// is never auto-migrated. When set to one of the ReservedTargets entries,
// the decision service copies the value onto the appropriate downstream
// record on approval (Student column, guardian_phone_numbers row, schedule
// row, additional student-guardian relationship, etc.). Type is constrained by
// Target — see ReservedTargets for the allowed (target, type) pairs.
type FormField struct {
	Attachments []FormAttachment  `json:"attachments,omitempty"`
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Type        FormFieldType     `json:"type"`
	Required    bool              `json:"required,omitempty"`
	HelpText    string            `json:"help_text,omitempty"`
	Content     string            `json:"content,omitempty"` // body text for FormFieldInfo blocks (plain text, newlines preserved); empty for all other types
	Options     []FormFieldOption `json:"options,omitempty"`
	// AllowedTimes constrains a FormFieldWeekdaySchedule field to a fixed
	// list of HH:MM pickup times. Empty/absent = free time entry (the
	// historical behaviour). When set, every per-weekday time a parent
	// submits must be a member of this list. Only valid on a
	// weekday_schedule field; rejected on every other type.
	AllowedTimes []string `json:"allowed_times,omitempty"`
	// SingleModeGrades is the Heimweg-Beschränkung (#2381): target grade
	// levels whose children may pick at most ONE departure mode per weekday
	// on this weekday_multi_mode field. Parents still choose WHICH mode.
	// Empty/absent = unrestricted multi-select (the historical behaviour);
	// children without a target grade level are never restricted. Only valid
	// on the allowed-departure-modes field; rejected everywhere else.
	SingleModeGrades []int                `json:"single_mode_grades,omitempty"`
	Validation       *FormFieldValidation `json:"validation,omitempty"`
	FileOptions      *FormFileOptions     `json:"file_options,omitempty"` // only on FormFieldFile (#4783)
	SortOrder        int                  `json:"sort_order"`
	AppliesToCh      bool                 `json:"applies_to_child,omitempty"` // false (default) = guardian-level field; true = per-child field
	Target           string               `json:"target,omitempty"`           // "" = free custom field; otherwise one of ReservedTargets
	VisibleWhen      *VisibilityCondition `json:"visible_when,omitempty"`     // nil = always visible; otherwise show only when the condition matches
	Translations     Translations         `json:"translations,omitempty"`     // of Label, HelpText, Content (#3377); see form_schema_translations.go
}

const CoreRequirementGuardianPhone = "guardian_phone"

// CoreRequirements stores per-template required-state for core fields
// that are rendered from dedicated request/request_child columns rather
// than from FormSchema.Fields. A missing or false value means optional.
type CoreRequirements map[string]bool

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

// formFieldKeyMaxLength caps the length of a custom form field key.
const formFieldKeyMaxLength = 64

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

// WeekdayMode is the value of a FormFieldWeekdayMode field. Keys are weekday
// names (mon/tue/wed/thu/fri); values are alone/bus/pickup. A missing day (or
// "alone") means the child goes home alone that day.
type WeekdayMode map[string]string

// WeekdayMultiMode is the value of a FormFieldWeekdayMultiMode field. Each
// weekday contains every allowed departure mode for that care day.
type WeekdayMultiMode map[string][]string

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

// FormLegalBlock is one consent/legal row owned by a form template.
// Standard blocks originate from tenant settings and may be overridden or
// disabled on the template. Custom blocks are stored only on the template.
type FormLegalBlock struct {
	Attachments  []FormAttachment `json:"attachments,omitempty"`
	Key          string           `json:"key"`
	Kind         string           `json:"kind"`
	Title        string           `json:"title"`
	Label        string           `json:"label"`
	Text         string           `json:"text"`
	Required     bool             `json:"required"`
	Enabled      bool             `json:"enabled"`
	SortOrder    int              `json:"sort_order"`
	Source       string           `json:"source,omitempty"`
	DisplayMode  string           `json:"display_mode,omitempty"`
	DocumentURL  string           `json:"document_url,omitempty"`
	Translations Translations     `json:"translations,omitempty"` // of Title, Label, Text (#3377)
}

// FormSchema is a row in enrollment.form_schemas. Each save creates a new
// version; submissions pin to a specific schema_id so editing the active
// schema doesn't break already-submitted requests. Name groups versions
// of the same logical schema (e.g. "Schuljahr", "Ferienbetreuung") so
// admins can maintain multiple parallel form variants per tenant.
type FormSchema struct {
	ID               int64            `json:"id"`
	TenantID         int64            `json:"tenant_id"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	Name             string           `json:"name"`
	Version          int              `json:"version"`
	Fields           []FormField      `json:"fields"`
	CoreRequirements CoreRequirements `json:"core_requirements"`
	LegalBlocks      []FormLegalBlock `json:"legal_blocks"`
	IsActive         bool             `json:"is_active"`
	CreatedBy        int64            `json:"created_by"`
}

const (
	ConsentKeyAGB            = "agb"
	ConsentKeyDataProcessing = "data_processing"
	ConsentKeyEmailContact   = "email_contact"
	ConsentKeyPhoto          = "photo"
)
