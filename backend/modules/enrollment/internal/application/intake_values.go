package application

import (
	"context"
	"encoding/json"
	"fmt"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The intake works on the decoded submission: answers, consent flags and
// source metadata are maps. The public contract carries them as raw JSON;
// the values below decode them on the way in and encode them on the way out.

// SubmitRequest is a decoded submission.
type SubmitRequest struct {
	TenantID                 int64
	PhaseID                  int64
	RemoteIP                 string
	LateInviteToken          string
	SkipRateLimit            bool
	AllowClosedPhase         bool
	SuppressSubmissionEmails bool
	ExternalConsentConfirmed bool
	SubmissionSource         string
	SourceMetadata           map[string]any
	GuardianFirstName        string
	GuardianLastName         string
	GuardianEmail            string
	GuardianPhone            *string
	ConsentFlags             map[string]any
	CustomData               map[string]any
	GuardianAccountID        *int64
	GuardianSubmitEligible   bool
	Children                 []SubmitChild
	AdditionalGuardians      []SubmitGuardian
}

// SubmitGuardian is one decoded co-guardian.
type SubmitGuardian = enrollment.SubmitGuardian

// SubmitOfferingDays is the day selection of one parent_choice offering.
type SubmitOfferingDays = enrollment.SubmitOfferingDays

// SubmitChild is one decoded child of a submission.
type SubmitChild struct {
	ID                       int64
	FirstName                string
	LastName                 string
	DateOfBirth              calendar.Date
	TargetGradeLevel         *int16
	TargetSchoolClass        *string
	CustomData               map[string]any
	OfferingIDs              []int64
	OfferingDays             []SubmitOfferingDays
	ExcludedAutoAddTargetIDs map[int64]bool
}

// decodeJSONMap decodes an optional JSON object; an absent document is nil.
func decodeJSONMap(raw json.RawMessage, what string) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode %s: %w", what, err)
	}
	return out, nil
}

