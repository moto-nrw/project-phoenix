package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestRolePermission_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rp      *RolePermission
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid role permission",
			rp: &RolePermission{
				RoleID:       1,
				PermissionID: 1,
			},
			wantErr: false,
		},
		{
			name: "zero role ID",
			rp: &RolePermission{
				RoleID:       0,
				PermissionID: 1,
			},
			wantErr: true,
			errMsg:  "role ID is required",
		},
		{
			name: "negative role ID",
			rp: &RolePermission{
				RoleID:       -1,
				PermissionID: 1,
			},
			wantErr: true,
			errMsg:  "role ID is required",
		},
		{
			name: "zero permission ID",
			rp: &RolePermission{
				RoleID:       1,
				PermissionID: 0,
			},
			wantErr: true,
			errMsg:  "permission ID is required",
		},
		{
			name: "negative permission ID",
			rp: &RolePermission{
				RoleID:       1,
				PermissionID: -1,
			},
			wantErr: true,
			errMsg:  "permission ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rp.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RolePermission.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("RolePermission.Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestRolePermission_GetID(t *testing.T) {
	t.Parallel()

	rp := &RolePermission{
		Model: base.Model{ID: 42},
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := rp.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", rp.GetID())
	}
}

func TestRolePermission_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	rp := &RolePermission{
		Model: base.Model{CreatedAt: now},
	}

	if got := rp.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestRolePermission_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	rp := &RolePermission{
		Model: base.Model{UpdatedAt: now},
	}

	if got := rp.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
