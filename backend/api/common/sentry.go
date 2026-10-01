package common

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5/middleware"
)

type serverErrorKey struct{}

// serverErrorCause holds the error a handler gave for the 5xx answer it
// wrote, so the Sentry event carries the cause instead of a bare status.
type serverErrorCause struct {
	err error
	// code is the error code of the answer, if it has one. It only becomes a
	// tag: Sentry groups by stack trace, since server errors mostly share a
	// generic code (#3590).
	code string
	// unreported marks the one 5xx answer that must not reach Sentry.
	unreported bool
}

// noteServerError records err and its error code as the cause of the 5xx
// answer the current request is about to write. Outside
// ServerErrorReporting it does nothing.
func noteServerError(ctx context.Context, err error, code string) {
	if cause, ok := ctx.Value(serverErrorKey{}).(*serverErrorCause); ok && err != nil {
		cause.err = err
		cause.code = code
	}
}

// SkipServerErrorReport keeps the 5xx answer the current request is about to
// write out of Sentry. It is the single exception to the 5xx rule: the
// error-report relay answers 502 when Sentry itself is unreachable, and
// reporting that to Sentry could only loop (#3645). Outside
// ServerErrorReporting it does nothing.
func SkipServerErrorReport(ctx context.Context) {
	if cause, ok := ctx.Value(serverErrorKey{}).(*serverErrorCause); ok {
		cause.unreported = true
	}
}

var sentryHandler = sentryhttp.New(sentryhttp.Options{Repanic: true})

// ServerErrorReporting is the one place that reports HTTP server errors to
// Sentry (#3639). Every answer with status 500 or higher becomes exactly one
// event, whether it was written through RenderError, http.Error or
// render.Status. Answers below 500 and requests the client canceled produce
// none.
//
// A panic is reported by sentryhttp and repanics to the Recoverer mounted
// before this middleware. The panic unwinds past the status check, so the
// 500 the Recoverer writes adds no second event.
//
// Every event of the request, the panic event included, is named after the
// chi route pattern (/api/students/{id}), carries the request ID as
// request_id tag, so support finds it by the Vorgangskennung a school reads
// out, and carries no query string. SentrySessionContext adds the session.
func ServerErrorReporting(next http.Handler) http.Handler {
	return sentryHandler.Handle(reportServerErrors(next))
}

func reportServerErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// sentryhttp bound a hub cloned for this request.
		hub := sentry.GetHubFromContext(r.Context())
		hub.Scope().AddEventProcessor(requestEventProcessor(r))
		if requestID := middleware.GetReqID(r.Context()); requestID != "" {
			hub.Scope().SetTag("request_id", requestID)
		}

		cause := &serverErrorCause{}
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r.WithContext(context.WithValue(r.Context(), serverErrorKey{}, cause)))

		status := ww.Status()
		if status < http.StatusInternalServerError || cause.unreported || clientCanceled(r, cause.err) {
			return
		}
		hub.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("http.status_code", strconv.Itoa(status))
			if cause.code != "" {
				scope.SetTag("error_code", cause.code)
			}
			if cause.err != nil {
				hub.CaptureException(cause.err)
				return
			}
			event := sentry.NewEvent()
			event.Level = sentry.LevelError
			event.Message = fmt.Sprintf("HTTP %d %s %s", status, r.Method, RoutePattern(r))
			hub.CaptureEvent(event)
		})
	})
}

// clientCanceled reports whether the answer only failed because the client
// went away. That is no server error.
func clientCanceled(r *http.Request, cause error) bool {
	return errors.Is(r.Context().Err(), context.Canceled) || errors.Is(cause, context.Canceled)
}

// requestEventProcessor names events after the matched route and strips query
// strings: staff searches carry student names and e-mail addresses as query
// parameters (#2105). Paths with IDs stay because they locate the failure.
// The route is read when the event is built, so panic events get it too.
func requestEventProcessor(r *http.Request) sentry.EventProcessor {
	return func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
		if pattern := RoutePattern(r); pattern != unmatchedRoute {
			event.Transaction = pattern
			event.TransactionInfo = &sentry.TransactionInfo{Source: sentry.SourceRoute}
		}
		if event.Request != nil {
			event.Request.URL = stripQueryStrings(event.Request.URL)
			event.Request.QueryString = ""
		}
		for _, crumb := range event.Breadcrumbs {
			stripBreadcrumbQueryStrings(crumb)
		}
		return event
	}
}

func stripBreadcrumbQueryStrings(crumb *sentry.Breadcrumb) {
	if crumb == nil {
		return
	}
	crumb.Message = stripQueryStrings(crumb.Message)
	for key, value := range crumb.Data {
		if s, ok := value.(string); ok {
			crumb.Data[key] = stripQueryStrings(s)
		}
	}
}

// urlWithQuery matches an absolute URL or a path followed by a query string.
var urlWithQuery = regexp.MustCompile(`((?:[a-zA-Z][a-zA-Z0-9+.-]*://[^\s/?#]*)?/[^\s?#]*)\?[^\s#]*`)

// stripQueryStrings removes the query string from every URL or path in s and
// keeps the path itself.
func stripQueryStrings(s string) string {
	return urlWithQuery.ReplaceAllString(s, "$1")
}
