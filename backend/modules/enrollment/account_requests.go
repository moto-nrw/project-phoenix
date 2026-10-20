package enrollment

import (
	"time"
)

// AccountRequest is the Enrollment-owned part of a parent's application list.
// The parent workflow checks relationship permissions for every created student
// before exposing the request and enriches school information through its owner.
type AccountRequest struct {
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
	Children                 []AccountRequestChild
}

type AccountRequestChild struct {
	ChildID          int64
	FirstName        string
	LastName         string
	Status           string
	StatusReason     *string
	CreatedStudentID *int64
}
