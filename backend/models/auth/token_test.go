package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestToken_Validate(t *testing.T) {
	futureTime := time.Now().Add(time.Hour)
	pastTime := time.Now().Add(-time.Hour)

	tests := []struct {
		name    string
		token   *Token
		wantErr bool
	}{
		{
			name: "valid token",
			token: &Token{
				AccountID: 1,
				Token:     "valid-token-string",
				Expiry:    futureTime,
			},
			wantErr: false,
		},
		{
			name: "valid token with all fields",
			token: &Token{
				AccountID:  1,
				Token:      "valid-token-string",
				Expiry:     futureTime,
				Mobile:     true,
				Identifier: base.StringPtr("device-123"),
				FamilyID:   "family-abc",
				Generation: 1,
			},
			wantErr: false,
		},
		{
			name: "missing account ID",
			token: &Token{
				AccountID: 0,
				Token:     "valid-token-string",
				Expiry:    futureTime,
			},
			wantErr: true,
		},
		{
			name: "negative account ID",
			token: &Token{
				AccountID: -1,
				Token:     "valid-token-string",
				Expiry:    futureTime,
			},
			wantErr: true,
		},
		{
			name: "missing token value",
			token: &Token{
				AccountID: 1,
				Token:     "",
				Expiry:    futureTime,
			},
			wantErr: true,
		},
		{
			name: "expired token",
			token: &Token{
				AccountID: 1,
				Token:     "expired-token",
				Expiry:    pastTime,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Token.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToken_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		token    *Token
		expected bool
	}{
		{
			name: "not expired - future expiry",
			token: &Token{
				AccountID: 1,
				Token:     "token",
				Expiry:    time.Now().Add(time.Hour),
			},
			expected: false,
		},
		{
			name: "expired - past expiry",
			token: &Token{
				AccountID: 1,
				Token:     "token",
				Expiry:    time.Now().Add(-time.Hour),
			},
			expected: true,
		},
		{
			name: "expired - zero time",
			token: &Token{
				AccountID: 1,
				Token:     "token",
				Expiry:    time.Time{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.token.IsExpired()
			if got != tt.expected {
				t.Errorf("Token.IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestToken_SetExpiry(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{
			name:     "15 minute expiry",
			duration: 15 * time.Minute,
		},
		{
			name:     "1 hour expiry",
			duration: time.Hour,
		},
		{
			name:     "24 hour expiry",
			duration: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &Token{
				AccountID: 1,
				Token:     "token",
			}

			before := time.Now()
			token.SetExpiry(tt.duration)
			after := time.Now()

			expectedMin := before.Add(tt.duration)
			expectedMax := after.Add(tt.duration)

			if token.Expiry.Before(expectedMin) || token.Expiry.After(expectedMax) {
				t.Errorf("Token.SetExpiry() expiry = %v, want between %v and %v",
					token.Expiry, expectedMin, expectedMax)
			}
		})
	}
}

func TestToken_MobileFlag(t *testing.T) {
	t.Run("default is false", func(t *testing.T) {
		token := &Token{
			AccountID: 1,
			Token:     "token",
			Expiry:    time.Now().Add(time.Hour),
		}

		if token.Mobile != false {
			t.Error("Token.Mobile should default to false")
		}
	})

	t.Run("can be set to true", func(t *testing.T) {
		token := &Token{
			AccountID: 1,
			Token:     "token",
			Expiry:    time.Now().Add(time.Hour),
			Mobile:    true,
		}

		if token.Mobile != true {
			t.Error("Token.Mobile should be true when set")
		}
	})
}

func TestToken_FamilyTracking(t *testing.T) {
	t.Run("token family fields", func(t *testing.T) {
		token := &Token{
			AccountID:  1,
			Token:      "token",
			Expiry:     time.Now().Add(time.Hour),
			FamilyID:   "family-123",
			Generation: 3,
		}

		if token.FamilyID != "family-123" {
			t.Errorf("Token.FamilyID = %q, want %q", token.FamilyID, "family-123")
		}

		if token.Generation != 3 {
			t.Errorf("Token.Generation = %d, want %d", token.Generation, 3)
		}
	})

	t.Run("default generation is 0", func(t *testing.T) {
		token := &Token{
			AccountID: 1,
			Token:     "token",
			Expiry:    time.Now().Add(time.Hour),
		}

		if token.Generation != 0 {
			t.Errorf("Token.Generation should default to 0, got %d", token.Generation)
		}
	})
}
