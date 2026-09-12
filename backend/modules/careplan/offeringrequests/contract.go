package offeringrequests

import (
	"encoding/json"
	"time"
)

type Request struct {
	ID                          int64
	TenantID                    int64
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
	StudentID                   int64
	RequestChildID              int64
	SubmittedBy                 int64
	CompleteWithdrawalConfirmed bool
	WithdrawalConfirmedBy       *int64
	WithdrawalConfirmedAt       *time.Time
	ApprovedCompleteWithdrawal  bool
	Payload                     json.RawMessage
	EffectiveFrom               string
	ParentNote                  *string
	Status                      string
	DecisionReason              *string
	DecisionSnapshot            json.RawMessage
	ReviewedBy                  *int64
	ReviewedAt                  *time.Time
	AppliedAt                   *time.Time
}

type ReviewItem struct {
	Request                *Request
	StudentName            string
	RequestedEffectiveFrom string
	EarliestEffectiveFrom  string
	LatestEffectiveFrom    string
	Diff                   []DiffEntry
	Unchanged              []DiffEntry
	FullWithdrawal         bool
}

type RequestedItem struct {
	OfferingID int64
	Name       string
	Days       []string
}

type HistoryItem struct {
	Request      *Request
	StudentName  string
	ReviewerName string
	Diff         []DiffEntry
	Requested    []RequestedItem
}
