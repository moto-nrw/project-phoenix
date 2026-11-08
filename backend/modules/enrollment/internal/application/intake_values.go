package application

import (
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The intake validates and sanitizes a submission on working maps of its
// answers, consent flags and source metadata. The public contract carries
// them as FormValues (ADR 0170); the values below copy them into maps on the
// way in, and the stored request carries FormValues again.

// SubmitRequest is a submission with working maps of its documents.
type SubmitRequest struct {
	// DisplayedFormSchemaID is the form version the parent's form showed
	// (#4680). Versions never change, so the snapshot proves the shown legal
	// texts are the stored ones only when it equals the version in force.
	DisplayedFormSchemaID int64
	TenantID              int64
	PhaseID               int64
	RemoteIP              string
	LateInviteToken       string
	// The trust of a submission. Only the intake sets these: the staff
	// manual enrollment all four, a late invite AllowClosedPhase (#6113).
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
	Signature                *enrollment.SubmitSignature
}

// SubmitGuardian is one co-guardian.
type SubmitGuardian = enrollment.SubmitGuardian

// SubmitOfferingDays is the day selection of one parent_choice offering.
type SubmitOfferingDays = enrollment.SubmitOfferingDays

// SubmitChild is one child of a submission with a working map of its answers.
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

func decodeSubmitRequest(in enrollment.SubmitRequest) SubmitRequest {
	out := SubmitRequest{
		TenantID: in.TenantID, PhaseID: in.PhaseID, RemoteIP: in.RemoteIP, LateInviteToken: in.LateInviteToken,
		SubmissionSource: in.SubmissionSource, GuardianFirstName: in.GuardianFirstName,
		GuardianLastName: in.GuardianLastName, GuardianEmail: in.GuardianEmail, GuardianPhone: in.GuardianPhone,
		GuardianAccountID: in.GuardianAccountID, GuardianSubmitEligible: in.GuardianSubmitEligible,
		AdditionalGuardians: in.AdditionalGuardians, Signature: in.Signature, DisplayedFormSchemaID: in.DisplayedFormSchemaID,
	}
	out.SourceMetadata = submittedMap(in.SourceMetadata)
	out.ConsentFlags = submittedMap(in.ConsentFlags)
	out.CustomData = submittedMap(in.CustomData)
	if in.Children != nil {
		out.Children = make([]SubmitChild, 0, len(in.Children))
	}
	for _, child := range in.Children {
		out.Children = append(out.Children, SubmitChild{
			ID: child.ID, FirstName: child.FirstName, LastName: child.LastName, DateOfBirth: child.DateOfBirth,
			TargetGradeLevel: child.TargetGradeLevel, TargetSchoolClass: child.TargetSchoolClass, CustomData: submittedMap(child.CustomData),
			OfferingIDs: child.OfferingIDs, OfferingDays: child.OfferingDays, ExcludedAutoAddTargetIDs: child.ExcludedAutoAddTargetIDs,
		})
	}
	return out
}

// submittedMap is the working map of a submitted document; an empty one is
// nil, as an absent document was before ADR 0170.
func submittedMap(values enrollment.FormValues) map[string]any {
	if values.IsZero() {
		return nil
	}
	return formMap(values)
}

// submitOutcome is what a stored submission or edit produced.
type submitOutcome struct {
	Request   *enrollment.Request
	Children  []*enrollment.RequestChild
	StatusURL string
	Warnings  []enrollment.SubmissionWarning
}

func (o *submitOutcome) public() *enrollment.SubmitResult {
	return &enrollment.SubmitResult{Request: o.Request, Children: o.Children, StatusURL: o.StatusURL, Warnings: o.Warnings}
}

// childTakenOver reports whether an enrollment child has already been taken
// over into care: an approval materialized it into a student. From then on
// change wishes for the child run through the parent app, so the status link
// keeps it readable but locks it in the change form (ADR 0003).
func childTakenOver(child *enrollment.RequestChild) bool {
	return child != nil && child.CreatedStudentID != nil && *child.CreatedStudentID > 0
}

// allChildrenTakenOver reports whether every child is past the takeover.
func allChildrenTakenOver(children []*enrollment.RequestChild) bool {
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
func childrenParentResolved(children []*enrollment.RequestChild) bool {
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

func childIDsForStatus(children []*enrollment.RequestChild, status string) map[int64]struct{} {
	ids := make(map[int64]struct{})
	for _, child := range children {
		if child != nil && child.Status == status {
			ids[child.ID] = struct{}{}
		}
	}
	return ids
}
