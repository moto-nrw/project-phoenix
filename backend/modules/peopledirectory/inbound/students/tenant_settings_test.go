package students

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/stretchr/testify/assert"
)

// TestTenantSettingKeysMatchTheRegistry pins every key and value this
// resource reads to the Settings Platform's public name: a key renamed in the
// registry must fail here, not resolve as an unknown definition at runtime.
func TestTenantSettingKeysMatchTheRegistry(t *testing.T) {
	t.Parallel()

	assert.Equal(t, settings.KeyAttendanceLogEnabled, settingAttendanceLogEnabled)
	assert.Equal(t, settings.KeyAttendanceVisibleDays, settingAttendanceVisibleDays)
	assert.Equal(t, settings.KeyRoomDetailVisibleDays, settingRoomDetailVisibleDays)
	assert.Equal(t, settings.KeyPrivacyConsentRetentionDays, settingPrivacyConsentRetentionDays)
	assert.Equal(t, settings.KeyEnrollmentBookingsAuthoritative, settingEnrollmentBookingsAuthoritative)
	assert.Equal(t, settings.KeyCareDefaultArrivalTime, settingCareDefaultArrivalTime)
	assert.Equal(t, settings.KeyCareDefaultPickupTime, settingCareDefaultPickupTime)
	assert.Equal(t, settings.KeyParentRequestReasonPolicy, settingParentRequestReasonPolicy)
	assert.Equal(t, settings.KeySessionEndTime, settingSessionEndTime)
	assert.Equal(t, settings.KeyStudentAbsenceEditScope, settingStudentAbsenceEditScope)
	assert.Equal(t, settings.KeyPresenceMode, settingPresenceMode)
	assert.Equal(t, settings.KeyStudentPhotosEnabled, settingStudentPhotosEnabled)
	assert.Equal(t, settings.KeyClassArrivalExceptionEditors, settingClassArrivalExceptionEditors)
	assert.Equal(t, settings.KeyFeedbackEnabled, settingFeedbackEnabled)
	assert.Equal(t, settings.StudentAbsenceEditScopeAllStaff, studentAbsenceEditScopeAllStaff)
	assert.Equal(t, settings.ClassArrivalExceptionEditorsAllStaff, classArrivalExceptionEditorsAllStaff)
	assert.Equal(t, settings.SchoolPeriodCount, schoolPeriodCount)
	for period := 1; period <= schoolPeriodCount; period++ {
		assert.Equal(t, settings.SchoolPeriodEndKey(period), schoolPeriodEndSetting(period))
	}
}

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
