package care

import (
	"encoding/json"
	"time"
)

// StudentDataChangeRequest is the complete native field-change snapshot.
type StudentDataChangeRequest struct {
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
