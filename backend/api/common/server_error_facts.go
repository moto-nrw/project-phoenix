package common

import "context"

type serverErrorKey struct{}

// ServerErrorCapture holds the error a handler gave for the 5xx answer it
// wrote, so the Sentry event carries the cause instead of a bare status.
type ServerErrorCapture struct {
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
	if cause, ok := ctx.Value(serverErrorKey{}).(*ServerErrorCapture); ok && err != nil {
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
	if cause, ok := ctx.Value(serverErrorKey{}).(*ServerErrorCapture); ok {
		cause.unreported = true
	}
}

// NewServerErrorCapture creates request-local facts without installing an SDK.
func NewServerErrorCapture() *ServerErrorCapture { return &ServerErrorCapture{} }

// WithServerErrorCapture binds the HTTP response facts the root observes.
func WithServerErrorCapture(ctx context.Context, capture *ServerErrorCapture) context.Context {
	return context.WithValue(ctx, serverErrorKey{}, capture)
}

func (c *ServerErrorCapture) Err() error       { return c.err }
func (c *ServerErrorCapture) Code() string     { return c.code }
func (c *ServerErrorCapture) Unreported() bool { return c.unreported }
