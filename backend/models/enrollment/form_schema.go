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
)

// validFormFieldTypes is the set of accepted FormFieldType values.
var validFormFieldTypes = map[FormFieldType]bool{
	FormFieldBoolean:  true,
	FormFieldNumber:   true,
	FormFieldText:     true,
	FormFieldTextarea: true,
	FormFieldDate:     true,
	FormFieldSelect:   true,
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

	return nil
}

// FormSchema is a row in enrollment.form_schemas. Each save creates a new
// version; submissions pin to a specific schema_id so editing the active
// schema doesn't break already-submitted requests.
type FormSchema struct {
	base.Model `bun:"schema:enrollment,table:form_schemas"`
	base.TenantModel
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
