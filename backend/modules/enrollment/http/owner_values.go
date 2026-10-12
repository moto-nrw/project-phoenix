package enrollmenthttp

import (
	"encoding/json"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Enrollment owner hands its requests, children and offerings over with
// the family's answers, consent flags and availability rules as raw JSON
// (#3565). These routes render them from the decoded values below.

// Request is a decoded parent submission: the owner's request with the
// family's consent flags, answers, source metadata and consent evidence
// decoded. The two internal fields stay out of every response, as they do
// in the owner's value.
type Request struct {
	ID                       int64                                 `json:"id"`
	TenantID                 int64                                 `json:"tenant_id"`
	CreatedAt                time.Time                             `json:"created_at"`
	UpdatedAt                time.Time                             `json:"updated_at"`
	SchemaID                 *int64                                `json:"schema_id,omitempty"`
	PhaseID                  int64                                 `json:"phase_id"`
	GuardianFirstName        string                                `json:"guardian_first_name"`
	GuardianLastName         string                                `json:"guardian_last_name"`
	GuardianEmail            string                                `json:"guardian_email"`
	GuardianPhone            *string                               `json:"guardian_phone,omitempty"`
	GuardianAccountID        *int64                                `json:"guardian_account_id,omitempty"`
	ConsentFlags             map[string]any                        `json:"consent_flags"`
	LegalBlocksSnapshot      []capability.LegalBlocksSnapshotEntry `json:"-"`
	CustomData               map[string]any                        `json:"custom_data"`
	SubmissionSource         string                                `json:"submission_source"`
	SourceMetadata           map[string]any                        `json:"source_metadata"`
	StatusToken              string                                `json:"status_token"`
	StatusTokenExpires       *time.Time                            `json:"status_token_expires,omitempty"`
	SubmittedAt              time.Time                             `json:"submitted_at"`
	WithdrawnAt              *time.Time                            `json:"withdrawn_at,omitempty"`
	DecisionNotificationMode *string                               `json:"-"`
}

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

// RequestChild is a decoded child of a parent submission.
type RequestChild struct {
	ID                    int64          `json:"id"`
	TenantID              int64          `json:"tenant_id"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	RequestID             int64          `json:"request_id"`
	FirstName             string         `json:"first_name"`
	LastName              string         `json:"last_name"`
	DateOfBirth           calendar.Date  `json:"date_of_birth"`
	TargetGradeLevel      *int16         `json:"target_grade_level,omitempty"`
	TargetSchoolClass     *string        `json:"target_school_class,omitempty"`
	CustomData            map[string]any `json:"custom_data"`
	Status                string         `json:"status"`
	StatusReason          *string        `json:"status_reason,omitempty"`
	ActivationMode        string         `json:"activation_mode"`
	ActivateOn            *calendar.Date `json:"activate_on,omitempty"`
	ReviewedAt            *time.Time     `json:"reviewed_at,omitempty"`
	ReviewedBy            *int64         `json:"reviewed_by,omitempty"`
	CreatedStudentID      *int64         `json:"created_student_id,omitempty"`
	MatchedStudentID      *int64         `json:"matched_student_id,omitempty"`
	SortOrder             int            `json:"sort_order"`
	RolloverSourceChildID *int64         `json:"rollover_source_child_id,omitempty"`
	ReviewReason          *string        `json:"review_reason,omitempty"`
}

// RequestChildOffering is one child's offering selection with its validity.
type RequestChildOffering = capability.RequestChildOfferingRecord

// ChildTakenOver reports whether an enrollment child has already been taken
// over into care: an approval created its student. Change wishes for it run
// through the parent app, so the status link keeps it readable but locked
// (ADR 0003).
func ChildTakenOver(child *RequestChild) bool {
	return child != nil && child.CreatedStudentID != nil && *child.CreatedStudentID > 0
}

func decodeJSONObject(raw json.RawMessage, into any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, into)
}

func encodeJSONObject(value map[string]any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func childValue(r *capability.RequestChild) (*RequestChild, error) {
	if r == nil {
		return nil, nil
	}
	result := &RequestChild{
		ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestID: r.RequestID,
		FirstName: r.FirstName, LastName: r.LastName, TargetGradeLevel: r.TargetGradeLevel,
		TargetSchoolClass: r.TargetSchoolClass, Status: r.Status, StatusReason: r.StatusReason,
		ActivationMode: r.ActivationMode, ReviewedAt: r.ReviewedAt, ReviewedBy: r.ReviewedBy,
		CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID, SortOrder: r.SortOrder,
		RolloverSourceChildID: r.RolloverSourceChildID, ReviewReason: r.ReviewReason,
	}
	dob, err := calendar.ParseDate(string(r.DateOfBirth))
	if err != nil {
		return nil, err
	}
	result.DateOfBirth = dob
	if r.ActivateOn != nil {
		date, err := calendar.ParseDate(string(*r.ActivateOn))
		if err != nil {
			return nil, err
		}
		result.ActivateOn = &date
	}
	if err := decodeJSONObject(r.CustomData, &result.CustomData); err != nil {
		return nil, err
	}
	return result, nil
}

func childValues(values []*capability.RequestChild) ([]*RequestChild, error) {
	var result []*RequestChild
	for _, value := range values {
		converted, err := childValue(value)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

func requestValue(r *capability.Request) (*Request, error) {
	if r == nil {
		return nil, nil
	}
	result := &Request{
		ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, SchemaID: r.SchemaID,
		PhaseID: r.PhaseID, GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName,
		GuardianEmail: r.GuardianEmail, GuardianPhone: r.GuardianPhone, GuardianAccountID: r.GuardianAccountID,
		SubmissionSource: r.SubmissionSource, StatusToken: r.StatusToken, StatusTokenExpires: r.StatusTokenExpires,
		SubmittedAt: r.SubmittedAt, WithdrawnAt: r.WithdrawnAt, DecisionNotificationMode: r.DecisionNotificationMode,
	}
	for _, field := range []struct {
		raw  json.RawMessage
		into any
	}{
		{r.ConsentFlags, &result.ConsentFlags},
		{r.LegalBlocksSnapshot, &result.LegalBlocksSnapshot},
		{r.CustomData, &result.CustomData},
		{r.SourceMetadata, &result.SourceMetadata},
	} {
		if err := decodeJSONObject(field.raw, field.into); err != nil {
			return nil, err
		}
	}
	return result, nil
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

// requestStatusInput encodes a request and its children back into the
// owner's values, so the owner can judge the edit mode of a status it
// handed out.
func requestStatusInput(req *Request, children []*RequestChild) (*capability.RequestStatus, error) {
	request := &capability.Request{
		ID: req.ID, TenantID: req.TenantID, CreatedAt: req.CreatedAt, UpdatedAt: req.UpdatedAt,
		SchemaID: req.SchemaID, PhaseID: req.PhaseID, GuardianFirstName: req.GuardianFirstName,
		GuardianLastName: req.GuardianLastName, GuardianEmail: req.GuardianEmail, GuardianPhone: req.GuardianPhone,
		GuardianAccountID: req.GuardianAccountID, SubmissionSource: req.SubmissionSource,
		StatusToken: req.StatusToken, StatusTokenExpires: req.StatusTokenExpires, SubmittedAt: req.SubmittedAt,
		WithdrawnAt: req.WithdrawnAt, DecisionNotificationMode: req.DecisionNotificationMode,
	}
	var err error
	if request.ConsentFlags, err = encodeJSONObject(req.ConsentFlags); err != nil {
		return nil, err
	}
	if request.CustomData, err = encodeJSONObject(req.CustomData); err != nil {
		return nil, err
	}
	status := &capability.RequestStatus{Request: request}
	for _, child := range children {
		value, err := childInput(child)
		if err != nil {
			return nil, err
		}
		status.Children = append(status.Children, value)
	}
	return status, nil
}

func childInput(r *RequestChild) (*capability.RequestChild, error) {
	if r == nil {
		return nil, nil
	}
	result := &capability.RequestChild{
		ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestID: r.RequestID,
		FirstName: r.FirstName, LastName: r.LastName, DateOfBirth: capability.Date(r.DateOfBirth.String()),
		TargetGradeLevel: r.TargetGradeLevel, TargetSchoolClass: r.TargetSchoolClass, Status: r.Status,
		StatusReason: r.StatusReason, ActivationMode: r.ActivationMode, ReviewedAt: r.ReviewedAt,
		ReviewedBy: r.ReviewedBy, CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID,
		SortOrder: r.SortOrder, RolloverSourceChildID: r.RolloverSourceChildID, ReviewReason: r.ReviewReason,
	}
	if r.ActivateOn != nil {
		date := capability.Date(r.ActivateOn.String())
		result.ActivateOn = &date
	}
	data, err := encodeJSONObject(r.CustomData)
	if err != nil {
		return nil, err
	}
	result.CustomData = data
	return result, nil
}

// deref returns the value p points at, or the zero value for nil.
func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
