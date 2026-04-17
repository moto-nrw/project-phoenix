package active

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeSettingsResolver is a minimal stub that satisfies the SettingsResolver
// interface. Each field is independently settable so tests can script the
// full branch space of resolveClearMode without pulling in the real service.
type fakeSettingsResolver struct {
	hasOverride    bool
	hasOverrideErr error
	resolved       string
	resolveErr     error
}

func (f *fakeSettingsResolver) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return f.hasOverride, f.hasOverrideErr
}

func (f *fakeSettingsResolver) ResolveString(_ context.Context, _ string) (string, error) {
	return f.resolved, f.resolveErr
}

// TestResolveClearMode_NilSettings — when no SettingsResolver is injected the
// fallback value is returned untouched.
func TestResolveClearMode_NilSettings(t *testing.T) {
	s := &service{}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "next_checkin")
	assert.Equal(t, "next_checkin", got)
}

// TestResolveClearMode_HasOverrideError — override check failure must not
// crash and must degrade gracefully to the fallback.
func TestResolveClearMode_HasOverrideError(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{
			hasOverrideErr: errors.New("db error"),
		},
	}
	got := s.resolveClearMode(context.Background(), "operations.excused_clear_mode", "end_of_day")
	assert.Equal(t, "end_of_day", got)
}

// TestResolveClearMode_NoOverride — when the tenant has not set a value the
// service returns the fallback instead of the registry default (because
// ResolveString would return the registry default, which the caller wants to
// distinguish from a real override).
func TestResolveClearMode_NoOverride(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{hasOverride: false},
	}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "next_checkin")
	assert.Equal(t, "next_checkin", got)
}

// TestResolveClearMode_ResolveStringError — override exists but read fails;
// fall through to the caller-supplied fallback.
func TestResolveClearMode_ResolveStringError(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{
			hasOverride: true,
			resolveErr:  errors.New("boom"),
		},
	}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "manual")
	assert.Equal(t, "manual", got)
}

// TestResolveClearMode_EmptyString — an empty override is treated as absent so
// the caller's fallback wins (matches the scheduler's resolveStringSetting
// contract).
func TestResolveClearMode_EmptyString(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{
			hasOverride: true,
			resolved:    "",
		},
	}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "manual")
	assert.Equal(t, "manual", got)
}

// TestResolveClearMode_OverrideReturned — the tenant value wins when
// everything succeeds.
func TestResolveClearMode_OverrideReturned(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{
			hasOverride: true,
			resolved:    "end_of_day",
		},
	}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "next_checkin")
	assert.Equal(t, "end_of_day", got)
}
