package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captchaSettingsStub struct {
	required bool
	secret   string
	siteKey  string
	err      error
}

func (s captchaSettingsStub) CaptchaRequired(context.Context) (bool, error) { return s.required, s.err }
func (s captchaSettingsStub) CaptchaSecretKey(context.Context) (string, error) {
	return s.secret, s.err
}
func (s captchaSettingsStub) CaptchaSiteKey(context.Context) (string, error) { return s.siteKey, s.err }
func (s *captchaProviderStub) calledWith() (string, string, string) {
	return s.secret, s.token, s.remoteIP
}
func newCaptchaForTest(settings CaptchaSettings, provider *captchaProviderStub) *Captcha {
	return NewCaptcha(settings, provider, nil)
}

type captchaProviderStub struct {
	success  bool
	codes    []string
	err      error
	calls    int
	secret   string
	token    string
	remoteIP string
}

func (s *captchaProviderStub) SiteVerify(_ context.Context, secret, token, remoteIP string) (bool, []string, error) {
	s.calls++
	s.secret, s.token, s.remoteIP = secret, token, remoteIP
	return s.success, s.codes, s.err
}

func TestCaptchaDisabledAcceptsEveryToken(t *testing.T) {
	t.Parallel()
	provider := &captchaProviderStub{}
	svc := newCaptchaForTest(captchaSettingsStub{}, provider)

	enabled, err := svc.IsEnabled(context.Background())
	assert.NoError(t, err)
	assert.False(t, enabled)
	// Even a blank token + IP must not error when captcha is off.
	assert.NoError(t, svc.Verify(context.Background(), "", ""))
	assert.Zero(t, provider.calls)
}

func TestCaptchaWithoutSettingsIsDisabled(t *testing.T) {
	t.Parallel()
	svc := newCaptchaForTest(nil, &captchaProviderStub{})

	enabled, err := svc.IsEnabled(context.Background())
	assert.NoError(t, err)
	assert.False(t, enabled)
	assert.NoError(t, svc.Verify(context.Background(), "", ""))
	siteKey, err := svc.SiteKey(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, siteKey)
}

func TestCaptchaRequiresASecret(t *testing.T) {
	t.Parallel()
	svc := newCaptchaForTest(captchaSettingsStub{required: true}, &captchaProviderStub{})

	err := svc.Verify(context.Background(), "any-token", "")
	require.Error(t, err)
	assert.Equal(t, "captcha secret key not configured", err.Error())
}

func TestCaptchaRequiresAToken(t *testing.T) {
	t.Parallel()
	provider := &captchaProviderStub{}
	svc := newCaptchaForTest(captchaSettingsStub{required: true, secret: "secret"}, provider)

	err := svc.Verify(context.Background(), "  ", "")
	require.Error(t, err)
	assert.Equal(t, "captcha token is required", err.Error())
	assert.Zero(t, provider.calls)
}

func TestCaptchaPassesTheTrimmedTokenToTheProvider(t *testing.T) {
	t.Parallel()
	provider := &captchaProviderStub{success: true}
	svc := newCaptchaForTest(captchaSettingsStub{required: true, secret: "tenant-secret"}, provider)

	require.NoError(t, svc.Verify(context.Background(), " tok-abc ", "203.0.113.5"))
	secret, token, remoteIP := provider.calledWith()
	assert.Equal(t, "tenant-secret", secret)
	assert.Equal(t, "tok-abc", token)
	assert.Equal(t, "203.0.113.5", remoteIP)
}

func TestCaptchaProviderRefusalFailsVerification(t *testing.T) {
	t.Parallel()
	provider := &captchaProviderStub{codes: []string{"invalid-input-response"}}
	svc := newCaptchaForTest(captchaSettingsStub{required: true, secret: "s"}, provider)

	err := svc.Verify(context.Background(), "tok-bad", "203.0.113.5")
	require.Error(t, err)
	assert.Equal(t, "captcha verification failed", err.Error())
}

func TestCaptchaProviderErrorsBubbleUp(t *testing.T) {
	t.Parallel()
	failure := errors.New("captcha verify: http request: connection refused")
	svc := newCaptchaForTest(captchaSettingsStub{required: true, secret: "s"}, &captchaProviderStub{err: failure})

	assert.ErrorIs(t, svc.Verify(context.Background(), "tok", ""), failure)
}

func TestCaptchaSiteKeyComesFromTheSettings(t *testing.T) {
	t.Parallel()
	svc := newCaptchaForTest(captchaSettingsStub{siteKey: "site"}, &captchaProviderStub{})

	siteKey, err := svc.SiteKey(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "site", siteKey)
}

func TestCaptchaSettingsErrorsFailClosed(t *testing.T) {
	t.Parallel()
	failure := errors.New("settings unavailable")
	svc := newCaptchaForTest(captchaSettingsStub{err: failure}, &captchaProviderStub{})
	_, err := svc.IsEnabled(context.Background())
	assert.ErrorIs(t, err, failure)
	assert.ErrorIs(t, svc.Verify(context.Background(), "", ""), failure)
	_, err = svc.SiteKey(context.Background())
	assert.ErrorIs(t, err, failure)
}
