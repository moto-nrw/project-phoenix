package common_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingTransport keeps every event the Sentry client would send. The
// client sends synchronously, so the buffer only has to hold one request's
// events.
type recordingTransport chan *sentry.Event

func (t recordingTransport) Configure(sentry.ClientOptions)        {}
func (t recordingTransport) Flush(time.Duration) bool              { return true }
func (t recordingTransport) FlushWithContext(context.Context) bool { return true }
func (t recordingTransport) Close()                                {}
func (t recordingTransport) SendEvent(event *sentry.Event)         { t <- event }

func (t recordingTransport) recorded() []*sentry.Event {
	var events []*sentry.Event
	for {
		select {
		case event := <-t:
			events = append(events, event)
		default:
			return events
		}
	}
}

// serveWithRecordingSentry sends req through a router that mounts the Sentry
// chain as api/base.go does (Recoverer, then ServerErrorReporting) with a
// client that records instead of sending. It returns the response status and
// the recorded events. The hub is bound per request, as sentryhttp clones it
// from the global hub in production.
func serveWithRecordingSentry(t *testing.T, register func(chi.Router), req *http.Request) (int, []*sentry.Event) {
	t.Helper()

	transport := make(recordingTransport, 8)
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: transport})
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	router.Use(common.ServerErrorReporting)
	register(router)

	hub := sentry.NewHub(client, sentry.NewScope())
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req.WithContext(sentry.SetHubOnContext(req.Context(), hub)))
	return rr.Code, transport.recorded()
}

// Issue #3639: every server error reaches Sentry exactly once, however the
// handler wrote the answer.
func TestServerErrorsBecomeExactlyOneSentryEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		handler http.HandlerFunc
		status  int
		// exception is the error the event must carry; empty means a message
		// event for an answer written without a recorded error.
		exception string
	}{
		{
			name: "RenderError",
			handler: func(w http.ResponseWriter, r *http.Request) {
				common.RenderError(w, r, common.ErrorInternalServer(errors.New("load student: connection reset")))
			},
			status:    http.StatusInternalServerError,
			exception: "load student: connection reset",
		},
		{
			name: "RespondWithError",
			handler: func(w http.ResponseWriter, r *http.Request) {
				common.RespondWithError(w, r, http.StatusServiceUnavailable, "settings unavailable")
			},
			status:    http.StatusServiceUnavailable,
			exception: "settings unavailable",
		},
		{
			name: "http.Error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "Error rendering response", http.StatusInternalServerError)
			},
			status: http.StatusInternalServerError,
		},
		{
			name: "render.Status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				render.Status(r, http.StatusInternalServerError)
				render.JSON(w, r, map[string]string{"status": "error"})
			},
			status: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			code, events := serveWithRecordingSentry(t, func(router chi.Router) {
				router.Get("/api/students/{id}", tc.handler)
			}, httptest.NewRequest(http.MethodGet, "/api/students/42", nil))

			require.Equal(t, tc.status, code)
			require.Len(t, events, 1, "a 5xx answer must produce exactly one Sentry event")
			event := events[0]
			assert.Equal(t, sentry.LevelError, event.Level)
			assert.Equal(t, "/api/students/{id}", event.Transaction)
			if tc.exception != "" {
				require.NotEmpty(t, event.Exception)
				assert.Equal(t, tc.exception, event.Exception[len(event.Exception)-1].Value)
			} else {
				assert.Empty(t, event.Exception)
				assert.Equal(t, "HTTP 500 GET /api/students/{id}", event.Message)
			}
		})
	}
}

