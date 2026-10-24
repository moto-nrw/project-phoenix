package careplan

import "time"

// The guardian portal's views of Enrollment. Enrollment sits above Care Plan
// (ADR 0058, #3417), so the parent repositories read them in Care Plan's
// vocabulary and the root binds Enrollment's phases and applications to it.

// ParentEnrollmentPhase is an open enrollment phase a guardian may apply in.
type ParentEnrollmentPhase struct {
	ID       int64
	TenantID int64
	Name     string
	// NameTranslations are the school's current translations of Name.
	NameTranslations  map[string]string
	Kind              string
	ServiceStartDate  Date
	ServiceEndDate    Date
	EnrollmentOpenAt  *time.Time
	EnrollmentCloseAt *time.Time
	Audience          string
}

// ParentEnrollmentRequest is one application of a guardian account.
type ParentEnrollmentRequest struct {
	RequestID                int64
	TenantID                 int64
	StatusToken              string
	SubmittedAt              time.Time
	WithdrawnAt              *time.Time
	PhaseID                  int64
	PhaseName                string
	ServiceStartDate         Date
	ServiceEndDate           Date
	ShowStatusReasonToParent bool
	Children                 []ParentEnrollmentRequestChild
}

// ParentEnrollmentRequestChild is one child of an application.
type ParentEnrollmentRequestChild struct {
	ChildID          int64
	FirstName        string
	LastName         string
	Status           string
	StatusReason     *string
	CreatedStudentID *int64
}
