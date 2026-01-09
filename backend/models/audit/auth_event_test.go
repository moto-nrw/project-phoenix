package audit

import (
	"testing"
	"time"
)

func TestAuthEvent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ae      *AuthEvent
		wantErr bool
	}{
		{
			name: "valid login event",
			ae: &AuthEvent{
				AccountID: 1,
				EventType: EventTypeLogin,
				Success:   true,
				IPAddress: "192.168.1.1",
			},
			wantErr: false,
		},
		{
			name: "valid logout event",
			ae: &AuthEvent{
				AccountID: 1,
				EventType: EventTypeLogout,
				Success:   true,
				IPAddress: "10.0.0.1",
			},
			wantErr: false,
		},
		{
			name: "valid token refresh",
			ae: &AuthEvent{
				AccountID: 1,
				EventType: EventTypeTokenRefresh,
				Success:   true,
				IPAddress: "127.0.0.1",
			},
			wantErr: false,
		},
		{
			name: "valid token expired",
			ae: &AuthEvent{
				AccountID: 1,
				EventType: EventTypeTokenExpired,
				Success:   false,
				IPAddress: "192.168.0.100",
			},
			wantErr: false,
		},
		{
			name: "valid password reset",
			ae: &AuthEvent{
				AccountID: 1,
				EventType: EventTypePasswordReset,
				Success:   true,
				IPAddress: "10.10.10.10",
			},
			wantErr: false,
		},
		{
			name: "valid account locked",
			ae: &AuthEvent{
				AccountID:    1,
				EventType:    EventTypeAccountLocked,
				Success:      false,
				IPAddress:    "8.8.8.8",
				ErrorMessage: "Too many failed attempts",
			},
			wantErr: false,
		},
		{
			name: "valid failed login with error message",
			ae: &AuthEvent{
				AccountID:    1,
				EventType:    EventTypeLogin,
				Success:      false,
				IPAddress:    "192.168.1.1",
				ErrorMessage: "Invalid password",
				UserAgent:    "Mozilla/5.0",
			},
			wantErr: false,
		},
		{
			name: "zero account ID",
			ae: &AuthEvent{
				AccountID: 0,
				EventType: EventTypeLogin,
				Success:   true,
				IPAddress: "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "negative account ID",
			ae: &AuthEvent{
				AccountID: -1,
				EventType: EventTypeLogin,
				Success:   true,
				IPAddress: "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "empty event type",
			ae: &AuthEvent{
				AccountID: 1,
				EventType: "",
				Success:   true,
				IPAddress: "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "invalid event type",
			ae: &AuthEvent{
				AccountID: 1,
				EventType: "unknown_event",
				Success:   true,
				IPAddress: "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "empty IP address",
			ae: &AuthEvent{
				AccountID: 1,
				EventType: EventTypeLogin,
				Success:   true,
				IPAddress: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ae.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("AuthEvent.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthEvent_Validate_SetsDefaultCreatedAt(t *testing.T) {
	ae := &AuthEvent{
		AccountID: 1,
		EventType: EventTypeLogin,
		Success:   true,
		IPAddress: "192.168.1.1",
		CreatedAt: time.Time{}, // Zero time
	}

	before := time.Now()
	err := ae.Validate()
	after := time.Now()

	if err != nil {
		t.Fatalf("AuthEvent.Validate() unexpected error = %v", err)
	}

	if ae.CreatedAt.IsZero() {
		t.Error("AuthEvent.Validate() should set CreatedAt when zero")
	}

	if ae.CreatedAt.Before(before) || ae.CreatedAt.After(after) {
		t.Errorf("AuthEvent.CreatedAt = %v, want between %v and %v", ae.CreatedAt, before, after)
	}
}

func TestAuthEvent_Metadata(t *testing.T) {
	t.Run("GetMetadata initializes nil map", func(t *testing.T) {
		ae := &AuthEvent{
			Metadata: nil,
		}

		got := ae.GetMetadata()
		if got == nil {
			t.Error("AuthEvent.GetMetadata() should not return nil")
		}

		if ae.Metadata == nil {
			t.Error("AuthEvent.GetMetadata() should initialize Metadata field")
		}
	})

	t.Run("GetMetadata returns existing map", func(t *testing.T) {
		ae := &AuthEvent{
			Metadata: map[string]interface{}{"browser": "Chrome"},
		}

		got := ae.GetMetadata()
		if got["browser"] != "Chrome" {
			t.Errorf("AuthEvent.GetMetadata() = %v, want map with browser=Chrome", got)
		}
	})

	t.Run("SetMetadata initializes nil map", func(t *testing.T) {
		ae := &AuthEvent{
			Metadata: nil,
		}

		ae.SetMetadata("device", "mobile")

		if ae.Metadata == nil {
			t.Error("AuthEvent.SetMetadata() should initialize Metadata")
		}

		if ae.Metadata["device"] != "mobile" {
			t.Errorf("AuthEvent.Metadata[device] = %v, want mobile", ae.Metadata["device"])
		}
	})

	t.Run("SetMetadata adds to existing map", func(t *testing.T) {
		ae := &AuthEvent{
			Metadata: map[string]interface{}{"existing": "value"},
		}

		ae.SetMetadata("attempt", 3)

		if ae.Metadata["existing"] != "value" {
			t.Error("SetMetadata should preserve existing values")
		}
		if ae.Metadata["attempt"] != 3 {
			t.Errorf("AuthEvent.Metadata[attempt] = %v, want 3", ae.Metadata["attempt"])
		}
	})
}

func TestNewAuthEvent(t *testing.T) {
	before := time.Now()
	ae := NewAuthEvent(42, EventTypeLogin, true, "10.0.0.1")
	after := time.Now()

	if ae.AccountID != 42 {
		t.Errorf("NewAuthEvent().AccountID = %v, want 42", ae.AccountID)
	}

	if ae.EventType != EventTypeLogin {
		t.Errorf("NewAuthEvent().EventType = %v, want %v", ae.EventType, EventTypeLogin)
	}

	if ae.Success != true {
		t.Errorf("NewAuthEvent().Success = %v, want true", ae.Success)
	}

	if ae.IPAddress != "10.0.0.1" {
		t.Errorf("NewAuthEvent().IPAddress = %v, want 10.0.0.1", ae.IPAddress)
	}

	if ae.CreatedAt.Before(before) || ae.CreatedAt.After(after) {
		t.Errorf("NewAuthEvent().CreatedAt = %v, want between %v and %v", ae.CreatedAt, before, after)
	}

	if ae.Metadata == nil {
		t.Error("NewAuthEvent().Metadata should not be nil")
	}
}

func TestEventTypeConstants(t *testing.T) {
	// Verify constants have expected values
	tests := []struct {
		constant string
		expected string
	}{
		{EventTypeLogin, "login"},
		{EventTypeLogout, "logout"},
		{EventTypeTokenRefresh, "token_refresh"},
		{EventTypeTokenExpired, "token_expired"},
		{EventTypePasswordReset, "password_reset"},
		{EventTypeAccountLocked, "account_locked"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("EventType constant = %q, want %q", tt.constant, tt.expected)
		}
	}
}