// Client errors and requests the client gave up on are no server errors.
func TestClientErrorsAndCanceledRequestsProduceNoSentryEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		renderer render.Renderer
		status   int
	}{
		{"400", common.ErrorInvalidRequest(errors.New("bad id")), http.StatusBadRequest},
		{"401", common.ErrorUnauthorized(common.ErrUnauthorized), http.StatusUnauthorized},
		{"403", common.ErrorForbidden(errors.New("forbidden")), http.StatusForbidden},
		{"404", common.ErrorNotFound(errors.New("student not found")), http.StatusNotFound},
		{"429", common.ErrorTooManyRequests(errors.New("slow down")), http.StatusTooManyRequests},
		{"499", common.ErrorClientClosed(context.Canceled), 499},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			code, events := serveWithRecordingSentry(t, func(router chi.Router) {
				router.Get("/api/students/{id}", func(w http.ResponseWriter, r *http.Request) {
					common.RenderError(w, r, tc.renderer)
				})
			}, httptest.NewRequest(http.MethodGet, "/api/students/42", nil))

			require.Equal(t, tc.status, code)
			assert.Empty(t, events)
		})
	}

	t.Run("500 after the client canceled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/api/students/42", nil).WithContext(ctx)
		code, events := serveWithRecordingSentry(t, func(router chi.Router) {
			router.Get("/api/students/{id}", func(w http.ResponseWriter, _ *http.Request) {
				cancel()
				http.Error(w, "canceled", http.StatusInternalServerError)
			})
		}, req)

		require.Equal(t, http.StatusInternalServerError, code)
		assert.Empty(t, events)
	})
}

// Issue #3645: the error-report relay's 502 for an unreachable Sentry is the
// one server error that stays out of Sentry. The skip covers only its own
// request.
func TestSkippedServerErrorProducesNoSentryEvent(t *testing.T) {
	t.Parallel()

	code, events := serveWithRecordingSentry(t, func(router chi.Router) {
		router.Post("/api/iot/error-reports", func(w http.ResponseWriter, r *http.Request) {
			common.SkipServerErrorReport(r.Context())
			common.RenderError(w, r, common.ErrorBadGatewayWrap("error reporting service unavailable", errors.New("connection refused")))
		})
	}, httptest.NewRequest(http.MethodPost, "/api/iot/error-reports", nil))

	require.Equal(t, http.StatusBadGateway, code)
	assert.Empty(t, events)

	code, events = serveWithRecordingSentry(t, func(router chi.Router) {
		router.Post("/api/iot/error-reports", func(w http.ResponseWriter, r *http.Request) {
			common.RenderError(w, r, common.ErrorBadGatewayWrap("error reporting service unavailable", errors.New("connection refused")))
		})
	}, httptest.NewRequest(http.MethodPost, "/api/iot/error-reports", nil))

	require.Equal(t, http.StatusBadGateway, code)
	assert.Len(t, events, 1, "without the skip a 502 is an ordinary server error")
}

// sentryhttp reports the panic; the 500 that chi's Recoverer writes afterwards
// must not add a second event.
func TestPanicProducesExactlyOneSentryEvent(t *testing.T) {
	t.Parallel()

	code, events := serveWithRecordingSentry(t, func(router chi.Router) {
		router.Get("/api/students/{id}", func(http.ResponseWriter, *http.Request) {
			panic("nil student record")
		})
	}, httptest.NewRequest(http.MethodGet, "/api/students/42", nil))

	require.Equal(t, http.StatusInternalServerError, code)
	require.Len(t, events, 1)
	assert.Equal(t, "/api/students/{id}", events[0].Transaction)
}

// Events are named after the route pattern and carry no query string; paths
// with IDs stay because they locate the failure.
func TestServerErrorEventUsesRoutePatternWithoutQueryString(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/students/42?search=Mustermann&email=erika%40example.com", nil)
	_, events := serveWithRecordingSentry(t, func(router chi.Router) {
		router.Route("/api/students", func(r chi.Router) {
			r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
				sentry.GetHubFromContext(r.Context()).AddBreadcrumb(&sentry.Breadcrumb{
					Category: "http",
					Message:  "GET https://api.example.com/api/students/42?search=Mustermann",
					Data:     map[string]any{"url": "/api/students/42?email=erika%40example.com"},
				}, nil)
				common.RenderError(w, r, common.ErrorInternalServer(errors.New("boom")))
			})
		})
	}, req)

	require.Len(t, events, 1)
	event := events[0]
	assert.Equal(t, "/api/students/{id}", event.Transaction)
	require.NotNil(t, event.Request)
	assert.Empty(t, event.Request.QueryString)
	assert.Equal(t, "http://example.com/api/students/42", event.Request.URL)
	require.Len(t, event.Breadcrumbs, 1)
	assert.Equal(t, "GET https://api.example.com/api/students/42", event.Breadcrumbs[0].Message)
	assert.Equal(t, "/api/students/42", event.Breadcrumbs[0].Data["url"])
}
