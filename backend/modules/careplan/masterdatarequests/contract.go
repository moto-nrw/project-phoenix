// Package masterdatarequests is the Care Plan contract for reviewing parent
// field-change requests. It exposes no persistence or retained service types.
package masterdatarequests

import (
	"encoding/json"
	"time"
)

type Request struct {
	ID           int64
	TenantID     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	StudentID    int64
	SubmittedBy  int64
	Target       string
	TargetRefID  *int64
	FieldKey     string
	OldValue     json.RawMessage
	NewValue     json.RawMessage
	Status       string
	ReviewReason *string
	ReviewedBy   *int64
	ReviewedAt   *time.Time
	AppliedAt    *time.Time
}

type ReviewItem struct {
	Request              *Request
	FirstName            string
	LastName             string
	BulkEligible         bool
	BulkIneligibleReason string
	BulkIneligibleText   string
	CurrentValueChanged  *bool
}

type HistoryItem struct {
	Request      *Request
	FirstName    string
	LastName     string
	ReviewerName string
}

func (r Request) ConflictKeys() []string {
	if r.Target == "" || r.FieldKey == "" {
		return nil
	}
	return []string{"md:" + r.Target + ":" + r.FieldKey}
}

func (r Request) CanCorrect() bool {
	return r.Status == "approved" || r.Status == "rejected"
}
