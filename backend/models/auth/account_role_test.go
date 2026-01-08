package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestAccountRole_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ar      *AccountRole
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid account role",
			ar: &AccountRole{
				AccountID: 1,
				RoleID:    1,
			},
			wantErr: false,
		},
		{
			name: "zero account ID",
			ar: &AccountRole{
				AccountID: 0,
				RoleID:    1,
			},
			wantErr: true,
			errMsg:  "account ID is required",
		},
		{
			name: "negative account ID",
			ar: &AccountRole{
				AccountID: -1,
				RoleID:    1,
			},
			wantErr: true,
			errMsg:  "account ID is required",
		},
		{
			name: "zero role ID",
			ar: &AccountRole{
				AccountID: 1,
				RoleID:    0,
			},
			wantErr: true,
			errMsg:  "role ID is required",
		},
		{
			name: "negative role ID",
			ar: &AccountRole{
				AccountID: 1,
				RoleID:    -1,
			},
			wantErr: true,
			errMsg:  "role ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ar.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("AccountRole.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("AccountRole.Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestAccountRole_TableName(t *testing.T) {
	ar := &AccountRole{}
	expected := "auth.account_roles"

	got := ar.TableName()
	if got != expected {
		t.Errorf("AccountRole.TableName() = %q, want %q", got, expected)
	}
}

func TestAccountRole_EntityInterface(t *testing.T) {
	now := time.Now()
	ar := &AccountRole{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		AccountID: 1,
		RoleID:    2,
	}

	t.Run("GetID", func(t *testing.T) {
		got := ar.GetID()
		if got != int64(123) {
			t.Errorf("AccountRole.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := ar.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("AccountRole.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := ar.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("AccountRole.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
