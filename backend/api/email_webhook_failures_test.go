package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	emailWebhooksHTTP "github.com/moto-nrw/project-phoenix/modules/delivery/http/webhooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailWebhookFailuresUseTheEnvelopeWithRegisteredCodes(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"unknown provider": {deliveryContract.ErrEmailWebhookUnknownProvider, http.StatusNotFound, "general.input"},
		"no secret":        {deliveryContract.ErrEmailWebhookNotConfigured, http.StatusServiceUnavailable, "general.unavailable"},
		"signature":        {deliveryContract.ErrEmailWebhookSignature, http.StatusUnauthorized, "general.permission"},
		"payload":          {fmt.Errorf("%w: secret detail", deliveryContract.ErrEmailWebhookPayload), http.StatusBadRequest, "general.input"},
		"too large":        {emailWebhooksHTTP.ErrBodyTooLarge, http.StatusRequestEntityTooLarge, "general.request_too_large"},
		"store":            {errors.New("database secret detail"), http.StatusInternalServerError, "general.server"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()

			renderEmailWebhookFailure(recorder, httptest.NewRequest(http.MethodPost, "/webhooks/email/delivery/lettermint", nil), test.err)

			assert.Equal(t, test.status, recorder.Code)
			var problem struct {
				Status int    `json:"status"`
				Code   string `json:"code"`
				Detail string `json:"detail"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &problem))
			assert.Equal(t, test.status, problem.Status)
			assert.Equal(t, test.code, problem.Code)
			assert.NotContains(t, problem.Detail, "secret detail", "the sender learns nothing about moto")
		})
	}
}

// A deployment without a secret answers every anonymous POST with 503. That is a
// state of the deployment, not a server error: it must not page anyone. The access
// log and Sentry are real here. The "server error" line of RenderError hangs on the
// cause the capture holds, so an empty capture proves that line away too.
func TestEmailWebhookWithoutSecretStaysOutOfSentryAndTheErrorLog(t *testing.T) {
	t.Parallel()
	serve := func(t *testing.T, failure error) (int, string, []*sentry.Event, *apiCommon.ServerErrorCapture) {
		t.Helper()
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))
		transport := make(recordingTransport, 8)
		client, err := sentry.NewClient(sentry.ClientOptions{Transport: transport})
		require.NoError(t, err)
		capture := apiCommon.NewServerErrorCapture()
		router := chi.NewRouter()
		setupBasicMiddleware(router, logger, nil)
		router.Post("/webhooks/email/delivery/{provider}", func(w http.ResponseWriter, r *http.Request) {
			// The inner capture stands next to the one ServerErrorReporting reads.
			renderEmailWebhookFailure(w, r, failure)
			inner := apiCommon.WithServerErrorCapture(r.Context(), capture)
			renderEmailWebhookFailure(httptest.NewRecorder(), r.WithContext(inner), failure)
		})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/webhooks/email/delivery/lettermint", nil)
		router.ServeHTTP(recorder, request.WithContext(sentry.SetHubOnContext(request.Context(), sentry.NewHub(client, sentry.NewScope()))))
		return recorder.Code, logs.String(), transport.recorded(), capture
	}

	status, logs, events, capture := serve(t, deliveryContract.ErrEmailWebhookNotConfigured)
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Empty(t, events)
	assert.NotContains(t, logs, "level=ERROR")
	assert.NoError(t, capture.Err())
	assert.True(t, capture.Unreported())

	status, logs, events, capture = serve(t, errors.New("database is down"))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Len(t, events, 1, "a failed store stays a server error")
	assert.Contains(t, logs, "level=ERROR")
	assert.Error(t, capture.Err())
	assert.False(t, capture.Unreported())
}

func TestEmailWebhookRateLimitWritesTheSecurityEvent(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	limit := newEmailWebhookRateLimit(customMiddleware.NewSecurityLoggerWith(slog.New(slog.NewTextHandler(&logs, nil))))
	handler := limit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	rejected := 0
	for range emailWebhookBurst + 1 {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/webhooks/email/delivery/lettermint", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		handler.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusTooManyRequests {
			rejected++
		}
	}

	require.Positive(t, rejected)
	assert.Equal(t, rejected, strings.Count(logs.String(), customMiddleware.EventRateLimitExceed))
}
