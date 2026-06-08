package auth

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// Moved from models/auth/account_test.go (TestAccount_IsPINLocked) when the
// lockout decision left the model in issue #586 (Rule 12). The decision is now
// a pure account read with the clock injected, so no repos are needed.
func TestService_IsPINLocked(t *testing.T) {
	s := &Service{}
	now := time.Now()

	tests := []struct {
		name           string
		pinLockedUntil *time.Time
		expected       bool
	}{
		{name: "nil locked until", pinLockedUntil: nil, expected: false},
		{name: "locked until past", pinLockedUntil: ptr(now.Add(-time.Hour)), expected: false},
		{name: "locked until future", pinLockedUntil: ptr(now.Add(time.Hour)), expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &auth.Account{PINLockedUntil: tt.pinLockedUntil}
			if got := s.IsPINLocked(account, now); got != tt.expected {
				t.Errorf("IsPINLocked() = %v, want %v", got, tt.expected)
			}
		})
	}

	// Nil account is treated as unlocked.
	if s.IsPINLocked(nil, now) {
		t.Error("IsPINLocked(nil) should return false")
	}
}

func ptr(t time.Time) *time.Time { return &t }
