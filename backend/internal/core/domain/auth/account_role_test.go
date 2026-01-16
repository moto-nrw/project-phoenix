package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
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
	if got := ar.TableName(); got != "auth.account_roles" {
		t.Errorf("TableName() = %v, want auth.account_roles", got)
	}
}

func TestAccountRole_BeforeAppendModel(t *testing.T) {
	// BeforeAppendModel modifies query table expressions for different query types
	// It doesn't set timestamps - those are handled by the base model or repository

	t.Run("handles nil query", func(t *testing.T) {
		ar := &AccountRole{AccountID: 1, RoleID: 1}
		err := ar.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		ar := &AccountRole{AccountID: 1, RoleID: 1}
		err := ar.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestAccountRole_GetID(t *testing.T) {
	ar := &AccountRole{
		Model: base.Model{ID: 42},
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := ar.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", ar.GetID())
	}
}

func TestAccountRole_GetCreatedAt(t *testing.T) {
	now := time.Now()
	ar := &AccountRole{
		Model: base.Model{CreatedAt: now},
	}

	if got := ar.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestAccountRole_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	ar := &AccountRole{
		Model: base.Model{UpdatedAt: now},
	}

	if got := ar.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
