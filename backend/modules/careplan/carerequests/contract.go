package carerequests

import (
	"encoding/json"
	"time"
)

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
}

type HistoryItem struct {
	Request      *Request
	FirstName    string
	LastName     string
	ReviewerName string
	Requested    []DiffEntry
	Diff         []DiffEntry
}

type Block struct {
	ID        int64
	Title     string
	StartTime time.Time
	EndTime   time.Time
}
