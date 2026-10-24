package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captchaSettingsStub returns the resolved tenant requirement.
type captchaSettingsStub struct {
	EnrollmentSettingsReads
	required bool
	err      error
}

func (s captchaSettingsStub) ResolveBool(context.Context, string) (bool, error) {
	return s.required, s.err
}

func TestEnrollmentCaptchaSettingsUseRegistryDefaults(t *testing.T) {
	t.Parallel()
	settings := EnrollmentCaptchaSettings{Settings: captchaSettingsStub{}}

	required, err := settings.CaptchaRequired(context.Background())
	assert.NoError(t, err)
	assert.False(t, required)
}

// The tenant decides whether the captcha is required; the keys are the
// platform's, whatever the tenant resolves.
func TestEnrollmentCaptchaSettingsServePlatformKeys(t *testing.T) {
	t.Parallel()
	settings := EnrollmentCaptchaSettings{
		Settings: captchaSettingsStub{required: true}, SiteKey: "platform-site", SecretKey: "platform-secret",
	}

	required, err := settings.CaptchaRequired(context.Background())
	assert.NoError(t, err)
	assert.True(t, required)
	secret, err := settings.CaptchaSecretKey(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "platform-secret", secret)
	siteKey, err := settings.CaptchaSiteKey(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "platform-site", siteKey)
}

func TestEnrollmentCaptchaSettingsPropagateResolutionErrors(t *testing.T) {
	t.Parallel()
	failure := errors.New("settings unavailable")
	settings := EnrollmentCaptchaSettings{Settings: captchaSettingsStub{err: failure}}
	_, err := settings.CaptchaRequired(context.Background())
	assert.ErrorIs(t, err, failure)
}
