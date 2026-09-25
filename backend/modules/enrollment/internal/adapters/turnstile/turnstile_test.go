package turnstile

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recordingServer(t *testing.T, body string, seen *url.Values) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if seen != nil {
			*seen, _ = url.ParseQuery(string(raw))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestSiteVerifyPostsSecretTokenAndRemoteIP(t *testing.T) {
	t.Parallel()
	var seen url.Values
	server := recordingServer(t, `{"success":true}`, &seen)

	success, codes, err := New(nil, server.URL).SiteVerify(context.Background(), "tenant-secret", "tok-abc", "203.0.113.5")
	require.NoError(t, err)
	assert.True(t, success)
	assert.Empty(t, codes)
	assert.Equal(t, "tenant-secret", seen.Get("secret"), "the secret must be POSTed verbatim")
	assert.Equal(t, "tok-abc", seen.Get("response"))
	assert.Equal(t, "203.0.113.5", seen.Get("remoteip"), "remoteip helps the provider score the request")
}

func TestSiteVerifyOmitsBlankRemoteIP(t *testing.T) {
	t.Parallel()
	var seen url.Values
	server := recordingServer(t, `{"success":true}`, &seen)

	_, _, err := New(nil, server.URL).SiteVerify(context.Background(), "s", "tok-abc", "")
	require.NoError(t, err)
	_, present := seen["remoteip"]
	assert.False(t, present, "blank remoteip MUST be omitted (sending an empty IP can cause provider 400)")
}

func TestSiteVerifyReturnsTheProviderVerdict(t *testing.T) {
	t.Parallel()
	server := recordingServer(t, `{"success":false,"error-codes":["invalid-input-response"]}`, nil)

	success, codes, err := New(nil, server.URL).SiteVerify(context.Background(), "s", "tok-bad", "203.0.113.5")
	require.NoError(t, err)
	assert.False(t, success)
	assert.Equal(t, []string{"invalid-input-response"}, codes)
}

func TestSiteVerifyRejectsBrokenJSON(t *testing.T) {
	t.Parallel()
	server := recordingServer(t, `not json`, nil)

	_, _, err := New(nil, server.URL).SiteVerify(context.Background(), "s", "tok", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "captcha verify: decode response")
}

func TestSiteVerifyReportsTransportErrors(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close() // close before any request to force a transport error

	_, _, err := New(nil, server.URL).SiteVerify(context.Background(), "s", "tok", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "captcha verify: http request")
}

func TestNewFallsBackToDefaults(t *testing.T) {
	t.Parallel()
	client := New(nil, "")
	require.NotNil(t, client.httpClient, "nil HTTPClient still produces a usable client")
	assert.Equal(t, VerifyURL, client.verifyURL, "an empty URL falls back to Turnstile")
}
