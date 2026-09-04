package active

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
)

// These unit tests cover the active-session lifecycle policy that moved off the
// models (Rule 12). The clock is injected as `now` everywhere, so the tests are
// deterministic and need no DB or sleeps.

func ptrTime(t time.Time) *time.Time { return &t }

func ptrDate(d timezone.Date) *timezone.Date { return &d }

func TestSessionInactivityDuration(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	group := &activeModels.Group{LastActivity: now.Add(-15 * time.Minute)}
	assert.Equal(t, 15*time.Minute, SessionInactivityDuration(group, now))
}

func TestIsSupervisorActive(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	assert.False(t, IsSupervisorActive(&activeModels.GroupSupervisor{StartDate: timezone.DateFromTime(now).AddDays(1)}, now),
		"a future supervision has not started yet")
	assert.True(t, IsSupervisorActive(&activeModels.GroupSupervisor{EndDate: nil}, now),
		"nil end date is open-ended and active")
	assert.True(t, IsSupervisorActive(&activeModels.GroupSupervisor{EndDate: ptrDate(timezone.DateFromTime(now).AddDays(1))}, now),
		"future end date is still active")
	assert.False(t, IsSupervisorActive(&activeModels.GroupSupervisor{EndDate: ptrDate(timezone.DateFromTime(now).AddDays(-1))}, now),
		"past end date is not active")
}

func TestIsCombinedGroupActive(t *testing.T) {
	t.Parallel()

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	assert.True(t, IsCombinedGroupActive(&activeModels.CombinedGroup{EndTime: nil}, now),
		"nil end time is open-ended and active")
	assert.True(t, IsCombinedGroupActive(&activeModels.CombinedGroup{EndTime: ptrTime(now.Add(time.Hour))}, now),
		"future end time is still active")
	assert.False(t, IsCombinedGroupActive(&activeModels.CombinedGroup{EndTime: ptrTime(now.Add(-time.Hour))}, now),
		"past end time is not active")
}

func TestResolveSessionTimeout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("per-session TimeoutMinutes wins", func(t *testing.T) {
		s := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default()}}
		group := &activeModels.Group{TimeoutMinutes: 45}
		assert.Equal(t, 45*time.Minute, s.ResolveSessionTimeout(ctx, group))
	})

	t.Run("nil settings falls back to default", func(t *testing.T) {
		s := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default()}}
		group := &activeModels.Group{TimeoutMinutes: 0}
		assert.Equal(t, DefaultSessionInactivityTimeout, s.ResolveSessionTimeout(ctx, group))
	})

	t.Run("tenant override resolved from settings", func(t *testing.T) {
		s := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default()}, settings: &stubSettingsResolver{
			intValues: map[string]int{configModel.KeySessionInactivityTimeoutMin: 20},
		},
		}
		group := &activeModels.Group{TimeoutMinutes: 0}
		assert.Equal(t, 20*time.Minute, s.ResolveSessionTimeout(ctx, group))
	})

	t.Run("zero from settings degrades to default", func(t *testing.T) {
		s := &service{ServiceDependencies: ServiceDependencies{Logger: slog.Default()}, settings: &stubSettingsResolver{
			intValues: map[string]int{configModel.KeySessionInactivityTimeoutMin: 0},
		},
		}
		group := &activeModels.Group{TimeoutMinutes: 0}
		assert.Equal(t, DefaultSessionInactivityTimeout, s.ResolveSessionTimeout(ctx, group))
	})
}

func TestIsSessionTimedOut(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	s := &service{ServiceDependencies: ServiceDependencies{ // nil settings → 30 min default
		Logger: slog.Default()}}

	t.Run("active session within window is not timed out", func(t *testing.T) {
		group := &activeModels.Group{LastActivity: now.Add(-10 * time.Minute)}
		assert.False(t, s.IsSessionTimedOut(ctx, group, now))
	})

	t.Run("active session past window is timed out", func(t *testing.T) {
		group := &activeModels.Group{LastActivity: now.Add(-35 * time.Minute)}
		assert.True(t, s.IsSessionTimedOut(ctx, group, now))
	})

	t.Run("exactly at threshold is timed out", func(t *testing.T) {
		group := &activeModels.Group{LastActivity: now.Add(-30 * time.Minute)}
		assert.True(t, s.IsSessionTimedOut(ctx, group, now))
	})

	t.Run("ended session is never timed out", func(t *testing.T) {
		ended := now.Add(-5 * time.Minute)
		group := &activeModels.Group{
			LastActivity: now.Add(-35 * time.Minute),
			EndTime:      &ended,
		}
		assert.False(t, s.IsSessionTimedOut(ctx, group, now))
	})

	t.Run("per-session timeout respected", func(t *testing.T) {
		group := &activeModels.Group{
			LastActivity:   now.Add(-50 * time.Minute),
			TimeoutMinutes: 60,
		}
		assert.False(t, s.IsSessionTimedOut(ctx, group, now))
	})
}

func TestSessionTimeUntilTimeout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	s := &service{ServiceDependencies: ServiceDependencies{ // nil settings → 30 min default
		Logger: slog.Default()}}

	t.Run("positive when within window", func(t *testing.T) {
		group := &activeModels.Group{LastActivity: now.Add(-20 * time.Minute)}
		assert.Equal(t, 10*time.Minute, s.SessionTimeUntilTimeout(ctx, group, now))
	})

	t.Run("negative when past window", func(t *testing.T) {
		group := &activeModels.Group{LastActivity: now.Add(-35 * time.Minute)}
		assert.Equal(t, -5*time.Minute, s.SessionTimeUntilTimeout(ctx, group, now))
	})
}
