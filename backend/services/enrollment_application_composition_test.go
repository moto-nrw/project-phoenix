package services

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captchaSettingsStub answers the captcha settings keys: an override exists
// for the flag and the secret independently, and ResolveString returns the
// stored secret.
type captchaSettingsStub struct {
	enrollmentSettingsReads
	requireOverride bool
	required        bool
	secretOverride  bool
	secret          string
}

func (s captchaSettingsStub) HasTenantOverride(_ context.Context, key string) (bool, error) {
	if strings.Contains(key, "captcha_secret_key") {
		return s.secretOverride, nil
	}
	return s.requireOverride, nil
}

func (s captchaSettingsStub) ResolveBool(context.Context, string) (bool, error) {
	return s.required, nil
}

func (s captchaSettingsStub) ResolveString(context.Context, string) (string, error) {
	return s.secret, nil
}

func TestEnrollmentCaptchaSettingsDefaultToDisabledWithoutTenantOverride(t *testing.T) {
	t.Parallel()
	settings := enrollmentCaptchaSettings{settings: captchaSettingsStub{}}

	assert.False(t, settings.CaptchaRequired(context.Background()))
}

func TestEnrollmentCaptchaSettingsHonorTheTenantOverride(t *testing.T) {
	t.Parallel()
	settings := enrollmentCaptchaSettings{settings: captchaSettingsStub{requireOverride: true, required: true}}

	assert.True(t, settings.CaptchaRequired(context.Background()))
}

func TestEnrollmentCaptchaSettingsFallBackToTheDeployment(t *testing.T) {
	t.Parallel()
	settings := enrollmentCaptchaSettings{settings: captchaSettingsStub{}, requireCaptcha: true, secretKey: "env-secret"}

	assert.True(t, settings.CaptchaRequired(context.Background()))
	assert.Equal(t, "env-secret", settings.CaptchaSecretKey(context.Background()),
		"the deployment secret must be used when no tenant override exists")
}

func TestEnrollmentCaptchaSettingsPreferTheTenantSecret(t *testing.T) {
	t.Parallel()
	settings := enrollmentCaptchaSettings{
		settings:  captchaSettingsStub{secretOverride: true, secret: "tenant-wins"},
		secretKey: "env-secret",
	}

	assert.Equal(t, "tenant-wins", settings.CaptchaSecretKey(context.Background()),
		"the tenant override takes precedence over the deployment secret")
}
