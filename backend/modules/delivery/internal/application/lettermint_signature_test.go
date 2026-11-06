package application

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
	lettermintTestSecret = "whsec_test_current"
	lettermintOldSecret  = "whsec_test_previous"
)

var lettermintSignedAt = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func lettermintSign(secret string, at time.Time, body string) (string, string) {
	stamp := fmt.Sprint(at.Unix())
	return "t=" + stamp + ",v1=" + hex.EncodeToString(lettermintMAC(secret, stamp, []byte(body))), stamp
}

func lettermintHeaders(signature, delivery string) func(string) string {
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

func TestLettermintVerifyAcceptsTheDocumentedScheme(t *testing.T) {
	t.Parallel()
	body := `{"id":"abc","event":"message.delivered"}`
	signature, delivery := lettermintSign(lettermintTestSecret, lettermintSignedAt, body)

	require.NoError(t, newLettermintWebhook(lettermintTestSecret).Verify(lettermintHeaders(signature, delivery), []byte(body), lettermintSignedAt))
}

// A vector computed outside this code from the documented scheme:
// HMAC-SHA256 hex of "{t}.{body}", with the secret as is, prefix included.
func TestLettermintVerifyMatchesTheDocumentedScheme(t *testing.T) {
	t.Parallel()
	signature := "t=1704067200,v1=8be9b61ebbb939baae27c799f094bdffe529304f0b8cc1b7121f46e72d1a5781"

	require.NoError(t, newLettermintWebhook("whsec_abc").Verify(lettermintHeaders(signature, "1704067200"), []byte(`{"id":"abc"}`), time.Unix(1704067200, 0)))
}

func TestLettermintVerifyAcceptsEverySecretOfTheRotationList(t *testing.T) {
	t.Parallel()
	body := `{}`
	webhook := newLettermintWebhook(lettermintTestSecret + " , " + lettermintOldSecret + ",")
	for _, secret := range []string{lettermintTestSecret, lettermintOldSecret} {
		signature, delivery := lettermintSign(secret, lettermintSignedAt, body)
		require.NoError(t, webhook.Verify(lettermintHeaders(signature, delivery), []byte(body), lettermintSignedAt))
	}
}

func TestLettermintVerifyRejects(t *testing.T) {
	t.Parallel()
	body := `{"id":"abc"}`
	good, delivery := lettermintSign(lettermintTestSecret, lettermintSignedAt, body)
	wrongSecret, _ := lettermintSign("whsec_other", lettermintSignedAt, body)
	tampered, _ := lettermintSign(lettermintTestSecret, lettermintSignedAt, `{"id":"xyz"}`)
	stale, staleDelivery := lettermintSign(lettermintTestSecret, lettermintSignedAt.Add(-5*time.Minute-time.Second), body)
	future, futureDelivery := lettermintSign(lettermintTestSecret, lettermintSignedAt.Add(5*time.Minute+time.Second), body)
	for name, headers := range map[string]func(string) string{
		"no headers":        lettermintHeaders("", ""),
		"no delivery":       lettermintHeaders(good, ""),
		"other delivery":    lettermintHeaders(good, "1"),
		"wrong secret":      lettermintHeaders(wrongSecret, delivery),
		"tampered body":     lettermintHeaders(tampered, delivery),
		"too old":           lettermintHeaders(stale, staleDelivery),
		"too far ahead":     lettermintHeaders(future, futureDelivery),
		"no timestamp":      lettermintHeaders("v1=abc", delivery),
		"no mac":            lettermintHeaders("t="+delivery, delivery),
		"mac is no hex":     lettermintHeaders("t="+delivery+",v1=zz", delivery),
		"short mac":         lettermintHeaders("t="+delivery+",v1=abcd", delivery),
		"timestamp no text": lettermintHeaders("t=now,v1=abcd", "now"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Error(t, newLettermintWebhook(lettermintTestSecret).Verify(headers, []byte(body), lettermintSignedAt))
		})
	}
}

func TestLettermintVerifyAcceptsTheEdgeOfTheWindow(t *testing.T) {
	t.Parallel()
	body := `{}`
	signature, delivery := lettermintSign(lettermintTestSecret, lettermintSignedAt.Add(-5*time.Minute), body)

	require.NoError(t, newLettermintWebhook(lettermintTestSecret).Verify(lettermintHeaders(signature, delivery), []byte(body), lettermintSignedAt))
}

func TestLettermintWebhookWithoutSecretIsNotConfigured(t *testing.T) {
	t.Parallel()
	assert.False(t, newLettermintWebhook("  , ").Configured())
	assert.True(t, newLettermintWebhook(lettermintTestSecret).Configured())
	assert.Error(t, newLettermintWebhook("").Verify(lettermintHeaders("t=1,v1=a", "1"), []byte(`{}`), lettermintSignedAt), "no secret never passes")
}
