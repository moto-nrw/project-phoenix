package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type captchaSettingsStub struct {
	hasOverride bool
	value       bool
}

func (s captchaSettingsStub) HasTenantOverride(context.Context, string) (bool, error) {
	return s.hasOverride, nil
}

func (s captchaSettingsStub) ResolveBool(context.Context, string) (bool, error) {
	return s.value, nil
}

func (s captchaSettingsStub) ResolveString(context.Context, string) (string, error) {
	return "", nil
}

func TestCaptchaServiceIsEnabledDefaultsToFalseWithoutTenantOverride(t *testing.T) {
	t.Parallel()

	svc := NewCaptchaService(CaptchaServiceConfig{
		Settings: captchaSettingsStub{hasOverride: false},
	})

	require.False(t, svc.IsEnabled(context.Background()))
	require.NoError(t, svc.Verify(context.Background(), "", ""))
}

func TestCaptchaServiceIsEnabledHonorsTenantOverride(t *testing.T) {
	t.Parallel()

	svc := NewCaptchaService(CaptchaServiceConfig{
		Settings: captchaSettingsStub{hasOverride: true, value: true},
	})

	require.True(t, svc.IsEnabled(context.Background()))
	err := svc.Verify(context.Background(), "", "")
	require.ErrorContains(t, err, "captcha secret key not configured")
}

func TestCaptchaServiceIsEnabledFallsBackToEnvWhenNoTenantOverride(t *testing.T) {
	t.Parallel()

	svc := NewCaptchaService(CaptchaServiceConfig{
		Settings:       captchaSettingsStub{hasOverride: false},
		RequireCaptcha: true,
	})

	require.True(t, svc.IsEnabled(context.Background()))
}

var _ CaptchaSettingsResolver = captchaSettingsStub{}
