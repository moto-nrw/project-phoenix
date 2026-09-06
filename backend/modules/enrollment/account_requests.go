package enrollment

import (
	"context"
	"fmt"
	"strings"
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

// AccountRequests reads linked requests and pre-account submissions matching the
// account's identity email, supplied by the trusted parent workflow.
// Cross-school callers must supply their authorized admin transaction.
func (m *Module) AccountRequests(ctx context.Context, accountID int64, accountEmail string) ([]AccountRequest, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	return m.engine.AccountRequests(ctx, accountID, strings.ToLower(strings.TrimSpace(accountEmail)))
}
