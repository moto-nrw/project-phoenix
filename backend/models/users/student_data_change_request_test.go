package users_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestStudentDataChangeRequest_IsTerminal(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: usersModels.DataChangeStatusAutoApplied, want: true},
		{status: usersModels.DataChangeStatusApproved, want: true},
		{status: usersModels.DataChangeStatusRejected, want: true},
		{status: usersModels.DataChangeStatusPending, want: false},
		{status: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			row := &usersModels.StudentDataChangeRequest{Status: tt.status}
			assert.Equal(t, tt.want, row.IsTerminal())
		})
	}
}
