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
}

func (s captchaSettingsStub) CaptchaRequired(context.Context) bool    { return s.required }
func (s captchaSettingsStub) CaptchaSecretKey(context.Context) string { return s.secret }
func (s captchaSettingsStub) CaptchaSiteKey(context.Context) string   { return s.siteKey }
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

	assert.False(t, svc.IsEnabled(context.Background()))
	// Even a blank token + IP must not error when captcha is off.
	assert.NoError(t, svc.Verify(context.Background(), "", ""))
	assert.Zero(t, provider.calls)
}

func TestCaptchaWithoutSettingsIsDisabled(t *testing.T) {
	t.Parallel()
	svc := newCaptchaForTest(nil, &captchaProviderStub{})

	assert.False(t, svc.IsEnabled(context.Background()))
	assert.NoError(t, svc.Verify(context.Background(), "", ""))
	assert.Empty(t, svc.SiteKey(context.Background()))
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

	assert.Equal(t, "site", svc.SiteKey(context.Background()))
}
