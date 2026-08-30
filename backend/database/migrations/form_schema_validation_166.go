package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// These frozen wire types preserve the validation contract that migration
// 1.15.166 had when it shipped. A historical migration must not import the
// live enrollment model, because later model changes would change old schema
// migrations on fresh installations.
type migration166Field struct {
	Key              string                  `json:"key"`
	Label            string                  `json:"label"`
	Type             string                  `json:"type"`
	Required         bool                    `json:"required,omitempty"`
	Content          string                  `json:"content,omitempty"`
	Options          []map[string]any        `json:"options,omitempty"`
	AllowedTimes     []string                `json:"allowed_times,omitempty"`
	SingleModeGrades []int                   `json:"single_mode_grades,omitempty"`
	Validation       map[string]any          `json:"validation,omitempty"`
	AppliesToChild   bool                    `json:"applies_to_child,omitempty"`
	Target           string                  `json:"target,omitempty"`
	VisibleWhen      *migration166Visibility `json:"visible_when,omitempty"`
}

type migration166Visibility struct {
	Source   string `json:"source"`
	Field    string `json:"field,omitempty"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

type migration166LegalBlock struct {
	Key         string `json:"key"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Label       string `json:"label"`
	Required    bool   `json:"required"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source,omitempty"`
	DisplayMode string `json:"display_mode,omitempty"`
	DocumentURL string `json:"document_url,omitempty"`
}

var migration166FieldTypes = map[string]bool{
	"boolean": true, "number": true, "text": true, "textarea": true,
	"date": true, "select": true, "information": true, "phone_list": true,
	"weekday_schedule": true, "weekday_boolean": true, "weekday_mode": true,
	"weekday_multi_mode": true, "contact_list": true,
}

var migration166Targets = map[string]string{
	"student.health_info": "textarea", "student.extra_info": "textarea",
	"student.allowed_departure_modes": "weekday_multi_mode", "student.departure": "weekday_mode",
	"student.bus_days": "weekday_boolean", "student.bus": "weekday_boolean",
	"student.pickup_status": "weekday_boolean", "schedule.pickup": "weekday_schedule",
	"schedule.arrival": "weekday_schedule", "student.contacts": "contact_list",
}

var migration166StructuredTypes = map[string]bool{
	"phone_list": true, "weekday_schedule": true, "weekday_boolean": true,
	"weekday_mode": true, "weekday_multi_mode": true, "contact_list": true,
}

var migration166CoreKeys = map[string]bool{
	"guardian_first_name": true, "guardian_last_name": true, "guardian_email": true,
	"guardian_phone": true, "first_name": true, "last_name": true,
	"date_of_birth": true, "target_grade_level": true,
}

func validateConvertedDepartureSchema(row departureSchemaRow, fieldsJSON string) error {
	if row.Name == "" {
		return errors.New("form schema name is required")
	}
	if row.Version+1 <= 0 {
		return errors.New("form schema version must be positive")
	}
	if row.CreatedBy <= 0 {
		return errors.New("form schema created_by is required")
	}
	fields, err := decodeMigration166Schema(row, fieldsJSON)
	if err != nil {
		return err
	}
	byKey := make(map[string]*migration166Field, len(fields))
	targets := make(map[string]string, len(fields))
	modernFields := 0
	for index := range fields {
		field := &fields[index]
		if err := validateMigration166Field(field); err != nil {
			return fmt.Errorf("field %d: %w", index, err)
		}
		if byKey[field.Key] != nil {
			return fmt.Errorf("duplicate form field key %q", field.Key)
		}
		byKey[field.Key] = field
		if err := indexMigration166Target(field, targets); err != nil {
			return err
		}
		if field.Target == modernDepartureTarget {
			modernFields++
			if field.Type != "weekday_multi_mode" || !field.AppliesToChild {
				return errors.New("modern departure field has invalid type or scope")
			}
		}
	}
	if modernFields != 1 {
		return fmt.Errorf("converted schema has %d modern departure fields, want 1", modernFields)
	}
	for index := range fields {
		if err := validateMigration166VisibilityReference(&fields[index], byKey); err != nil {
			return err
		}
	}
	return nil
}

func decodeMigration166Schema(row departureSchemaRow, fieldsJSON string) ([]migration166Field, error) {
	var fields []migration166Field
	if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
		return nil, fmt.Errorf("failed decoding converted fields: %w", err)
	}
	var core map[string]bool
	if err := json.Unmarshal([]byte(row.CoreRequirements), &core); err != nil {
		return nil, fmt.Errorf("failed decoding core requirements: %w", err)
	}
	for key := range core {
		if key != "guardian_phone" {
			return nil, fmt.Errorf("unknown core requirement %q", key)
		}
	}
	var legal []migration166LegalBlock
	if err := json.Unmarshal([]byte(row.LegalBlocks), &legal); err != nil {
		return nil, fmt.Errorf("failed decoding legal blocks: %w", err)
	}
	if err := validateMigration166LegalBlocks(legal); err != nil {
		return nil, err
	}
	return fields, nil
}

