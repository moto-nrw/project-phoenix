package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The decision, restore and rollover flows work on the decoded request and
// child: their answers and consent flags are maps. The owner persists them
// as JSON; the flows decode them here and hand the owner's own records to
// their callers.

// RequestChild is a decoded child of a parent submission. Each child has an
// independent status; the parent request's status is derived from its
// children.
type RequestChild struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RequestID        int64
	FirstName        string
	LastName         string
	DateOfBirth      calendar.Date
	TargetGradeLevel *int16
	// TargetSchoolClass is the concrete future class (e.g. "2a") chosen at
	// enrollment (#1833). Empty means grade-only ("Klasse offen"); on
	// approval a non-empty value lands verbatim in the student's school
	// class, otherwise the grade-derived fallback is used.
	TargetSchoolClass *string
	CustomData        map[string]any
	Status            string
	StatusReason      *string
	ActivationMode    string
	ActivateOn        *calendar.Date
	ReviewedAt        *time.Time
	ReviewedBy        *int64
	CreatedStudentID  *int64
	// MatchedStudentID is set only for existing_students phases: the
	// already-enrolled student this child was matched to at submission. On
	// approval the decision renews that student instead of creating a
	// duplicate person and student.
	MatchedStudentID      *int64
	SortOrder             int
	RolloverSourceChildID *int64
	ReviewReason          *string
}

func childValue(r *enrollment.RequestChild) (*RequestChild, error) {
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
	if len(r.CustomData) > 0 {
		if err := json.Unmarshal(r.CustomData, &result.CustomData); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func childValues(values []*enrollment.RequestChild) ([]*RequestChild, error) {
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

func childInput(r *RequestChild) (*enrollment.RequestChild, error) {
	if r == nil {
		return nil, nil
	}
	result := &enrollment.RequestChild{
		ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestID: r.RequestID,
		FirstName: r.FirstName, LastName: r.LastName, TargetGradeLevel: r.TargetGradeLevel,
		TargetSchoolClass: r.TargetSchoolClass, Status: r.Status, StatusReason: r.StatusReason,
		ActivationMode: r.ActivationMode, ReviewedAt: r.ReviewedAt, ReviewedBy: r.ReviewedBy,
		CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID, SortOrder: r.SortOrder,
		RolloverSourceChildID: r.RolloverSourceChildID, ReviewReason: r.ReviewReason,
	}
	result.DateOfBirth = enrollment.Date(r.DateOfBirth.String())
	if r.ActivateOn != nil {
		date := enrollment.Date(r.ActivateOn.String())
		result.ActivateOn = &date
	}
	data, err := json.Marshal(r.CustomData)
	if err != nil {
		return nil, err
	}
	result.CustomData = data
	return result, nil
}

func requestValues(values []*enrollment.Request) ([]*enrollmentModels.Request, error) {
	var result []*enrollmentModels.Request
	for _, value := range values {
		converted, err := requestValue(value)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

func requestInput(r *enrollmentModels.Request) (*enrollment.Request, error) {
	if r == nil {
		return nil, nil
	}
	result := &enrollment.Request{
		ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, SchemaID: r.SchemaID,
		PhaseID: r.PhaseID, GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName,
		GuardianEmail: r.GuardianEmail, GuardianPhone: r.GuardianPhone, GuardianAccountID: r.GuardianAccountID,
		SubmissionSource: r.SubmissionSource, StatusToken: r.StatusToken, StatusTokenExpires: r.StatusTokenExpires,
		SubmittedAt: r.SubmittedAt, WithdrawnAt: r.WithdrawnAt, DecisionNotificationMode: r.DecisionNotificationMode,
	}
	var err error
	if result.ConsentFlags, err = json.Marshal(r.ConsentFlags); err != nil {
		return nil, err
	}
	if result.LegalBlocksSnapshot, err = json.Marshal(r.LegalBlocksSnapshot); err != nil {
		return nil, err
	}
	if result.CustomData, err = json.Marshal(r.CustomData); err != nil {
		return nil, err
	}
	if result.SourceMetadata, err = json.Marshal(r.SourceMetadata); err != nil {
		return nil, err
	}
	return result, nil
}

// decodedRequestByID reads and decodes one request, optionally locked.
func decodedRequestByID(ctx context.Context, owner interface {
	RequestByID(context.Context, int64, bool) (*enrollment.Request, error)
}, id int64, lock bool) (*enrollmentModels.Request, error) {
	value, err := owner.RequestByID(ctx, id, lock)
	if err != nil {
		return nil, err
	}
	return requestValue(value)
}

// decodedChildByID reads and decodes one request child.
func decodedChildByID(ctx context.Context, owner interface {
	ChildByID(context.Context, int64) (*enrollment.RequestChild, error)
}, id int64) (*RequestChild, error) {
	value, err := owner.ChildByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return childValue(value)
}

// decodedChildrenOfRequest reads and decodes the children of one request in
// the owner's stable sort_order/id order, optionally locked.
func decodedChildrenOfRequest(ctx context.Context, owner interface {
	ChildrenForRequest(context.Context, int64, bool) ([]*enrollment.RequestChild, error)
}, requestID int64, forUpdate bool) ([]*RequestChild, error) {
	values, err := owner.ChildrenForRequest(ctx, requestID, forUpdate)
	if err != nil {
		return nil, err
	}
	return childValues(values)
}

// schoolBrand returns the school's mail branding, or empty strings when no
// Enrollment notifications are bound.
func schoolBrand(ctx context.Context, notifications enrollment.Notifications, tenantID int64, baseURL string) (string, string) {
	if notifications == nil {
		return "", ""
	}
	return notifications.SchoolBrand(ctx, tenantID, baseURL)
}

// decisionGeneration is one persisted child-state generation of a request as
// the decision flow holds it: the decoded request and its children.
type decisionGeneration struct {
	Request           *enrollmentModels.Request
	Children          []*RequestChild
	Phase             *enrollment.Phase
	ImmediateChildIDs map[int64]struct{}
	ParentsURL        string
}

// notifyDecisions routes a decision generation through the Enrollment
// notifications and records the notification mode they pinned on the
// request.
func notifyDecisions(ctx context.Context, notifications enrollment.Notifications, notice decisionGeneration) error {
	if notifications == nil {
		return fmt.Errorf("decision: parent notifications are not configured")
	}
	if notice.Request == nil {
		return fmt.Errorf("decision: request is required for notification mode")
	}
	request := notice.Request
	mode, err := notifications.NotifyDecisions(ctx, enrollment.DecisionNotice{
		Request: enrollment.DecisionRequest{
			ID: request.ID, TenantID: request.TenantID,
			GuardianFirstName: request.GuardianFirstName, GuardianLastName: request.GuardianLastName,
			GuardianEmail: request.GuardianEmail, StatusToken: request.StatusToken,
			NotificationMode: request.DecisionNotificationMode,
		},
		Children: noticeChildren(notice.Children), Phase: notice.Phase,
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

func noticeChildren(values []*RequestChild) []enrollment.DecisionChild {
	children := make([]enrollment.DecisionChild, 0, len(values))
	for _, child := range values {
		if child == nil {
			// A missing child keeps a digest unresolved and is never mailed.
			children = append(children, enrollment.DecisionChild{})
			continue
		}
		children = append(children, enrollment.DecisionChild{
			ID: child.ID, FirstName: child.FirstName, LastName: child.LastName,
			Status: child.Status, StatusReason: child.StatusReason, ReviewedAt: child.ReviewedAt,
		})
	}
	return children
}
