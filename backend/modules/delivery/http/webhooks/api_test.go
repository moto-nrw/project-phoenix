package webhooks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

type failingBody struct{ err error }

func (body failingBody) Read([]byte) (int, error) { return 0, body.err }
func (failingBody) Close() error                  { return nil }

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

func TestWebhookHandsAnUnreadableBodyToTheRoot(t *testing.T) {
	t.Parallel()
	readFailure := errors.New("connection reset")
	ingest := &stubIngest{}
	var got error
	request := httptest.NewRequest(http.MethodPost, "/email/delivery/lettermint", nil)
	request.Body = failingBody{err: readFailure}
	recorder := httptest.NewRecorder()

	NewResource(ingest, func(w http.ResponseWriter, _ *http.Request, err error) {
		got = err
		w.WriteHeader(http.StatusTeapot)
	}, nil).Router().ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusTeapot, recorder.Code)
	assert.ErrorIs(t, got, readFailure)
	assert.NotErrorIs(t, got, ErrBodyTooLarge)
	assert.Empty(t, ingest.provider)
}