func indexMigration166Target(field *migration166Field, targets map[string]string) error {
	if field.Target == "" {
		return nil
	}
	if first, exists := targets[field.Target]; exists {
		return fmt.Errorf("duplicate field target %q on fields %q and %q", field.Target, first, field.Key)
	}
	targets[field.Target] = field.Key
	return nil
}

func validateMigration166Field(field *migration166Field) error {
	field.Key = strings.TrimSpace(field.Key)
	if field.Key == "" {
		return errors.New("form field key is required")
	}
	if len(field.Key) > 64 {
		return errors.New("form field key must be at most 64 characters")
	}
	for _, r := range field.Key {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("form field key %q must be lowercase letters, digits, or underscores", field.Key)
		}
	}
	if migration166CoreKeys[field.Key] {
		return fmt.Errorf("form field key %q is reserved for a core field", field.Key)
	}
	if !migration166FieldTypes[field.Type] {
		return fmt.Errorf("unknown form field type %q", field.Type)
	}
	field.Label = strings.TrimSpace(field.Label)
	field.Content = strings.TrimSpace(field.Content)
	if field.Type == "information" {
		if field.Content == "" {
			return fmt.Errorf("information field %q requires content text", field.Key)
		}
		if field.Required || len(field.Options) > 0 || field.Target != "" || field.Validation != nil || len(field.AllowedTimes) > 0 || len(field.SingleModeGrades) > 0 {
			return fmt.Errorf("information field %q declares answer-only attributes", field.Key)
		}
	} else if err := validateMigration166Question(field); err != nil {
		return err
	}
	if field.VisibleWhen != nil {
		if err := validateMigration166Visibility(field.VisibleWhen, field.AppliesToChild); err != nil {
			return fmt.Errorf("form field %q visibility: %w", field.Key, err)
		}
	}
	return nil
}

func validateMigration166Question(field *migration166Field) error {
	if field.Label == "" {
		return errors.New("form field label is required")
	}
	if field.Content != "" {
		return fmt.Errorf("only information fields may declare content (field %q)", field.Key)
	}
	if field.Type == "select" && len(field.Options) == 0 {
		return fmt.Errorf("select field %q must declare at least one option", field.Key)
	}
	if field.Type != "select" && len(field.Options) > 0 {
		return fmt.Errorf("non-select field %q must not declare options", field.Key)
	}
	if err := validateMigration166AllowedTimes(field); err != nil {
		return err
	}
	if err := validateMigration166Grades(field); err != nil {
		return err
	}
	if migration166StructuredTypes[field.Type] && field.Target == "" {
		return fmt.Errorf("field type %q must declare a target", field.Type)
	}
	if field.Target != "" {
		requiredType, ok := migration166Targets[field.Target]
		if !ok {
			return fmt.Errorf("unknown form field target %q", field.Target)
		}
		if requiredType != field.Type {
			return fmt.Errorf("target %q requires type %q, got %q", field.Target, requiredType, field.Type)
		}
	}
	return nil
}

func validateMigration166AllowedTimes(field *migration166Field) error {
	if len(field.AllowedTimes) == 0 {
		return nil
	}
	if field.Target != "schedule.pickup" {
		return fmt.Errorf("field %q: allowed_times is only valid on the pickup-times field", field.Key)
	}
	seen := map[string]bool{}
	for _, value := range field.AllowedTimes {
		value = strings.TrimSpace(value)
		if _, err := time.Parse("15:04", value); value == "" || err != nil || seen[value] {
			return fmt.Errorf("field %q has invalid or duplicate allowed_times entry %q", field.Key, value)
		}
		seen[value] = true
	}
	return nil
}

func validateMigration166Grades(field *migration166Field) error {
	if len(field.SingleModeGrades) == 0 {
		return nil
	}
	if field.Target != modernDepartureTarget || !field.AppliesToChild {
		return fmt.Errorf("field %q: single_mode_grades requires the child allowed-departure-modes field", field.Key)
	}
	for _, grade := range field.SingleModeGrades {
		if grade < 1 || grade > 13 {
			return fmt.Errorf("field %q: single_mode_grades contains invalid grade %d", field.Key, grade)
		}
	}
	return nil
}

