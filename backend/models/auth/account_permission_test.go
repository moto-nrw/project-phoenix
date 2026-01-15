package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestAccountPermission_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ap      *AccountPermission
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid account permission",
			ap: &AccountPermission{
				AccountID:    1,
				PermissionID: 1,
				Granted:      true,
			},
			wantErr: false,
		},
		{
			name: "zero account ID",
			ap: &AccountPermission{
				AccountID:    0,
				PermissionID: 1,
			},
			wantErr: true,
			errMsg:  "account ID is required",
		},
		{
			name: "negative account ID",
			ap: &AccountPermission{
				AccountID:    -1,
				PermissionID: 1,
			},
			wantErr: true,
			errMsg:  "account ID is required",
		},
		{
			name: "zero permission ID",
			ap: &AccountPermission{
				AccountID:    1,
				PermissionID: 0,
			},
			wantErr: true,
			errMsg:  "permission ID is required",
		},
		{
			name: "negative permission ID",
			ap: &AccountPermission{
				AccountID:    1,
				PermissionID: -1,
			},
			wantErr: true,
			errMsg:  "permission ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ap.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("AccountPermission.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("AccountPermission.Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestAccountPermission_TableName(t *testing.T) {
	ap := &AccountPermission{}
	if got := ap.TableName(); got != "auth.account_permissions" {
		t.Errorf("TableName() = %v, want auth.account_permissions", got)
	}
}

func TestAccountPermission_BeforeAppendModel(t *testing.T) {
	// BeforeAppendModel modifies query table expressions for different query types
	// It doesn't set timestamps - those are handled by the base model or repository

	t.Run("handles nil query", func(t *testing.T) {
		ap := &AccountPermission{AccountID: 1, PermissionID: 1}
		err := ap.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		ap := &AccountPermission{AccountID: 1, PermissionID: 1}
		err := ap.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestAccountPermission_IsGranted(t *testing.T) {
	t.Run("granted permission", func(t *testing.T) {
		ap := &AccountPermission{Granted: true}
		if !ap.IsGranted() {
			t.Error("IsGranted() should return true")
		}
	})

	t.Run("denied permission", func(t *testing.T) {
		ap := &AccountPermission{Granted: false}
		if ap.IsGranted() {
			t.Error("IsGranted() should return false")
		}
	})
}

func TestAccountPermission_Grant(t *testing.T) {
	ap := &AccountPermission{Granted: false}
	ap.Grant()

	if !ap.Granted {
		t.Error("Grant() should set Granted to true")
	}
}

func TestAccountPermission_Deny(t *testing.T) {
	ap := &AccountPermission{Granted: true}
	ap.Deny()

	if ap.Granted {
		t.Error("Deny() should set Granted to false")
	}
}

func TestAccountPermission_GetID(t *testing.T) {
	ap := &AccountPermission{
		Model: base.Model{ID: 42},
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := ap.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", ap.GetID())
	}
}

func TestAccountPermission_GetCreatedAt(t *testing.T) {
	now := time.Now()
	ap := &AccountPermission{
		Model: base.Model{CreatedAt: now},
	}

	if got := ap.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestAccountPermission_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	ap := &AccountPermission{
		Model: base.Model{UpdatedAt: now},
	}

	if got := ap.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
