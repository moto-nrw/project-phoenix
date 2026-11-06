package lettermint

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The test secrets are random-looking only by name: they sign test bodies.
const (
	testSecret    = "whsec_test_current"
	testOldSecret = "whsec_test_previous"
)

var signedAt = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func sign(secret string, at time.Time, body string) (string, string) {
	stamp := fmt.Sprint(at.Unix())
	return "t=" + stamp + ",v1=" + hex.EncodeToString(signatureMAC(secret, stamp, []byte(body))), stamp
}

func headersOf(signature, delivery string) func(string) string {
	return func(name string) string {
		switch name {
		case "X-Lettermint-Signature":
			return signature
		case "X-Lettermint-Delivery":
			return delivery
		}
		return ""
	}
}

func TestVerifyAcceptsTheDocumentedScheme(t *testing.T) {
	t.Parallel()
	body := `{"id":"abc","event":"message.delivered"}`
	signature, delivery := sign(testSecret, signedAt, body)

	require.NoError(t, NewWebhook(testSecret).Verify(headersOf(signature, delivery), []byte(body), signedAt))
}

// A vector computed outside this code from the documented scheme:
// HMAC-SHA256 hex of "{t}.{body}", with the secret as is, prefix included.
func TestVerifyMatchesTheDocumentedScheme(t *testing.T) {
	t.Parallel()
	signature := "t=1704067200,v1=8be9b61ebbb939baae27c799f094bdffe529304f0b8cc1b7121f46e72d1a5781"

	require.NoError(t, NewWebhook("whsec_abc").Verify(headersOf(signature, "1704067200"), []byte(`{"id":"abc"}`), time.Unix(1704067200, 0)))
}

func TestVerifyAcceptsEverySecretOfTheRotationList(t *testing.T) {
	t.Parallel()
	body := `{}`
	webhook := NewWebhook(testSecret + " , " + testOldSecret + ",")
	for _, secret := range []string{testSecret, testOldSecret} {
		signature, delivery := sign(secret, signedAt, body)
		require.NoError(t, webhook.Verify(headersOf(signature, delivery), []byte(body), signedAt))
	}
}

func TestVerifyRejects(t *testing.T) {
	t.Parallel()
	body := `{"id":"abc"}`
	good, delivery := sign(testSecret, signedAt, body)
	wrongSecret, _ := sign("whsec_other", signedAt, body)
	tampered, _ := sign(testSecret, signedAt, `{"id":"xyz"}`)
	stale, staleDelivery := sign(testSecret, signedAt.Add(-5*time.Minute-time.Second), body)
	future, futureDelivery := sign(testSecret, signedAt.Add(5*time.Minute+time.Second), body)
	for name, headers := range map[string]func(string) string{
		"no headers":        headersOf("", ""),
		"no delivery":       headersOf(good, ""),
		"other delivery":    headersOf(good, "1"),
		"wrong secret":      headersOf(wrongSecret, delivery),
		"tampered body":     headersOf(tampered, delivery),
		"too old":           headersOf(stale, staleDelivery),
		"too far ahead":     headersOf(future, futureDelivery),
		"no timestamp":      headersOf("v1=abc", delivery),
		"no mac":            headersOf("t="+delivery, delivery),
		"mac is no hex":     headersOf("t="+delivery+",v1=zz", delivery),
		"short mac":         headersOf("t="+delivery+",v1=abcd", delivery),
		"timestamp no text": headersOf("t=now,v1=abcd", "now"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Error(t, NewWebhook(testSecret).Verify(headers, []byte(body), signedAt))
		})
	}
}

func TestVerifyAcceptsTheEdgeOfTheWindow(t *testing.T) {
	t.Parallel()
	body := `{}`
	signature, delivery := sign(testSecret, signedAt.Add(-5*time.Minute), body)

	require.NoError(t, NewWebhook(testSecret).Verify(headersOf(signature, delivery), []byte(body), signedAt))
}

func TestWebhookWithoutSecretIsNotConfigured(t *testing.T) {
	t.Parallel()
	assert.False(t, NewWebhook("  , ").Configured())
	assert.True(t, NewWebhook(testSecret).Configured())
	assert.Error(t, NewWebhook("").Verify(headersOf("t=1,v1=a", "1"), []byte(`{}`), signedAt), "no secret never passes")
}
