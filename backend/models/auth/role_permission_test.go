package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestRolePermission_Validate(t *testing.T) {
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

func TestRolePermission_TableName(t *testing.T) {
	rp := &RolePermission{}
	expected := "auth.role_permissions"

	got := rp.TableName()
	if got != expected {
		t.Errorf("RolePermission.TableName() = %q, want %q", got, expected)
	}
}

func TestRolePermission_EntityInterface(t *testing.T) {
	now := time.Now()
	rp := &RolePermission{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		RoleID:       1,
		PermissionID: 2,
	}

	t.Run("GetID", func(t *testing.T) {
		got := rp.GetID()
		if got != int64(123) {
			t.Errorf("RolePermission.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := rp.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("RolePermission.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := rp.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("RolePermission.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
