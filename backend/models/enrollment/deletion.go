package enrollment

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
