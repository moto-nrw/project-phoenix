package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func TestRequestIDMiddlewarePreservesChiContextContract(t *testing.T) {
	t.Parallel()

	const requestValue = "edge-proxy/request-42"
	var got string
	handler := requestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = middleware.GetReqID(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, requestValue)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got != requestValue {
		t.Fatalf("GetReqID() = %q, want %q", got, requestValue)
	}
}

func TestRequestIDMiddlewareSetsResponseHeaderBeforeHandler(t *testing.T) {
	t.Parallel()

	const requestValue = "edge-proxy/request-42"
	handler := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if got := w.Header().Get(middleware.RequestIDHeader); got != requestValue {
			t.Errorf("response %s header = %q, want %q", middleware.RequestIDHeader, got, requestValue)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, requestValue)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if got := res.Header().Get(middleware.RequestIDHeader); got != requestValue {
		t.Errorf("response %s header = %q, want %q", middleware.RequestIDHeader, got, requestValue)
	}
}
