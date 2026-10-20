package domain

import (
	"strings"
	"time"
)

const DefaultUnregisteredTagScanLimit = 500

// ResolveUnregisteredTagScan marks one open scan as handled by an operator.
type ResolveUnregisteredTagScan struct {
	ID         int64
	OperatorID int64
	Note       *string
	ResolvedAt time.Time
}

// Normalize drops an empty note and stamps a missing resolution time.
func (r *ResolveUnregisteredTagScan) Normalize(now time.Time) {
	if r.Note != nil {
		trimmed := strings.TrimSpace(*r.Note)
		if trimmed == "" {
			r.Note = nil
		} else {
			r.Note = &trimmed
		}
	}
	if r.ResolvedAt.IsZero() {
		r.ResolvedAt = now
	}
}
