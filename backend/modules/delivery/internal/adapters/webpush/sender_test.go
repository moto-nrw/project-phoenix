package webpush

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"

	provider "github.com/SherClockHolmes/webpush-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendPushRejectsUntrustedEndpoint(t *testing.T) {
	t.Parallel()
	sender := New(deliveryContract.WebPushConfig{Subscriber: "mailto:push@example.test", PublicKey: "public", PrivateKey: "private"}, nil)

	_, err := sender.SendPush(context.Background(), deliveryContract.ClaimedIntent{
		PushRecipient: deliveryContract.PushRecipient{Endpoint: "https://127.0.0.1/internal"},
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid push endpoint")
}

func TestPushOptionsFollowPersistedPriority(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		priority string
		ttl      int
		urgency  provider.Urgency
	}{
		{priority: "high", ttl: 3600, urgency: provider.UrgencyHigh},
		{priority: "normal", ttl: 86400, urgency: provider.UrgencyNormal},
		{priority: "low", ttl: 86400, urgency: provider.UrgencyLow},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.priority, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.ttl, pushTTL(testCase.priority))
			assert.Equal(t, testCase.urgency, pushUrgency(testCase.priority))
		})
	}
}

// Sentry BACKEND-Y carries the send error of a dead letter. The endpoint path
// is the device's push token and must not reach Sentry or logs.
func TestWithoutPushTokenKeepsOnlyThePushServiceOrigin(t *testing.T) {
	t.Parallel()
	refused := errors.New("dial tcp: connection refused")

	err := withoutPushToken(&url.Error{Op: "Post", URL: "https://fcm.googleapis.com/fcm/send/device-token-123", Err: refused})

	assert.EqualError(t, err, `Post "https://fcm.googleapis.com": dial tcp: connection refused`)
	assert.ErrorIs(t, err, refused)
	var urlErr *url.Error
	assert.ErrorAs(t, err, &urlErr, "transport errors keep their type")
}

func TestSendPushKeepsPushTokenOutOfInvalidEndpointErrors(t *testing.T) {
	t.Parallel()
	sender := New(deliveryContract.WebPushConfig{Subscriber: "mailto:push@example.test", PublicKey: "public", PrivateKey: "private"}, nil)

	_, err := sender.SendPush(context.Background(), deliveryContract.ClaimedIntent{
		PushRecipient: deliveryContract.PushRecipient{Endpoint: "https://fcm.googleapis.com/fcm/send/device-token-123\x7f"},
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid push endpoint")
	assert.NotContains(t, err.Error(), "device-token-123")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

// pushServiceAnswering returns a sender whose push service answers every
// request with status, and a counter of subscription cleanups.
func pushServiceAnswering(t *testing.T, status int, transportErr error) (*Sender, *int) {
	t.Helper()
	privateKey, publicKey, err := provider.GenerateVAPIDKeys()
	require.NoError(t, err)
	cleanups := 0
	sender := New(deliveryContract.WebPushConfig{Subscriber: "mailto:push@example.test", PublicKey: publicKey, PrivateKey: privateKey},
		func(context.Context, deliveryContract.ClaimedIntent) error {
			cleanups++
			return nil
		})
	sender.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		if transportErr != nil {
			return nil, transportErr
		}
		return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	return sender, &cleanups
}

func pushIntent(t *testing.T) deliveryContract.ClaimedIntent {
	t.Helper()
	// A VAPID public key is an uncompressed P-256 point, the same shape as a
	// browser subscription's p256dh key.
	_, devicePublicKey, err := provider.GenerateVAPIDKeys()
	require.NoError(t, err)
	return deliveryContract.ClaimedIntent{
		PushRecipient: deliveryContract.PushRecipient{
			Endpoint: "https://fcm.googleapis.com/fcm/send/device-token-123",
			P256DH:   devicePublicKey,
			Auth:     "MDEyMzQ1Njc4OWFiY2RlZg", // 16 bytes, base64url

		},
		PushPayload: deliveryContract.PushPayload{Title: "Abwesenheit", Priority: "normal"},
	}
}

// Sentry BACKEND-15: a 403 was retried until the attempt limit. RFC 8292 lets
// a push service answer 403 for an invalid JWT, which hits every subscription,
// so the rejection is final for this intent but keeps the subscription.
func TestSendPushClassifiesPushServiceAnswers(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusRequestEntityTooLarge} {
		t.Run(fmt.Sprintf("%d is final and keeps the subscription", status), func(t *testing.T) {
			t.Parallel()
			sender, cleanups := pushServiceAnswering(t, status, nil)

			result, err := sender.SendPush(context.Background(), pushIntent(t))

			require.ErrorIs(t, err, deliveryContract.ErrPermanent)
			assert.NotErrorIs(t, err, deliveryContract.ErrCancelled)
			assert.Equal(t, status, result.StatusCode)
			assert.Zero(t, *cleanups, "a permanent rejection must not prune the subscription")
			assert.NotContains(t, err.Error(), "device-token-123")
		})
	}
	for _, status := range []int{http.StatusNotFound, http.StatusGone} {
		t.Run(fmt.Sprintf("%d prunes the expired subscription", status), func(t *testing.T) {
			t.Parallel()
			sender, cleanups := pushServiceAnswering(t, status, nil)

			_, err := sender.SendPush(context.Background(), pushIntent(t))

			require.ErrorIs(t, err, deliveryContract.ErrCancelled)
			assert.Equal(t, 1, *cleanups)
		})
	}
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprintf("%d stays retryable", status), func(t *testing.T) {
			t.Parallel()
			sender, cleanups := pushServiceAnswering(t, status, nil)

			_, err := sender.SendPush(context.Background(), pushIntent(t))

			require.Error(t, err)
			assert.NotErrorIs(t, err, deliveryContract.ErrPermanent)
			assert.NotErrorIs(t, err, deliveryContract.ErrCancelled)
			assert.Zero(t, *cleanups)
		})
	}
	t.Run("a network failure stays retryable without the push token", func(t *testing.T) {
		t.Parallel()
		sender, cleanups := pushServiceAnswering(t, 0, errors.New("connection refused"))

		_, err := sender.SendPush(context.Background(), pushIntent(t))

		require.Error(t, err)
		assert.NotErrorIs(t, err, deliveryContract.ErrPermanent)
		assert.NotContains(t, err.Error(), "device-token-123")
		assert.Zero(t, *cleanups)
	})
	t.Run("2xx is acceptance", func(t *testing.T) {
		t.Parallel()
		sender, _ := pushServiceAnswering(t, http.StatusCreated, nil)

		result, err := sender.SendPush(context.Background(), pushIntent(t))

		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, result.StatusCode)
	})
}

