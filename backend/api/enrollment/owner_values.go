package enrollment

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The Enrollment owner hands its requests, children and offerings over with
// the family's answers, consent flags and availability rules as raw JSON
// (#3565). These routes render them from the decoded values below.

// RequestChild is a decoded child of a parent submission.
type RequestChild struct {
	ID                    int64          `json:"id"`
	TenantID              int64          `json:"tenant_id"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	RequestID             int64          `json:"request_id"`
	FirstName             string         `json:"first_name"`
	LastName              string         `json:"last_name"`
	DateOfBirth           timezone.Date  `json:"date_of_birth"`
	TargetGradeLevel      *int16         `json:"target_grade_level,omitempty"`
	TargetSchoolClass     *string        `json:"target_school_class,omitempty"`
	CustomData            map[string]any `json:"custom_data"`
	Status                string         `json:"status"`
	StatusReason          *string        `json:"status_reason,omitempty"`
	ActivationMode        string         `json:"activation_mode"`
	ActivateOn            *timezone.Date `json:"activate_on,omitempty"`
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
	dob, err := timezone.ParseDate(string(r.DateOfBirth))
	if err != nil {
		return nil, err
	}
	result.DateOfBirth = dob
	if r.ActivateOn != nil {
		date, err := timezone.ParseDate(string(*r.ActivateOn))
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

func requestValue(r *capability.Request) (*enrollmentModels.Request, error) {
	if r == nil {
		return nil, nil
	}
	result := &enrollmentModels.Request{
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

// careOfferingValue decodes a care offering the owner serves in its form
// loads into the enrollment row these routes render.
func careOfferingValue(o *capability.CareOffering) (*enrollmentModels.CareOffering, error) {
	row := &enrollmentModels.CareOffering{
		ID: o.ID, TenantID: o.TenantID, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt, PhaseID: o.PhaseID,
		ActivityGroupID: o.ActivityGroupID, Name: o.Name, Description: o.Description,
		DaysOfWeekMode: o.DaysOfWeekMode, AvailableDays: o.AvailableDays,
		IncludesHolidayCare: o.IncludesHolidayCare, IncludesLunch: o.IncludesLunch,
		Capacity: o.Capacity, PriceCents: o.PriceCents, IsActive: o.IsActive, IsRequired: o.IsRequired,
		CountsAsCare: o.CountsAsCare, CountsAsCareSet: true, AutoAddGradeLevels: o.AutoAddGradeLevels,
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

func careOfferingValues(values []*capability.CareOffering) ([]*enrollmentModels.CareOffering, error) {
	out := make([]*enrollmentModels.CareOffering, 0, len(values))
	for _, value := range values {
		row, err := careOfferingValue(value)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// requestStatusInput encodes a request and its children back into the
// owner's values, so the owner can judge the edit mode of a status it
// handed out.
func requestStatusInput(req *enrollmentModels.Request, children []*RequestChild) (*capability.RequestStatus, error) {
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
