package settings

import (
	"context"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// intakeSource returns resolved values, including registered defaults when unset.
type intakeSource struct {
	overrides map[string]any
	fail      error
}

func (s intakeSource) HasTenantOverride(_ context.Context, key string) (bool, error) {
	if s.fail != nil {
		return false, s.fail
	}
	_, ok := s.overrides[key]
	return ok, nil
}

func (s intakeSource) ResolveBool(_ context.Context, key string) (bool, error) {
	value, ok := s.overrides[key].(bool)
	if !ok && (key == configModel.KeyEnrollmentAllowSubmissionEdit || key == configModel.KeyModuleOnlineAnmeldung) {
		value = true
	}
	return value, s.fail
}

func (s intakeSource) ResolveString(_ context.Context, key string) (string, error) {
	value, _ := s.overrides[key].(string)
	return value, s.fail
}

func (s intakeSource) ResolveInt(_ context.Context, key string) (int, error) {
	value, ok := s.overrides[key].(int)
	if !ok && key == configModel.KeyEnrollmentStatusTokenTTLDays {
		value = 365
	}
	return value, s.fail
}

func TestEnrollmentIntakeValuesUseResolvedDefaultsAndFailClosed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, source := range map[string]intakeSource{"unset": {}, "failing": {fail: errors.New("down")}} {
		values := NewEnrollmentIntakeValues(source)
		assert.False(t, values.EnrollmentEnabled(ctx), name)
		allowed, editErr := values.AllowSubmissionEdit(ctx)
		if source.fail != nil {
			require.ErrorIs(t, editErr, source.fail)
		} else {
			require.NoError(t, editErr)
			assert.True(t, allowed, name)
		}
		days, err := values.StatusTokenTTLDays(ctx)
		if source.fail != nil {
			require.ErrorIs(t, err, source.fail)
		} else {
			require.NoError(t, err)
			assert.Equal(t, 365, days, name)
		}
		assert.Empty(t, values.AdminNotificationEmails(ctx), name)
		assert.False(t, values.ChangeRequestMailsEnabled(ctx), name)
	}

	values := NewEnrollmentIntakeValues(intakeSource{overrides: map[string]any{
		configModel.KeyEnrollmentEnabled: true, configModel.KeyEnrollmentAllowSubmissionEdit: false,
		configModel.KeyEnrollmentStatusTokenTTLDays: 30, configModel.KeyEnrollmentNotificationEmails: "a@b.test",
		configModel.KeyEnrollmentChangeRequestEmailNotificationsEnabled: true,
	}})
	assert.True(t, values.EnrollmentEnabled(ctx))
	allowed, editErr := values.AllowSubmissionEdit(ctx)
	require.NoError(t, editErr)
	assert.False(t, allowed)
	days, err := values.StatusTokenTTLDays(ctx)
	require.NoError(t, err)
	assert.Equal(t, 30, days)
	assert.Equal(t, "a@b.test", values.AdminNotificationEmails(ctx))
	assert.True(t, values.ChangeRequestMailsEnabled(ctx))
}

// A school switch never opens the intake of a cancelled module (#4893).
func TestEnrollmentIntakeStaysClosedWithoutTheModule(t *testing.T) {
	t.Parallel()
	values := NewEnrollmentIntakeValues(intakeSource{overrides: map[string]any{
		configModel.KeyEnrollmentEnabled: true, configModel.KeyModuleOnlineAnmeldung: false,
	}})
	assert.False(t, values.EnrollmentEnabled(context.Background()))
}

func TestEnrollmentLegalTogglesNeedAnOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	legal, err := NewEnrollmentIntakeValues(intakeSource{overrides: map[string]any{
		configModel.KeyEnrollmentLegalAGBText: "AGB", configModel.KeyEnrollmentLegalDSGVOEnabled: true,
	}}).Legal(ctx)
	require.NoError(t, err)
	assert.Equal(t, EnrollmentLegal{AGB: "AGB", DSGVOEnabled: true}, legal, "a toggle without an override stays off")

	_, err = NewEnrollmentIntakeValues(intakeSource{fail: errors.New("down")}).Legal(ctx)
	require.ErrorContains(t, err, "resolve AGB legal text")
}
