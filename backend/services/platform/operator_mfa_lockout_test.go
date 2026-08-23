package platform

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// Moved from models/platform/mfa_models_test.go (TestOperator_IsMFALocked)
// when the lockout decision left the model in issue #586 (Rule 12). The
// decision is now a pure operator read with the clock injected, so no repos
// are needed.
func TestOperatorMFAService_isMFALocked(t *testing.T) {
	t.Parallel()

	s := &operatorMFAService{}
	now := time.Now()

	tests := []struct {
		name           string
		mfaLockedUntil *time.Time
		expected       bool
	}{
		{name: "nil locked until", mfaLockedUntil: nil, expected: false},
		{name: "locked until past", mfaLockedUntil: ptrTime(now.Add(-time.Minute)), expected: false},
		{name: "locked until future", mfaLockedUntil: ptrTime(now.Add(time.Minute)), expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := &platform.Operator{MFALockedUntil: tt.mfaLockedUntil}
			if got := s.isMFALocked(op, now); got != tt.expected {
				t.Errorf("isMFALocked() = %v, want %v", got, tt.expected)
			}
		})
	}

	// Nil operator is treated as unlocked.
	if s.isMFALocked(nil, now) {
		t.Error("isMFALocked(nil) should return false")
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
