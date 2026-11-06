package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
