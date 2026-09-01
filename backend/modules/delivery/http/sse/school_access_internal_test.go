package sse

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type stubSchoolAccess struct {
	allowed bool
	err     error
	calls   int
}

func (s *stubSchoolAccess) HasSchoolPortalAccess(context.Context, int64, int64) (bool, error) {
	s.calls++
	return s.allowed, s.err
}

func TestSchoolAccessGateAllow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		checker   *stubSchoolAccess
		lastCheck time.Duration // age of the last successful verification
		want      bool
		wantCalls int
	}{
		{
			name:      "fresh verification is trusted without a query",
			checker:   &stubSchoolAccess{allowed: true},
			lastCheck: schoolAccessRecheckInterval / 2,
			want:      true,
			wantCalls: 0,
		},
		{
			name:      "stale verification re-checks and keeps serving",
			checker:   &stubSchoolAccess{allowed: true},
			lastCheck: schoolAccessRecheckInterval + time.Second,
			want:      true,
			wantCalls: 1,
		},
		{
			name:      "revoked role closes the stream",
			checker:   &stubSchoolAccess{allowed: false},
			lastCheck: schoolAccessRecheckInterval + time.Second,
			want:      false,
			wantCalls: 1,
		},
		{
			name:      "transient error keeps serving inside the grace period",
			checker:   &stubSchoolAccess{err: errors.New("db down")},
			lastCheck: schoolAccessRecheckInterval + time.Second,
			want:      true,
			wantCalls: 1,
		},
		{
			name:      "persistent error closes the stream after the grace period",
			checker:   &stubSchoolAccess{err: errors.New("db down")},
			lastCheck: schoolAccessGracePeriod + time.Second,
			want:      false,
			wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gate := &schoolAccessGate{
				checker:    tt.checker,
				accountID:  42,
				tenantID:   7,
				verifiedAt: now.Add(-tt.lastCheck),
				logger:     slog.Default(),
			}

			assert.Equal(t, tt.want, gate.allow(context.Background(), now))
			assert.Equal(t, tt.wantCalls, tt.checker.calls)
		})
	}
}

// A successful re-check must reset the window, so the next delivery inside the
// interval does not query again.
func TestSchoolAccessGateResetsWindow(t *testing.T) {
	t.Parallel()

	checker := &stubSchoolAccess{allowed: true}
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	gate := &schoolAccessGate{
		checker:    checker,
		accountID:  42,
		tenantID:   7,
		verifiedAt: now.Add(-2 * schoolAccessRecheckInterval),
		logger:     slog.Default(),
	}

	assert.True(t, gate.allow(context.Background(), now))
	assert.True(t, gate.allow(context.Background(), now.Add(schoolAccessRecheckInterval/2)))
	assert.Equal(t, 1, checker.calls)
}
