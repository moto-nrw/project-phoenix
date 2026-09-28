package enrollment

import (
	"context"
	"errors"
)

// Sentinels of the admin deletion of enrollment requests. The HTTP adapters
// classify them; the messages are part of the wire text of a refused request.
var (
	ErrEnrollmentDeletionNotFound      = errors.New("enrollment deletion target not found")
	ErrEnrollmentDeletionInvalidReason = errors.New("deletion reason must contain between 3 and 500 characters")
	ErrEnrollmentDeletionNotAllowed    = errors.New("enrollment child cannot be deleted in its current status")
	ErrEnrollmentDeletionStudentExists = errors.New("enrollment deletion blocked because a created student still exists")
)

// DeletionCounts describes every request-owned row affected by an enrollment
// deletion. Counts are captured before deletion and persisted in the audit row.
type DeletionCounts struct {
	Requests                  int `json:"requests"`
	RequestChildren           int `json:"request_children"`
	RequestChildOfferings     int `json:"request_child_offerings"`
	RequestGuardians          int `json:"request_guardians"`
	ChangeRequests            int `json:"change_requests"`
	ChangeRequestMessages     int `json:"change_request_messages"`
	LateInvites               int `json:"late_invites"`
	OfferingAdjustments       int `json:"offering_adjustments"`
	EmailOutbox               int `json:"email_outbox"`
	RolloverLinksCleared      int `json:"rollover_links_cleared"`
	StudentSourceLinksCleared int `json:"student_source_links_cleared"`
}

// Total returns the number of affected rows, including links that are cleared
// by ON DELETE SET NULL constraints.
func (c DeletionCounts) Total() int {
	return c.Requests + c.RequestChildren + c.RequestChildOfferings +
		c.RequestGuardians + c.ChangeRequests + c.ChangeRequestMessages +
		c.LateInvites + c.OfferingAdjustments + c.EmailOutbox +
		c.RolloverLinksCleared + c.StudentSourceLinksCleared
}

// DeletionImpact is the transaction-ready deletion preview. Student IDs are
// identifiers only; names and other PII deliberately never enter the audit path.
type DeletionImpact struct {
	RequestID                     int64
	ChildID                       *int64
	DeletesRequest                bool
	Counts                        DeletionCounts
	BlockingStudentIDs            []int64
	PreservedGuardianProfiles     int
	PreservedParentAccounts       int
	UnlinkedGuardianProfiles      int
	ParentAccountsWithoutStudents int
}

// EnrollmentDeletions previews and deletes an enrollment request or one of its
// children. The deletions cancel the request's pending mails and append the
// deletion audit row in the same tenant transaction.
type EnrollmentDeletions interface {
	PreviewRequest(ctx context.Context, requestID int64) (*DeletionImpact, error)
	PreviewChild(ctx context.Context, requestID, childID int64) (*DeletionImpact, error)
	DeleteRequest(ctx context.Context, requestID, actorAccountID int64, reason string) (*DeletionImpact, error)
	DeleteChild(ctx context.Context, requestID, childID, actorAccountID int64, reason string) (*DeletionImpact, error)
}

// RejectedEnrollmentCleanupResult reports one retention run.
type RejectedEnrollmentCleanupResult struct {
	DeletedRequests    int
	DeletedLateInvites int64
	// DeletedOutboxRows is retained as an API field name. Delivery rows are
	// cancelled and retained for audit; this counts rows transitioned to cancelled.
	DeletedOutboxRows int64
}

// RejectedEnrollmentCleaner removes rejected enrollment data after its
// tenant-configured retention window.
type RejectedEnrollmentCleaner interface {
	CleanupRejectedEnrollments(ctx context.Context) (RejectedEnrollmentCleanupResult, error)
}
