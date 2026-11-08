package enrollmenthttp

import (
	"encoding/json"
	"maps"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The Enrollment owner hands its offerings over with their availability rules
// as raw JSON (#3565). These routes render them from the decoded values below.
// Requests and children travel as the owner's values with decoded answers
// (ADR 0170).

// CareOffering is a decoded care offering: the owner's value with its
// availability rule decoded for rendering.
type CareOffering struct {
	ID                        int64                         `json:"id"`
	TenantID                  int64                         `json:"tenant_id"`
	CreatedAt                 time.Time                     `json:"created_at"`
	UpdatedAt                 time.Time                     `json:"updated_at"`
	PhaseID                   int64                         `json:"phase_id"`
	ActivityGroupID           *int64                        `json:"activity_group_id,omitempty"`
	Name                      string                        `json:"name"`
	Description               *string                       `json:"description,omitempty"`
	DaysOfWeekMode            string                        `json:"days_of_week_mode"`
	AvailableDays             []string                      `json:"available_days"`
	IncludesHolidayCare       bool                          `json:"includes_holiday_care"`
	IncludesLunch             bool                          `json:"includes_lunch"`
	Capacity                  *int                          `json:"capacity,omitempty"`
	PriceCents                *int                          `json:"price_cents,omitempty"`
	IsActive                  bool                          `json:"is_active"`
	IsRequired                bool                          `json:"is_required"`
	CountsAsCare              bool                          `json:"counts_as_care"`
	AutoAddGradeLevels        []int                         `json:"auto_add_grade_levels"`
	AvailabilityRule          *CareOfferingAvailabilityRule `json:"availability_rule,omitempty"`
	SortOrder                 int                           `json:"sort_order"`
	SelectionGroup            string                        `json:"selection_group,omitempty"`
	SelectionRule             string                        `json:"selection_rule"`
	PickupTimes               map[string]string             `json:"pickup_times,omitempty"`
	Translations              json.RawMessage               `json:"translations,omitempty"`
	AutoAddTriggerOfferingIDs []int64                       `json:"auto_add_trigger_offering_ids,omitempty"`
}

// CareOfferingAvailabilityRule limits an offering to the children whose
// grade level matches; the owner validates and stores it.
type CareOfferingAvailabilityRule struct {
	Match      string                              `json:"match"`
	Conditions []CareOfferingAvailabilityCondition `json:"conditions"`
}

// CareOfferingAvailabilityCondition is one condition of an availability rule.
type CareOfferingAvailabilityCondition struct {
	Source   string `json:"source"`
	Operator string `json:"operator"`
	Value    []int  `json:"value"`
}

// RequestChildOffering is one child's offering selection with its validity.
type RequestChildOffering = capability.RequestChildOfferingRecord

// ChildTakenOver reports whether an enrollment child has already been taken
// over into care: an approval created its student. Change wishes for it run
// through the parent app, so the status link keeps it readable but locked
// (ADR 0003).
func ChildTakenOver(child *capability.RequestChild) bool {
	return child != nil && child.CreatedStudentID != nil && *child.CreatedStudentID > 0
}

func decodeJSONObject(raw json.RawMessage, into any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, into)
}

// careOfferingValue decodes a care offering the owner serves for these
// routes to render.
func careOfferingValue(o *capability.CareOffering) (*CareOffering, error) {
	row := &CareOffering{
		ID: o.ID, TenantID: o.TenantID, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt, PhaseID: o.PhaseID,
		ActivityGroupID: o.ActivityGroupID, Name: o.Name, Description: o.Description,
		DaysOfWeekMode: o.DaysOfWeekMode, AvailableDays: o.AvailableDays,
		IncludesHolidayCare: o.IncludesHolidayCare, IncludesLunch: o.IncludesLunch,
		Capacity: o.Capacity, PriceCents: o.PriceCents, IsActive: o.IsActive, IsRequired: o.IsRequired,
		CountsAsCare: o.CountsAsCare, AutoAddGradeLevels: o.AutoAddGradeLevels,
		SortOrder: o.SortOrder, SelectionGroup: o.SelectionGroup, SelectionRule: o.SelectionRule,
		PickupTimes: o.PickupTimes, Translations: o.Translations, AutoAddTriggerOfferingIDs: o.AutoAddTriggerOfferingIDs,
	}
	if len(o.AvailabilityRule) > 0 && string(o.AvailabilityRule) != "null" {
		if err := json.Unmarshal(o.AvailabilityRule, &row.AvailabilityRule); err != nil {
			return nil, err
		}
	}
	return row, nil
}

func careOfferingValues(values []*capability.CareOffering) ([]*CareOffering, error) {
	out := make([]*CareOffering, 0, len(values))
	for _, value := range values {
		row, err := careOfferingValue(value)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// careOfferingInput encodes a care offering a route built back into the
// owner's value, its availability rule as the owner stores it.
func careOfferingInput(o *CareOffering) (*capability.CareOffering, error) {
	value := &capability.CareOffering{
		ID: o.ID, TenantID: o.TenantID, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt, PhaseID: o.PhaseID,
		ActivityGroupID: o.ActivityGroupID, Name: o.Name, Description: o.Description,
		DaysOfWeekMode: o.DaysOfWeekMode, AvailableDays: o.AvailableDays,
		IncludesHolidayCare: o.IncludesHolidayCare, IncludesLunch: o.IncludesLunch,
		Capacity: o.Capacity, PriceCents: o.PriceCents, IsActive: o.IsActive, IsRequired: o.IsRequired,
		CountsAsCare: o.CountsAsCare, AutoAddGradeLevels: o.AutoAddGradeLevels,
		SortOrder: o.SortOrder, SelectionGroup: o.SelectionGroup, SelectionRule: o.SelectionRule,
		PickupTimes: o.PickupTimes, Translations: o.Translations, AutoAddTriggerOfferingIDs: o.AutoAddTriggerOfferingIDs,
	}
	if o.AvailabilityRule != nil {
		rule, err := json.Marshal(o.AvailabilityRule)
		if err != nil {
			return nil, err
		}
		value.AvailabilityRule = rule
	}
	return value, nil
}

// formValues copies the answers a wire request decoded into the owner's
// values.
func formValues(values map[string]any) capability.FormValues {
	return capability.CollectFormValues(maps.All(values))
}

// patchFormValues is a patched document: nil leaves the stored one alone,
// any other map, even an empty one, replaces it.
func patchFormValues(values map[string]any) *capability.FormValues {
	if values == nil {
		return nil
	}
	return new(formValues(values))
}

// formValue is the answer stored under key, nil when absent.
func formValue(values capability.FormValues, key string) any {
	value, _ := values.Lookup(key)
	return value
}

// deref returns the value p points at, or the zero value for nil.
func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
