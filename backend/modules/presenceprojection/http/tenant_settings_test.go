package presenceprojectionhttp

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

type failingTenantSettings struct{ TenantSettings }

func (failingTenantSettings) ResolveBool(context.Context, string) (bool, error) {
	return false, errors.New("settings unavailable")
}

func (failingTenantSettings) ResolveInt(context.Context, string) (int, error) {
	return 0, errors.New("settings unavailable")
}

// TestResolveSettingFallsBackOnlyWhenTheReadFails: a resolved value wins,
// including the registry default the port returns without an override; the
// caller's fallback answers only a missing port or a failed read.
func TestResolveSettingFallsBackOnlyWhenTheReadFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)
	resolved := stubTenantSettings{bools: map[string]bool{settingFeedbackEnabled: false}, ints: map[string]int{settingAttendanceVisibleDays: 12}}

	assert.False(t, resolveBoolSetting(ctx, resolved, settingFeedbackEnabled, true, logger))
	assert.Equal(t, 12, resolveIntSetting(ctx, resolved, settingAttendanceVisibleDays, 30, logger))
	assert.True(t, resolveBoolSetting(ctx, nil, settingFeedbackEnabled, true, logger))
	assert.Equal(t, 30, resolveIntSetting(ctx, nil, settingAttendanceVisibleDays, 30, logger))
	assert.True(t, resolveBoolSetting(ctx, failingTenantSettings{}, settingFeedbackEnabled, true, logger))
	assert.Equal(t, 30, resolveIntSetting(ctx, failingTenantSettings{}, settingAttendanceVisibleDays, 30, logger))
}

type stubTenantSettings struct {
	bools map[string]bool
	ints  map[string]int
}

func (s stubTenantSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	return s.bools[key], nil
}

func (s stubTenantSettings) ResolveInt(_ context.Context, key string) (int, error) {
	return s.ints[key], nil
}

func (stubTenantSettings) ResolveString(context.Context, string) (string, error) {
	return "", nil
}
