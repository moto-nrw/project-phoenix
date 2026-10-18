package settings

import (
	"context"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// intakeSource answers from tenant overrides; a key without one is unset.
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
	value, _ := s.overrides[key].(bool)
	return value, s.fail
}

func (s intakeSource) ResolveString(_ context.Context, key string) (string, error) {
	value, _ := s.overrides[key].(string)
	return value, s.fail
}

func (s intakeSource) ResolveInt(_ context.Context, key string) (int, error) {
	value, _ := s.overrides[key].(int)
	return value, s.fail
}

func TestEnrollmentIntakeValuesFallBackWithoutAnOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, source := range map[string]intakeSource{"unset": {}, "failing": {fail: errors.New("down")}} {
		values := NewEnrollmentIntakeValues(source)
		assert.False(t, values.EnrollmentEnabled(ctx), name)
		assert.True(t, values.AllowSubmissionEdit(ctx), name)
		assert.Equal(t, 365, values.StatusTokenTTLDays(ctx), name)
		assert.Empty(t, values.AdminNotificationEmails(ctx), name)
		assert.False(t, values.ChangeRequestMailsEnabled(ctx), name)
	}

	values := NewEnrollmentIntakeValues(intakeSource{overrides: map[string]any{
		configModel.KeyEnrollmentEnabled: true, configModel.KeyEnrollmentAllowSubmissionEdit: false,
		configModel.KeyEnrollmentStatusTokenTTLDays: 30, configModel.KeyEnrollmentNotificationEmails: "a@b.test",
		configModel.KeyEnrollmentChangeRequestEmailNotificationsEnabled: true,
	}})
	assert.True(t, values.EnrollmentEnabled(ctx))
	assert.False(t, values.AllowSubmissionEdit(ctx))
	assert.Equal(t, 30, values.StatusTokenTTLDays(ctx))
	assert.Equal(t, "a@b.test", values.AdminNotificationEmails(ctx))
	assert.True(t, values.ChangeRequestMailsEnabled(ctx))
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