func validateMigration166Visibility(condition *migration166Visibility, appliesToChild bool) error {
	switch condition.Operator {
	case "eq", "neq", "not_empty", "includes":
	default:
		return fmt.Errorf("unknown visibility operator %q", condition.Operator)
	}
	if condition.Operator != "not_empty" {
		if condition.Value == nil || condition.Value == "" {
			return fmt.Errorf("visibility operator %q requires a value", condition.Operator)
		}
		switch condition.Value.(type) {
		case string, bool, float64:
		default:
			return fmt.Errorf("visibility condition value must be a string, number, or boolean, got %T", condition.Value)
		}
	}
	switch condition.Source {
	case "field":
		if strings.TrimSpace(condition.Field) == "" {
			return errors.New("field visibility condition requires the controlling field key")
		}
		if condition.Operator == "includes" {
			return errors.New("the 'includes' operator is only valid for care offering conditions")
		}
	case "grade_level":
		if !appliesToChild {
			return errors.New("grade level conditions are only valid on per-child fields")
		}
		if condition.Operator == "includes" {
			return errors.New("the 'includes' operator is only valid for care offering conditions")
		}
	case "care_offering":
		if !appliesToChild || condition.Operator != "includes" {
			return errors.New("care offering conditions require a child field and the 'includes' operator")
		}
	default:
		return fmt.Errorf("unknown visibility source %q", condition.Source)
	}
	return nil
}

func validateMigration166VisibilityReference(field *migration166Field, byKey map[string]*migration166Field) error {
	if field.VisibleWhen == nil || field.VisibleWhen.Source != "field" {
		return nil
	}
	ref := strings.TrimSpace(field.VisibleWhen.Field)
	if ref == field.Key {
		return fmt.Errorf("field %q cannot depend on itself", field.Key)
	}
	controller := byKey[ref]
	if controller == nil {
		return fmt.Errorf("field %q depends on unknown field %q", field.Key, ref)
	}
	if controller.Type != "boolean" && controller.Type != "select" {
		return fmt.Errorf("field %q can only depend on a yes/no or selection field, but %q is %q", field.Key, ref, controller.Type)
	}
	if !field.AppliesToChild && controller.AppliesToChild {
		return fmt.Errorf("parent-level field %q cannot depend on per-child field %q", field.Key, ref)
	}
	return nil
}

func validateMigration166LegalBlocks(blocks []migration166LegalBlock) error {
	seen := map[string]bool{}
	standard := map[string]bool{"agb": true, "data_processing": true, "email_contact": true, "photo": true}
	kinds := map[string]bool{"terms": true, "privacy_notice": true, "notice": true, "consent": true}
	for index, block := range blocks {
		if err := validateMigration166LegalBlock(index, block, seen, standard, kinds); err != nil {
			return err
		}
	}
	return nil
}

func validateMigration166LegalBlock(index int, block migration166LegalBlock, seen, standard, kinds map[string]bool) error {
	block.Key, block.Kind = strings.TrimSpace(block.Key), strings.TrimSpace(block.Kind)
	block.Title, block.Label = strings.TrimSpace(block.Title), strings.TrimSpace(block.Label)
	block.Source, block.DisplayMode = strings.TrimSpace(block.Source), strings.TrimSpace(block.DisplayMode)
	block.DocumentURL = strings.TrimSpace(block.DocumentURL)
	if block.Key == "" || len(block.Key) > 64 {
		return fmt.Errorf("legal block %d has an invalid key", index)
	}
	for _, r := range block.Key {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("legal block key %q must be lowercase letters, digits, or underscores", block.Key)
		}
	}
	if seen[block.Key] {
		return fmt.Errorf("duplicate legal block key %q", block.Key)
	}
	seen[block.Key] = true
	if block.Source == "" {
		if standard[block.Key] {
			block.Source = "standard"
		} else {
			block.Source = "custom"
		}
	}
	if block.Source != "standard" && block.Source != "custom" || block.Source == "standard" && !standard[block.Key] || block.Source == "custom" && standard[block.Key] {
		return fmt.Errorf("legal block %q has invalid source %q", block.Key, block.Source)
	}
	if !kinds[block.Kind] || block.Enabled && (block.Label == "" || block.Title == "") || block.Kind == "notice" && block.Required {
		return fmt.Errorf("legal block %q has invalid kind, label, title, or required state", block.Key)
	}
	if block.DisplayMode == "" {
		block.DisplayMode = "text"
	}
	if block.DisplayMode != "text" && block.DisplayMode != "pdf" {
		return fmt.Errorf("legal block %q has unknown display mode %q", block.Key, block.DisplayMode)
	}
	if block.DisplayMode == "pdf" && (block.Key != "agb" || block.Source != "standard" || block.Enabled && block.DocumentURL == "") {
		return fmt.Errorf("legal block %q has invalid PDF configuration", block.Key)
	}
	return nil
}
