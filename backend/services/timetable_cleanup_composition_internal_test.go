package services

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retentionSettingsProbe struct {
	overrideErr   error
	value         int
	resolveErr    error
	overrideKeys  []string
	resolvedKeys  []string
	overrideCalls int
}

func (p *retentionSettingsProbe) HasTenantOverride(_ context.Context, key string) (bool, error) {
	p.overrideCalls++
	p.overrideKeys = append(p.overrideKeys, key)
	return p.overrideErr == nil, p.overrideErr
}

func (p *retentionSettingsProbe) ResolveInt(_ context.Context, key string) (int, error) {
	p.resolvedKeys = append(p.resolvedKeys, key)
	return p.value, p.resolveErr
}

// The Timetable retention window resolves gdpr.timetable_retention_days
// through the settings service: tenant override, else registry default.
func TestTimetableRetentionSettingsResolveTheRetentionKey(t *testing.T) {
	t.Parallel()
	probe := &retentionSettingsProbe{value: 42}
	settings := timetableRetentionSettings{settings: probe, logger: slog.New(slog.DiscardHandler)}

	days, err := settings.TimetableRetentionDays(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 42, days)
	assert.Equal(t, []string{"gdpr.timetable_retention_days"}, probe.overrideKeys)
	assert.Equal(t, []string{"gdpr.timetable_retention_days"}, probe.resolvedKeys)
}

// A failed override check is only logged: the resolution still answers with
// the registry default.
func TestTimetableRetentionSettingsSurviveAFailedOverrideCheck(t *testing.T) {
	t.Parallel()
	probe := &retentionSettingsProbe{overrideErr: errors.New("settings DB down"), value: 365}
	settings := timetableRetentionSettings{settings: probe, logger: slog.New(slog.DiscardHandler)}

	days, err := settings.TimetableRetentionDays(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 365, days)
	assert.Equal(t, 1, probe.overrideCalls)

	wantErr := errors.New("resolve failed")
	probe.resolveErr = wantErr
	_, err = settings.TimetableRetentionDays(context.Background())
	require.ErrorIs(t, err, wantErr)
}
