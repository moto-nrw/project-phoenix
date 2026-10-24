package peopledirectory

import (
	"context"
	"encoding/json"
)

// StudentFieldChange is a proposed change to one People Directory field.
// Request IDs identify results only; this capability never reads or writes
// request rows, which belong to Care Plan.
type StudentFieldChange struct {
	RequestID int64
	StudentID int64
	Target    string
	Field     string
	OldValue  json.RawMessage
	NewValue  json.RawMessage
}

// StudentFieldReview contains the owner's baseline and validation facts.
// Care Plan applies the caller's review scope separately.
type StudentFieldReview struct {
	Student              *Student
	FirstName            string
	LastName             string
	BulkEligible         bool
	BulkIneligibleReason string
	BulkIneligibleText   string
	CurrentValueChanged  *bool
}

type StudentFieldReviewQuery interface {
	ReviewStudentFields(context.Context, []StudentFieldChange) (map[int64]StudentFieldReview, error)
}
