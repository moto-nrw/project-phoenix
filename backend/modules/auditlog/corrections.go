// Package auditlog exposes native read capabilities for Audit-owned records.
package auditlog

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type DirectCorrection struct {
	ID                 int64
	StudentID          int64
	ActorNameSnapshot  *string
	ActorEmailSnapshot *string
	Reason             string
	Before             json.RawMessage
	After              json.RawMessage
	ChangedAt          time.Time
}

func (row DirectCorrection) ActorName() string {
	for _, value := range []*string{row.ActorNameSnapshot, row.ActorEmailSnapshot} {
		if value != nil {
			if trimmed := strings.TrimSpace(*value); trimmed != "" {
				return trimmed
			}
		}
	}
	return "Unbekannt"
}

type CorrectionFilter struct {
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
}

type CorrectionQuery interface {
	ListDirectCorrections(context.Context, CorrectionFilter) ([]DirectCorrection, error)
}

type Observation struct {
	Operation         string
	Duration          time.Duration
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
	Err               error
}
