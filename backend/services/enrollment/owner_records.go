package enrollment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The request, decision, change-request and rollover flows still work on the
// decoded request and child values: their answers and consent flags are
// maps. Enrollment's public contract carries no map-shaped values and its
// application package is internal to the module, so the decoding stays with
// these flows until they move into the owner themselves (#3564, #3565). The
// typed reads they share with the owner (offering selections, care periods,
// window checks, notifications) are published by modules/enrollment.

// RequestChild is the legacy service value for a child in a parent submission.
// The Enrollment owner persists it through its child adapter. Each child has
// an independent status; the parent request's status is derived from its children.
type RequestChild struct {
	ID               int64         `json:"id"`
	TenantID         int64         `json:"tenant_id"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	RequestID        int64         `json:"request_id"`
	FirstName        string        `json:"first_name"`
	LastName         string        `json:"last_name"`
	DateOfBirth      timezone.Date `json:"date_of_birth"`
	TargetGradeLevel *int16        `json:"target_grade_level,omitempty"`
	// TargetSchoolClass is the concrete future class (e.g. "2a") chosen at
	// enrollment (migration 1.15.172, issue #1833). NULL/empty means
	// grade-only ("Klasse offen"); on approval a non-empty value lands
	// verbatim in users.students.school_class, otherwise the grade-derived
	// fallback is used. Only collected for grade >= 2 when the tenant
	// setting enrollment.collect_school_class is on.
	TargetSchoolClass *string        `json:"target_school_class,omitempty"`
	CustomData        map[string]any `json:"custom_data"`
	Status            string         `json:"status"`
	StatusReason      *string        `json:"status_reason,omitempty"`
	ActivationMode    string         `json:"activation_mode"`
	ActivateOn        *timezone.Date `json:"activate_on,omitempty"`
	ReviewedAt        *time.Time     `json:"reviewed_at,omitempty"`
	ReviewedBy        *int64         `json:"reviewed_by,omitempty"`
	CreatedStudentID  *int64         `json:"created_student_id,omitempty"`
	// MatchedStudentID (migration 1.15.221) — set only for existing_students
	// phases: the already-enrolled student this child was matched to at
	// submission (unambiguous name+birthday lookup). On approval the decision
	// service renews that student instead of creating a duplicate
	// Person/Student. NULL for every other audience and when the submission
	// matched zero or more than one enrolled student (ambiguous → left to the
	// fresh-create path). FK ON DELETE SET NULL: a student deleted before
	// approval clears the reference.
	MatchedStudentID *int64 `json:"matched_student_id,omitempty"`
	SortOrder        int    `json:"sort_order"`

	// Rollover columns (migration 1.15.62). NULL on rows created via
	// the public form; set by RolloverService when a previous-year
	// approved child is carried forward into a new phase.
	//
	// RolloverSourceChildID — the previous-year row this one was
	// derived from. Unique index ensures one-to-one mapping.
	// ReviewReason — populated only when Status == pending_admin_review;
	// drives the localised label in the admin review queue.
	RolloverSourceChildID *int64  `json:"rollover_source_child_id,omitempty"`
	ReviewReason          *string `json:"review_reason,omitempty"`
}

func offeringChildByID(ctx context.Context, owner ChildIDReader, id int64) (*RequestChild, error) {
	value, err := owner.ChildByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return intakeChildValue(value)
}
func createIntakeChild(ctx context.Context, owner ChildCreator, child *RequestChild) error {
	value, err := intakeChildInput(child)
	if err != nil {
		return err
	}
	if err := owner.InsertChild(ctx, value); err != nil {
		return err
	}
	result, err := intakeChildValue(value)
	if err != nil {
		return err
	}
	*child = *result
	return nil
}
func updateIntakeChild(ctx context.Context, owner IntakeChildren, child *RequestChild) error {
	value, err := intakeChildInput(child)
	if err != nil {
		return err
	}
	return owner.UpdateChildData(ctx, value)
}
func listIntakeChildren(ctx context.Context, owner RequestChildrenReader, requestID int64, forUpdate bool) ([]*RequestChild, error) {
	values, err := owner.ChildrenForRequest(ctx, requestID, forUpdate)
	if err != nil {
		return nil, err
	}
	return intakeChildValues(values)
}
func intakeChildInput(r *RequestChild) (*capability.RequestChild, error) {
	if r == nil {
		return nil, nil
	}
	result := &capability.RequestChild{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestID: r.RequestID, FirstName: r.FirstName, LastName: r.LastName, TargetGradeLevel: r.TargetGradeLevel, TargetSchoolClass: r.TargetSchoolClass, Status: r.Status, StatusReason: r.StatusReason, ActivationMode: r.ActivationMode, ReviewedAt: r.ReviewedAt, ReviewedBy: r.ReviewedBy, CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID, SortOrder: r.SortOrder, RolloverSourceChildID: r.RolloverSourceChildID, ReviewReason: r.ReviewReason}
	result.DateOfBirth = capability.Date(r.DateOfBirth.String())
	if r.ActivateOn != nil {
		date := capability.Date(r.ActivateOn.String())
		result.ActivateOn = &date
	}
	data, err := json.Marshal(r.CustomData)
	if err != nil {
		return nil, err
	}
	result.CustomData = data
	return result, nil
}
func intakeChildValue(r *capability.RequestChild) (*RequestChild, error) {
	if r == nil {
		return nil, nil
	}
	result := &RequestChild{}
	result.ID = r.ID
	result.TenantID = r.TenantID
	result.CreatedAt = r.CreatedAt
	result.UpdatedAt = r.UpdatedAt
	result.RequestID = r.RequestID
	result.FirstName = r.FirstName
	result.LastName = r.LastName
	result.TargetGradeLevel = r.TargetGradeLevel
	result.TargetSchoolClass = r.TargetSchoolClass
	result.Status = r.Status
	result.StatusReason = r.StatusReason
	result.ActivationMode = r.ActivationMode
	result.ReviewedAt = r.ReviewedAt
	result.ReviewedBy = r.ReviewedBy
	result.CreatedStudentID = r.CreatedStudentID
	result.MatchedStudentID = r.MatchedStudentID
	result.SortOrder = r.SortOrder
	result.RolloverSourceChildID = r.RolloverSourceChildID
	result.ReviewReason = r.ReviewReason
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
	if len(r.CustomData) > 0 {
		if err := json.Unmarshal(r.CustomData, &result.CustomData); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func intakeChildValues(values []*capability.RequestChild) ([]*RequestChild, error) {
	var result []*RequestChild
	for _, value := range values {
		converted, err := intakeChildValue(value)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

// Enrollment owner reads of the decision flow: the admin request list, the
// children of requests with their offering selections, and the phases.
type (
	ReportRequests interface {
		AdminRequests(context.Context, capability.RequestListFilters) ([]*capability.Request, error)
	}
	ReportChildren interface {
		RequestChildOfferingsForChildrenAtDate(context.Context, []int64, capability.Date) ([]*capability.RequestChildOffering, error)
		ChildrenForRequests(context.Context, []int64) ([]*capability.RequestChild, error)
	}
	PhaseReader interface {
		Phase(context.Context, int64) (*capability.Phase, error)
		Phases(context.Context) ([]*capability.Phase, error)
	}
)

func listIntakeChildrenForRequests(ctx context.Context, owner ReportChildren, requestIDs []int64) ([]*RequestChild, error) {
	values, err := owner.ChildrenForRequests(ctx, requestIDs)
	if err != nil {
		return nil, err
	}
	return intakeChildValues(values)
}

func createIntakeRequest(ctx context.Context, owner RequestCreator, req *enrollmentModels.Request) error {
	value, err := intakeRequestInput(req)
	if err != nil {
		return err
	}
	if err := owner.InsertRequest(ctx, value); err != nil {
		return err
	}
	result, err := intakeRequestValue(value)
	if err != nil {
		return err
	}
	*req = *result
	return nil
}
func updateIntakeRequest(ctx context.Context, owner IntakeRequests, req *enrollmentModels.Request, includeEmail bool) error {
	value, err := intakeRequestInput(req)
	if err != nil {
		return err
	}
	return owner.UpdateRequestGuardian(ctx, value, includeEmail)
}
func intakeRequestByID(ctx context.Context, owner RequestIDReader, id int64, lock bool) (*enrollmentModels.Request, error) {
	value, err := owner.RequestByID(ctx, id, lock)
	if err != nil {
		return nil, err
	}
	return intakeRequestValue(value)
}
func intakeRequestByToken(ctx context.Context, owner IntakeRequests, token string, lock bool) (*enrollmentModels.Request, error) {
	value, err := owner.RequestByToken(ctx, token, lock)
	if err != nil {
		return nil, err
	}
	return intakeRequestValue(value)
}
func intakeRequestsByID(ctx context.Context, owner RequestBatchReader, ids []int64) ([]*enrollmentModels.Request, error) {
	values, err := owner.RequestsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return intakeRequestValues(values)
}
func intakeRequestInput(r *enrollmentModels.Request) (*capability.Request, error) {
	if r == nil {
		return nil, nil
	}
	result := &capability.Request{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, SchemaID: r.SchemaID, PhaseID: r.PhaseID, GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName, GuardianEmail: r.GuardianEmail, GuardianPhone: r.GuardianPhone, GuardianAccountID: r.GuardianAccountID, SubmissionSource: r.SubmissionSource, StatusToken: r.StatusToken, StatusTokenExpires: r.StatusTokenExpires, SubmittedAt: r.SubmittedAt, WithdrawnAt: r.WithdrawnAt, DecisionNotificationMode: r.DecisionNotificationMode}
	var err error
	result.ConsentFlags, err = json.Marshal(r.ConsentFlags)
	if err != nil {
		return nil, err
	}
	result.LegalBlocksSnapshot, err = json.Marshal(r.LegalBlocksSnapshot)
	if err != nil {
		return nil, err
	}
	result.CustomData, err = json.Marshal(r.CustomData)
	if err != nil {
		return nil, err
	}
	result.SourceMetadata, err = json.Marshal(r.SourceMetadata)
	if err != nil {
		return nil, err
	}
	return result, nil
}
func intakeRequestValue(r *capability.Request) (*enrollmentModels.Request, error) {
	if r == nil {
		return nil, nil
	}
	result := &enrollmentModels.Request{}
	result.ID = r.ID
	result.TenantID = r.TenantID
	result.CreatedAt = r.CreatedAt
	result.UpdatedAt = r.UpdatedAt
	result.SchemaID = r.SchemaID
	result.PhaseID = r.PhaseID
	result.GuardianFirstName = r.GuardianFirstName
	result.GuardianLastName = r.GuardianLastName
	result.GuardianEmail = r.GuardianEmail
	result.GuardianPhone = r.GuardianPhone
	result.GuardianAccountID = r.GuardianAccountID
	result.SubmissionSource = r.SubmissionSource
	result.StatusToken = r.StatusToken
	result.StatusTokenExpires = r.StatusTokenExpires
	result.SubmittedAt = r.SubmittedAt
	result.WithdrawnAt = r.WithdrawnAt
	result.DecisionNotificationMode = r.DecisionNotificationMode
	if len(r.ConsentFlags) > 0 {
		if err := json.Unmarshal(r.ConsentFlags, &result.ConsentFlags); err != nil {
			return nil, err
		}
	}
	if len(r.LegalBlocksSnapshot) > 0 {
		if err := json.Unmarshal(r.LegalBlocksSnapshot, &result.LegalBlocksSnapshot); err != nil {
			return nil, err
		}
	}
	if len(r.CustomData) > 0 {
		if err := json.Unmarshal(r.CustomData, &result.CustomData); err != nil {
			return nil, err
		}
	}
	if len(r.SourceMetadata) > 0 {
		if err := json.Unmarshal(r.SourceMetadata, &result.SourceMetadata); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func intakeRequestValues(values []*capability.Request) ([]*enrollmentModels.Request, error) {
	var result []*enrollmentModels.Request
	for _, value := range values {
		converted, err := intakeRequestValue(value)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

// allChildrenParentResolved reports whether every child of a request has a
// final status the parents hear about. A request without children, or with a
// missing child, is not resolved.
func allChildrenParentResolved(children []*RequestChild) bool {
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if child == nil || !capability.ParentResolvedStatus(child.Status) {
			return false
		}
	}
	return true
}

// schoolBrand returns the school's mail branding, or empty strings when no
// Enrollment notifications are bound.
func schoolBrand(ctx context.Context, notifications capability.Notifications, tenantID int64, baseURL string) (string, string) {
	if notifications == nil {
		return "", ""
	}
	return notifications.SchoolBrand(ctx, tenantID, baseURL)
}

// decisionNotice is one persisted child-state generation of a request as the
// retained flows hold it: the decoded request and children.
type decisionNotice struct {
	Request           *enrollmentModels.Request
	Children          []*RequestChild
	Phase             *capability.Phase
	ImmediateChildIDs map[int64]struct{}
	ParentsURL        string
}

// notifyDecisions routes a decision generation through the Enrollment
// notifications and records the notification mode they pinned on the
// request.
func notifyDecisions(ctx context.Context, notifications capability.Notifications, notice decisionNotice) error {
	if notifications == nil {
		return fmt.Errorf("decision: parent notifications are not configured")
	}
	if notice.Request == nil {
		return fmt.Errorf("decision: request is required for notification mode")
	}
	request := notice.Request
	children := make([]capability.DecisionChild, 0, len(notice.Children))
	for _, child := range notice.Children {
		if child == nil {
			// A missing child keeps a digest unresolved and is never mailed.
			children = append(children, capability.DecisionChild{})
			continue
		}
		children = append(children, capability.DecisionChild{
			ID: child.ID, FirstName: child.FirstName, LastName: child.LastName,
			Status: child.Status, StatusReason: child.StatusReason, ReviewedAt: child.ReviewedAt,
		})
	}
	mode, err := notifications.NotifyDecisions(ctx, capability.DecisionNotice{
		Request: capability.DecisionRequest{
			ID: request.ID, TenantID: request.TenantID,
			GuardianFirstName: request.GuardianFirstName, GuardianLastName: request.GuardianLastName,
			GuardianEmail: request.GuardianEmail, StatusToken: request.StatusToken,
			NotificationMode: request.DecisionNotificationMode,
		},
		Children: children, Phase: notice.Phase,
		ImmediateChildIDs: notice.ImmediateChildIDs, ParentsURL: notice.ParentsURL,
	})
	if err != nil {
		return err
	}
	if request.DecisionNotificationMode == nil {
		request.DecisionNotificationMode = &mode
	}
	return nil
}