// Sentry BACKEND-17: the configured contact is the RFC 8292 URI
// "mailto:…", and webpush-go adds the mailto scheme itself. Passed through
// unchanged, the JWT claimed "mailto:mailto:…", which Apple's push service
// rejects with 403 for every subscription.
func TestSendPushClaimsTheConfiguredContactOnce(t *testing.T) {
	t.Parallel()

	for _, subscriber := range []string{"mailto:push@example.test", "https://example.test/contact"} {
		t.Run(subscriber, func(t *testing.T) {
			t.Parallel()
			privateKey, publicKey, err := provider.GenerateVAPIDKeys()
			require.NoError(t, err)
			sender := New(deliveryContract.WebPushConfig{Subscriber: subscriber, PublicKey: publicKey, PrivateKey: privateKey}, nil)
			var claims map[string]any
			sender.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				claims = vapidClaims(t, request.Header.Get("Authorization"))
				return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader(""))}, nil
			})}

			_, err = sender.SendPush(context.Background(), pushIntent(t))

			require.NoError(t, err)
			assert.Equal(t, subscriber, claims["sub"])
		})
	}
}

// vapidClaims decodes the JWT claims of a "vapid t=<jwt>, k=<key>" header.
func vapidClaims(t *testing.T, header string) map[string]any {
	t.Helper()
	token, _, _ := strings.Cut(strings.TrimPrefix(header, "vapid t="), ",")
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "VAPID header carries a JWT")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}

// Sentry 2026-11-11: Mozilla delivers to Firefox on Android through FCM. It
// puts the base64url-encoded request body into an FCM data map and answers
// 413 when that map's JSON exceeds 4096 bytes (autopush-rs FcmSettings.max_data,
// routers/common.rs message_size_check). webpush-go padded every body to 4096
// bytes, so every such push failed, whatever its length.
func TestSendPushFitsMozillasFCMBridge(t *testing.T) {
	t.Parallel()
	privateKey, publicKey, err := provider.GenerateVAPIDKeys()
	require.NoError(t, err)
	sender := New(deliveryContract.WebPushConfig{Subscriber: "mailto:push@example.test", PublicKey: publicKey, PrivateKey: privateKey}, nil)
	var bodies [][]byte
	sender.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, readErr := io.ReadAll(request.Body)
		require.NoError(t, readErr)
		bodies = append(bodies, body)
		return &http.Response{StatusCode: http.StatusCreated, Status: "201 Created", Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	short := pushIntent(t)
	largest := pushIntent(t)
	encoded, err := json.Marshal(largest.PushPayload)
	require.NoError(t, err)
	largest.PushPayload.Title += strings.Repeat("x", deliveryContract.WebPushMaxPayloadBytes-len(encoded))

	for _, intent := range []deliveryContract.ClaimedIntent{short, largest} {
		_, err := sender.SendPush(context.Background(), intent)
		require.NoError(t, err)
	}

	require.Len(t, bodies, 2)
	assert.Len(t, bodies[0], len(bodies[1]), "padding hides the message length")
	bridged, err := json.Marshal(map[string]string{
		"chid": "00000000-0000-0000-0000-000000000000",
		"body": base64.RawURLEncoding.EncodeToString(bodies[1]),
		"con":  "aes128gcm",
		"rid":  strings.Repeat("r", 128),
	})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(bridged), 4096, "Mozilla's FCM bridge answers 413 above 4096 bytes")
}
