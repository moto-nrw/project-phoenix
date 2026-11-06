package webhooks

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
)

type stubIngest struct {
	err      error
	provider string
	body     string
	header   string
}

func (s *stubIngest) IngestEmailWebhook(_ context.Context, provider string, header func(string) string, body []byte) error {
	s.provider, s.body, s.header = provider, string(body), header("X-Lettermint-Signature")
	return s.err
}

func post(ingest Ingest, body string) (*httptest.ResponseRecorder, *error) {
	var failure error
	failed := func(w http.ResponseWriter, _ *http.Request, err error) {
		failure = err
		w.WriteHeader(http.StatusTeapot)
	}
	request := httptest.NewRequest(http.MethodPost, "/email/delivery/lettermint", strings.NewReader(body))
	request.Header.Set("X-Lettermint-Signature", "t=1,v1=a")
	recorder := httptest.NewRecorder()
	NewResource(ingest, failed, nil).Router().ServeHTTP(recorder, request)
	return recorder, &failure
}

func TestWebhookPassesTheRawDeliveryOn(t *testing.T) {
	t.Parallel()
	ingest := &stubIngest{}

	response, _ := post(ingest, `{ "raw":  "body" }`)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "lettermint", ingest.provider)
	assert.Equal(t, `{ "raw":  "body" }`, ingest.body, "the signature covers these exact bytes")
	assert.Equal(t, "t=1,v1=a", ingest.header)
}

func TestWebhookHandsAFailureToTheRoot(t *testing.T) {
	t.Parallel()
	failure := errors.New("refused")

	response, got := post(&stubIngest{err: failure}, `{}`)

	assert.Equal(t, http.StatusTeapot, response.Code)
	assert.ErrorIs(t, *got, failure)
}

func TestWebhookRefusesAnOversizedBodyBeforeVerifying(t *testing.T) {
	t.Parallel()
	ingest := &stubIngest{}

	_, got := post(ingest, strings.Repeat("x", maxWebhookBody+1))

	assert.ErrorIs(t, *got, ErrBodyTooLarge)
	assert.Empty(t, ingest.provider)
}

// A body that breaks off is no body too large: the provider must try again.
func TestWebhookTellsAnInterruptedBodyFromAnOversizedOne(t *testing.T) {
	t.Parallel()
	ingest := &stubIngest{}
	var failure error
	failed := func(w http.ResponseWriter, _ *http.Request, err error) {
		failure = err
		w.WriteHeader(http.StatusTeapot)
	}
	request := httptest.NewRequest(http.MethodPost, "/email/delivery/lettermint", iotest.ErrReader(io.ErrUnexpectedEOF))

	NewResource(ingest, failed, nil).Router().ServeHTTP(httptest.NewRecorder(), request)

	assert.ErrorIs(t, failure, ErrBodyUnreadable)
	assert.NotErrorIs(t, failure, ErrBodyTooLarge)
	assert.Empty(t, ingest.provider, "half a body is never verified")
}
