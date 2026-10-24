package masterdatarequests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStudentDataChangeRequest_IsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status string
		want   bool
	}{
		{status: StatusAutoApplied, want: true},
		{status: StatusApproved, want: true},
		{status: StatusRejected, want: true},
		{status: StatusPending, want: false},
		{status: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			row := &Request{Status: tt.status}
			assert.Equal(t, tt.want, row.IsTerminal())
		})
	}
}

// The legacy status-constant contract retains its original predicate and cases.
// No production consumer of this model method remains.
// IsTerminal reports whether the row is in a final state — a Track A audit row
// (auto_applied) or a decided Track B row (approved/rejected). Only pending
// rows accept a staff decision.
func (c *Request) IsTerminal() bool {
	switch c.Status {
	case StatusAutoApplied, StatusApproved, StatusRejected,
		"care_ended", "done":
		return true
	default:
		return false
	}
}
