// Package enrollment holds domain entities for the parent-enrollment
// feature. The schema is split across several tables - form_schemas
// (versioned form definitions), requests (parent submissions),
// request_children (per-child decisions), and (PR 6) care_offerings +
// request_child_offerings.
//
// PR 5 ships form_schemas, requests, request_children, plus the
// platform.email_outbox table which lives outside this package because
// it's shared across features.
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

	// Structured types — admins cannot define their internal shape;
	// the renderer + decision service know how to interpret them.
	// Each pairs with a specific FormField.Target (see ReservedTargets).
	FormFieldPhoneList       FormFieldType = "phone_list"       // 0..N labelled phone numbers
	FormFieldWeekdaySchedule FormFieldType = "weekday_schedule" // mon..fri → HH:MM, optional per day
	FormFieldContactList     FormFieldType = "contact_list"     // 0..N people (name + phones + flags)
)

// validFormFieldTypes is the set of accepted FormFieldType values.
var validFormFieldTypes = map[FormFieldType]bool{
	FormFieldBoolean:         true,
	FormFieldNumber:          true,
	FormFieldText:            true,
	FormFieldTextarea:        true,
	FormFieldDate:            true,
	FormFieldSelect:          true,
	FormFieldPhoneList:       true,
	FormFieldWeekdaySchedule: true,
	FormFieldContactList:     true,
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
	Key         string               `json:"key"`
	Label       string               `json:"label"`
	Type        FormFieldType        `json:"type"`
	Required    bool                 `json:"required,omitempty"`
	HelpText    string               `json:"help_text,omitempty"`
	Options     []FormFieldOption    `json:"options,omitempty"`
	Validation  *FormFieldValidation `json:"validation,omitempty"`
	SortOrder   int                  `json:"sort_order"`
	AppliesToCh bool                 `json:"applies_to_child,omitempty"` // false (default) = guardian-level field; true = per-child field
	Target      string               `json:"target,omitempty"`           // "" = free custom field; otherwise one of ReservedTargets
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
	TargetStudentHealthInfo   = "student.health_info"
	TargetStudentExtraInfo    = "student.extra_info"
	TargetStudentBus          = "student.bus"
	TargetStudentPickupStatus = "student.pickup_status"
	TargetSchedulePickup      = "schedule.pickup"
	TargetScheduleArrival     = "schedule.arrival"
	TargetStudentContacts     = "student.contacts"
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
	TargetStudentHealthInfo:   {Type: FormFieldTextarea, AppliesToChild: true, Label: "Gesundheitsinformationen"},
	TargetStudentExtraInfo:    {Type: FormFieldTextarea, AppliesToChild: true, Label: "Hinweise an die Betreuung"},
	TargetStudentBus:          {Type: FormFieldBoolean, AppliesToChild: true, Label: "Buskind"},
	TargetStudentPickupStatus: {Type: FormFieldSelect, AppliesToChild: true, Label: "Abholregelung"},
	TargetSchedulePickup:      {Type: FormFieldWeekdaySchedule, AppliesToChild: true, Label: "Abholzeiten"},
	TargetScheduleArrival:     {Type: FormFieldWeekdaySchedule, AppliesToChild: true, Label: "Ankunftszeiten"},
	TargetStudentContacts:     {Type: FormFieldContactList, AppliesToChild: true, Label: "Weitere Kontakte / Abholberechtigte / Notfallkontakte"},
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
		return errors.New("form field key is required")
	}
	if len(f.Key) > 64 {
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

	f.Label = strings.TrimSpace(f.Label)
	if f.Label == "" {
		return errors.New("form field label is required")
	}

	if !validFormFieldTypes[f.Type] {
		return fmt.Errorf("unknown form field type %q", f.Type)
	}

	if f.Type == FormFieldSelect && len(f.Options) == 0 {
		return fmt.Errorf("select field %q must declare at least one option", f.Key)
	}
	if f.Type != FormFieldSelect && len(f.Options) > 0 {
		return fmt.Errorf("non-select field %q must not declare options", f.Key)
	}

	// Structured types are only meaningful when paired with their
	// canonical target. They have no free-form variant — admins can't
	// invent a "phone_list" with their own semantics.
	if isStructuredFieldType(f.Type) && f.Target == "" {
		return fmt.Errorf("field type %q must declare a target", f.Type)
	}

	// Target consistency: when set, the target dictates the type so
	// the decision service can dispatch without ambiguity. Admin
	// editor enforces this too; this check is the backstop.
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
	case FormFieldPhoneList, FormFieldWeekdaySchedule, FormFieldContactList:
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

// FormSchema is a row in enrollment.form_schemas. Each save creates a new
// version; submissions pin to a specific schema_id so editing the active
// schema doesn't break already-submitted requests. Name groups versions
// of the same logical schema (e.g. "Schuljahr", "Ferienbetreuung") so
// admins can maintain multiple parallel form variants per tenant.
type FormSchema struct {
	base.Model `bun:"schema:enrollment,table:form_schemas"`
	base.TenantModel
	Name      string      `bun:"name,notnull,default:''" json:"name"`
	Version   int         `bun:"version,notnull" json:"version"`
	Fields    []FormField `bun:"fields,type:jsonb,notnull,default:'[]'" json:"fields"`
	IsActive  bool        `bun:"is_active,notnull,default:false" json:"is_active"`
	CreatedBy int64       `bun:"created_by,notnull" json:"created_by"`
}

// TableName returns the schema-qualified table name.
func (s *FormSchema) TableName() string {
	return "enrollment.form_schemas"
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
	seenKeys := make(map[string]bool, len(s.Fields))
	for i := range s.Fields {
		if err := s.Fields[i].Validate(); err != nil {
			return fmt.Errorf("field %d: %w", i, err)
		}
		if seenKeys[s.Fields[i].Key] {
			return fmt.Errorf("duplicate form field key %q", s.Fields[i].Key)
		}
		seenKeys[s.Fields[i].Key] = true
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
	DeactivatePrevious(ctx context.Context) error
	UpdateActiveFlag(ctx context.Context, id int64, isActive bool) error
}

// SubmissionData is the shape submitted by parents. PR 7 will validate
// this against the schema's fields. Defined here so PR 5's schema-service
// validation helper can stay strongly typed.
type SubmissionData struct {
	GuardianFields map[string]any `json:"guardian_fields"`
	ChildFields    map[string]any `json:"child_fields"` // keyed by child index → field map; PR 7 fills in
}

// _ keeps `time` imported even when the model doesn't reference time.Time
// directly (the base.Model embeds CreatedAt/UpdatedAt).
var _ = time.Time{}
