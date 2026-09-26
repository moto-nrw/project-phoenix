package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captchaSettingsStub returns resolved tenant values or registry defaults.
type captchaSettingsStub struct {
	enrollmentSettingsReads
	required bool
	secret   string
	siteKey  string
	err      error
}

func (s captchaSettingsStub) ResolveBool(context.Context, string) (bool, error) {
	return s.required, s.err
}

func (s captchaSettingsStub) ResolveString(_ context.Context, key string) (string, error) {
	if key == "enrollment.captcha_site_key" {
		return s.siteKey, s.err
	}
	return s.secret, s.err
}

func TestEnrollmentCaptchaSettingsUseRegistryDefaults(t *testing.T) {
	t.Parallel()
	settings := enrollmentCaptchaSettings{settings: captchaSettingsStub{}}

	required, err := settings.CaptchaRequired(context.Background())
	assert.NoError(t, err)
	assert.False(t, required)
	secret, err := settings.CaptchaSecretKey(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, secret)
}

func TestEnrollmentCaptchaSettingsUseResolvedValues(t *testing.T) {
	t.Parallel()
	settings := enrollmentCaptchaSettings{settings: captchaSettingsStub{required: true, secret: "tenant-secret", siteKey: "tenant-site"}}

	required, err := settings.CaptchaRequired(context.Background())
	assert.NoError(t, err)
	assert.True(t, required)
	secret, err := settings.CaptchaSecretKey(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "tenant-secret", secret)
	siteKey, err := settings.CaptchaSiteKey(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "tenant-site", siteKey)
}

func TestEnrollmentCaptchaSettingsPropagateResolutionErrors(t *testing.T) {
	t.Parallel()
	failure := errors.New("settings unavailable")
	settings := enrollmentCaptchaSettings{settings: captchaSettingsStub{err: failure}}
	_, err := settings.CaptchaRequired(context.Background())
	assert.ErrorIs(t, err, failure)
	_, err = settings.CaptchaSecretKey(context.Background())
	assert.ErrorIs(t, err, failure)
	_, err = settings.CaptchaSiteKey(context.Background())
	assert.ErrorIs(t, err, failure)
}
