package requestfeedhttp_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed"
	requestfeedhttp "github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubEngine struct {
	status requestfeed.Status
	feed   requestfeed.Feed
	err    error
}

func (s stubEngine) Status(context.Context, int64, int64) (requestfeed.Status, error) {
	return s.status, s.err
}

func (s stubEngine) Provision(context.Context, int64, int64) (requestfeed.Created, error) {
	return requestfeed.Created{}, s.err
}

func (s stubEngine) Rotate(context.Context, int64, int64) (requestfeed.Created, error) {
	return requestfeed.Created{}, s.err
}

func (s stubEngine) ByToken(context.Context, string) (requestfeed.Feed, error) {
	return s.feed, s.err
}

func testResource(engine stubEngine) *requestfeedhttp.Resource {
	return requestfeedhttp.NewResource(requestfeed.NewModule(engine), requestfeedhttp.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, requestfeedhttp.Middleware)) {
			register(router, func(next http.Handler) http.Handler { return next })
		},
		CurrentTenantID:  func(*http.Request) int64 { return 7 },
		CurrentAccountID: func(*http.Request) int64 { return 9 },
		Logger:           slog.New(slog.DiscardHandler),
	})
}

func TestPublicFeedUsesUniformNotFoundResponse(t *testing.T) {
	t.Parallel()
	router := testResource(stubEngine{err: requestfeed.ErrNotFound}).PublicRouter()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/revoked-token", nil))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "Not Found\n", recorder.Body.String())
}

func TestPublicFeedDisablesCaching(t *testing.T) {
	t.Parallel()
	router := testResource(stubEngine{feed: requestfeed.Feed{XML: "<?xml version=\"1.0\"?><rss version=\"2.0\"></rss>"}}).PublicRouter()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/valid-token", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/rss+xml; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	assert.Contains(t, recorder.Body.String(), "<rss")
}

func TestTenantStatusDoesNotExposeTheStoredLink(t *testing.T) {
	t.Parallel()
	router := testResource(stubEngine{status: requestfeed.Status{Active: true}}).TenantRouter()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"active":true}`, recorder.Body.String())
	assert.NotContains(t, recorder.Body.String(), "url")
}