// encodeJSONMap encodes a decoded document; a nil map stays absent.
func encodeJSONMap(value map[string]any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func decodeSubmitRequest(in enrollment.SubmitRequest) (SubmitRequest, error) {
	out := SubmitRequest{
		TenantID: in.TenantID, PhaseID: in.PhaseID, RemoteIP: in.RemoteIP, LateInviteToken: in.LateInviteToken,
		SkipRateLimit: in.SkipRateLimit, AllowClosedPhase: in.AllowClosedPhase,
		SuppressSubmissionEmails: in.SuppressSubmissionEmails, ExternalConsentConfirmed: in.ExternalConsentConfirmed,
		SubmissionSource: in.SubmissionSource, GuardianFirstName: in.GuardianFirstName,
		GuardianLastName: in.GuardianLastName, GuardianEmail: in.GuardianEmail, GuardianPhone: in.GuardianPhone,
		GuardianAccountID: in.GuardianAccountID, GuardianSubmitEligible: in.GuardianSubmitEligible,
		AdditionalGuardians: in.AdditionalGuardians,
	}
	var err error
	if out.SourceMetadata, err = decodeJSONMap(in.SourceMetadata, "source metadata"); err != nil {
		return out, err
	}
	if out.ConsentFlags, err = decodeJSONMap(in.ConsentFlags, "consent flags"); err != nil {
		return out, err
	}
	if out.CustomData, err = decodeJSONMap(in.CustomData, "custom data"); err != nil {
		return out, err
	}
	if in.Children != nil {
		out.Children = make([]SubmitChild, 0, len(in.Children))
	}
	for _, child := range in.Children {
		decoded, err := decodeSubmitChild(child)
		if err != nil {
			return out, err
		}
		out.Children = append(out.Children, decoded)
	}
	return out, nil
}

func decodeSubmitChild(in enrollment.SubmitChild) (SubmitChild, error) {
	customData, err := decodeJSONMap(in.CustomData, "child custom data")
	if err != nil {
		return SubmitChild{}, err
	}
	return SubmitChild{
		ID: in.ID, FirstName: in.FirstName, LastName: in.LastName, DateOfBirth: in.DateOfBirth,
		TargetGradeLevel: in.TargetGradeLevel, TargetSchoolClass: in.TargetSchoolClass, CustomData: customData,
		OfferingIDs: in.OfferingIDs, OfferingDays: in.OfferingDays, ExcludedAutoAddTargetIDs: in.ExcludedAutoAddTargetIDs,
	}, nil
}

// submitOutcome is what a stored submission or edit produced, decoded.
type submitOutcome struct {
	Request   *enrollmentModels.Request
	Children  []*RequestChild
	StatusURL string
	Warnings  []enrollment.SubmissionWarning
}

func (o *submitOutcome) public() (*enrollment.SubmitResult, error) {
	request, err := requestInput(o.Request)
	if err != nil {
		return nil, err
	}
	children, err := childInputs(o.Children)
	if err != nil {
		return nil, err
	}
	return &enrollment.SubmitResult{Request: request, Children: children, StatusURL: o.StatusURL, Warnings: o.Warnings}, nil
}

func childInputs(values []*RequestChild) ([]*enrollment.RequestChild, error) {
	if values == nil {
		return nil, nil
	}
	out := make([]*enrollment.RequestChild, 0, len(values))
	for _, value := range values {
		converted, err := childInput(value)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

// createDecodedRequest stores a decoded request and refreshes it with the
// generated metadata.
func createDecodedRequest(ctx context.Context, owner interface {
	InsertRequest(context.Context, *enrollment.Request) error
}, req *enrollmentModels.Request) error {
	value, err := requestInput(req)
	if err != nil {
		return err
	}
	if err := owner.InsertRequest(ctx, value); err != nil {
		return err
	}
	stored, err := requestValue(value)
	if err != nil {
		return err
	}
	*req = *stored
	return nil
}

// updateDecodedRequest writes the guardian fields of a decoded request.
func updateDecodedRequest(ctx context.Context, owner interface {
	UpdateRequestGuardian(context.Context, *enrollment.Request, bool) error
}, req *enrollmentModels.Request, includeEmail bool) error {
	value, err := requestInput(req)
	if err != nil {
		return err
	}
	return owner.UpdateRequestGuardian(ctx, value, includeEmail)
}

// decodedRequestByToken reads and decodes the request behind a status
// token, optionally locked.
func decodedRequestByToken(ctx context.Context, owner interface {
	RequestByToken(context.Context, string, bool) (*enrollment.Request, error)
}, token string, lock bool) (*enrollmentModels.Request, error) {
	value, err := owner.RequestByToken(ctx, token, lock)
	if err != nil {
		return nil, err
	}
	return requestValue(value)
}

// decodedRequestsByID reads and decodes a batch of requests.
func decodedRequestsByID(ctx context.Context, owner interface {
	RequestsByID(context.Context, []int64) ([]*enrollment.Request, error)
}, ids []int64) ([]*enrollmentModels.Request, error) {
	values, err := owner.RequestsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return requestValues(values)
}

// createDecodedChild stores a decoded child and refreshes it with the
// generated metadata.
func createDecodedChild(ctx context.Context, owner interface {
	InsertChild(context.Context, *enrollment.RequestChild) error
}, child *RequestChild) error {
	value, err := childInput(child)
	if err != nil {
		return err
	}
	if err := owner.InsertChild(ctx, value); err != nil {
		return err
	}
	stored, err := childValue(value)
	if err != nil {
		return err
	}
	*child = *stored
	return nil
}

// updateDecodedChild writes the data of a decoded child.
func updateDecodedChild(ctx context.Context, owner interface {
	UpdateChildData(context.Context, *enrollment.RequestChild) error
}, child *RequestChild) error {
	value, err := childInput(child)
	if err != nil {
		return err
	}
	return owner.UpdateChildData(ctx, value)
}

// decodedChildrenOfRequests reads and decodes the children of a batch of
// requests.
func decodedChildrenOfRequests(ctx context.Context, owner interface {
	ChildrenForRequests(context.Context, []int64) ([]*enrollment.RequestChild, error)
}, requestIDs []int64) ([]*RequestChild, error) {
	values, err := owner.ChildrenForRequests(ctx, requestIDs)
	if err != nil {
		return nil, err
	}
	return childValues(values)
}

// childTakenOver reports whether an enrollment child has already been taken
// over into care: an approval materialized it into a student. From then on
// change wishes for the child run through the parent app, so the status link
// keeps it readable but locks it in the change form (ADR 0003).
func childTakenOver(child *RequestChild) bool {
	return child != nil && child.CreatedStudentID != nil && *child.CreatedStudentID > 0
}

// allChildrenTakenOver reports whether every child is past the takeover.
func allChildrenTakenOver(children []*RequestChild) bool {
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if !childTakenOver(child) {
			return false
		}
	}
	return true
}

// childrenParentResolved reports whether every child of a request has a
// final status the parents hear about. A request without children, or with
// a missing child, is not resolved.
func childrenParentResolved(children []*RequestChild) bool {
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if child == nil || !enrollment.ParentResolvedStatus(child.Status) {
			return false
		}
	}
	return true
}

func childIDsForStatus(children []*RequestChild, status string) map[int64]struct{} {
	ids := make(map[int64]struct{})
	for _, child := range children {
		if child != nil && child.Status == status {
			ids[child.ID] = struct{}{}
		}
	}
	return ids
}
