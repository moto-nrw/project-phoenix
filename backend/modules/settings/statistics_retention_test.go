package settings

import (
	"context"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retentionSource map[string]int

func (s retentionSource) ResolveInt(_ context.Context, key string) (int, error) {
	value, ok := s[key]
	if !ok {
		return 0, errors.New("unknown key " + key)
	}
	return value, nil
}

func TestRetentionWindowsReadTheirSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	source := retentionSource{configModel.KeyPrivacyConsentRetentionDays: 30, configModel.KeyGDPRTimetableRetentionDays: 90, configModel.KeyGDPRStatusDayRetentionDays: 60}
	rooms, err := NewStatisticsRetention(source).RoomRetentionDays(ctx)
	require.NoError(t, err)
	assert.Equal(t, 30, rooms)
	courses, err := NewStatisticsRetention(source).CourseRetentionDays(ctx)
	require.NoError(t, err)
	assert.Equal(t, 90, courses)
	days, err := NewStatusDayRetention(source).Days(ctx)
	require.NoError(t, err)
	assert.Equal(t, 60, days)

	_, err = NewStatusDayRetention(retentionSource{}).Days(ctx)
	require.ErrorContains(t, err, "resolve "+configModel.KeyGDPRStatusDayRetentionDays+": ")
	refused := errors.New("window")
	require.ErrorIs(t, NewStatusDayRetention(source).SettingError(refused), refused)
}

func TestAttendanceRetentionReadsItsSetting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	days, err := NewAttendanceRetention(retentionSource{configModel.KeyGDPRAttendanceRetentionDays: 120}).Days(ctx)
	require.NoError(t, err)
	assert.Equal(t, 120, days)

	_, err = NewAttendanceRetention(retentionSource{}).Days(ctx)
	require.ErrorContains(t, err, "resolve "+configModel.KeyGDPRAttendanceRetentionDays+": ")
	refused := errors.New("window")
	require.ErrorIs(t, NewAttendanceRetention(retentionSource{}).SettingError(refused), refused)
}

func TestGuardianRetentionReadsItsSetting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	days, err := NewGuardianRetention(retentionSource{configModel.KeyGDPRGuardianRetentionDays: 60}).Days(ctx)
	require.NoError(t, err)
	assert.Equal(t, 60, days)

	_, err = NewGuardianRetention(retentionSource{}).Days(ctx)
	require.ErrorContains(t, err, "resolve "+configModel.KeyGDPRGuardianRetentionDays+": ")
	refused := errors.New("window")
	require.ErrorIs(t, NewGuardianRetention(retentionSource{}).SettingError(refused), refused)
}
