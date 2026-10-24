package iot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// sentryForwardTimeout bounds one forward to Sentry.
const sentryForwardTimeout = 10 * time.Second

// sentryAnswerHeaders are the answer headers the Sentry SDK acts on; the
// relay passes them back to the device.
var sentryAnswerHeaders = []string{"Content-Type", "Retry-After", "X-Sentry-Rate-Limits"}

// sentryForwarder is the kiosks' Sentry tunnel (#3645). It accepts only
// envelopes addressed to the project of its DSN, so it cannot be used as an
// open proxy, and always posts to that project's endpoint, never to a host
// the envelope names.
type sentryForwarder struct {
	target  SentryTarget
	runtime SentryTransportRuntime
	client  *http.Client
}

// NewErrorReportRelay builds the relay from the DSN of the pyreportal
// project; the DSN gives both the project allowlist and the target. Without
// a DSN the backend runs without Sentry, which serve allows only while
// SENTRY_DSN is empty too, and the route answers 503. A malformed DSN stops
// the start.
func NewErrorReportRelay(dsn string, runtime SentryTransportRuntime) (ErrorReportRelay, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, nil
	}
	target, err := runtime.ParseTarget(dsn)
	if err != nil {
		return nil, fmt.Errorf("SENTRY_PYREPORTAL_DSN: %w", err)
	}
	return &sentryForwarder{target: target, runtime: runtime, client: &http.Client{Timeout: sentryForwardTimeout}}, nil
}

func (f *sentryForwarder) Relay(ctx context.Context, envelope []byte, origin ErrorReportOrigin) (ErrorReportAnswer, error) {
	tagged, err := f.runtime.TagEnvelope(envelope, f.target.ProjectID, origin)
	if err != nil {
		return ErrorReportAnswer{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.target.EnvelopeURL, bytes.NewReader(tagged))
	if err != nil {
		return ErrorReportAnswer{}, fmt.Errorf("%w: %w", ErrErrorReportUndelivered, err)
	}
	req.Header.Set("Content-Type", "application/x-sentry-envelope")
	resp, err := f.client.Do(req)
	if err != nil {
		return ErrorReportAnswer{}, fmt.Errorf("%w: %w", ErrErrorReportUndelivered, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return ErrorReportAnswer{}, fmt.Errorf("%w: read answer: %w", ErrErrorReportUndelivered, err)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return ErrorReportAnswer{}, fmt.Errorf("%w: sentry answered %d", ErrErrorReportUndelivered, resp.StatusCode)
	}
	answer := ErrorReportAnswer{Status: resp.StatusCode, Header: http.Header{}, Body: body}
	for _, key := range sentryAnswerHeaders {
		if value := resp.Header.Get(key); value != "" {
			answer.Header.Set(key, value)
		}
	}
	return answer, nil
}
