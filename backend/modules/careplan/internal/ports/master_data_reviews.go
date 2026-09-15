package ports

import (
	"context"
	"encoding/json"
)

type MasterDataFieldChange struct {
	RequestID int64
	StudentID int64
	Target    string
	Field     string
	OldValue  json.RawMessage
	NewValue  json.RawMessage
}

type MasterDataFieldFacts struct {
	Student              *ReviewStudent
	FirstName            string
	LastName             string
	BulkEligible         bool
	BulkIneligibleReason string
	BulkIneligibleText   string
	CurrentValueChanged  *bool
}

// MasterDataDirectory supplies only People Directory facts, never request
// persistence or a decision. Scope remains a separate identity capability.
type MasterDataDirectory interface {
	ReviewFields(context.Context, []MasterDataFieldChange) (map[int64]MasterDataFieldFacts, error)
	FindStudents(context.Context, []int64) (map[int64]ReviewStudent, error)
	PersonNames(context.Context, []int64) (map[int64]PersonName, error)
	ReviewerNames(context.Context, []int64) (map[int64]PersonName, error)
}
