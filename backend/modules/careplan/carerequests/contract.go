package carerequests

import (
	"encoding/json"
	"time"
)

// StatusPending is the status of a request no reviewer has decided yet.
const (
	StatusPending    = "pending"
	StatusApproved   = "approved"
	StatusRejected   = "rejected"
	StatusWithdrawn  = "withdrawn"
	StatusDone       = "done"
	StatusCareEnded  = "care_ended"
	KindPickupChange = "pickup_change"
)

// LedgerRequestType maps a request kind onto the request type of the parent
// request ledger. The weekly plan and the one-day pickup change are separate
// request types everywhere else too, so the history reads the same.
func LedgerRequestType(requestKind string) string {
	if requestKind == "pickup_change" {
		return "pickup_change"
	}
	return "care_schedule"
}

type Request struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StudentID        int64
	SubmittedBy      int64
	RequestKind      string
	Payload          json.RawMessage
	Status           string
	DecisionReason   *string
	ReviewedBy       *int64
	ReviewedAt       *time.Time
	AppliedAt        *time.Time
	DecisionSnapshot json.RawMessage
}

type ReviewItem struct {
	Request         *Request
	FirstName       string
	LastName        string
	Diff            []DiffEntry
	Reason          *string
	AffectedBlocks  []Block
	ImpactAvailable bool
	ImpactToken     string
}

type HistoryItem struct {
	Request       *Request
	FirstName     string
	LastName      string
	ReviewerName  string
	Requested     []DiffEntry
	Diff          []DiffEntry
	RequestReason *string
	PickupChange  *PickupChangeTerms
}

type Block struct {
	ID        int64
	Title     string
	StartTime time.Time
	EndTime   time.Time
}
