package users

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
		{status: DataChangeStatusAutoApplied, want: true},
		{status: DataChangeStatusApproved, want: true},
		{status: DataChangeStatusRejected, want: true},
		{status: DataChangeStatusPending, want: false},
		{status: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			row := &StudentDataChangeRequest{Status: tt.status}
			assert.Equal(t, tt.want, row.IsTerminal())
		})
	}
}

// The legacy status-constant contract retains its original predicate and cases.
// No production consumer of this model method remains.
// IsTerminal reports whether the row is in a final state — a Track A audit row
// (auto_applied) or a decided Track B row (approved/rejected). Only pending
// rows accept a staff decision.
func (c *StudentDataChangeRequest) IsTerminal() bool {
	switch c.Status {
	case DataChangeStatusAutoApplied, DataChangeStatusApproved, DataChangeStatusRejected,
		DataChangeStatusCareEnded, DataChangeStatusDone:
		return true
	default:
		return false
	}
}
