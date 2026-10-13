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
