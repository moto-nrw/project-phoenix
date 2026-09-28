package compose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	iotAPI "github.com/moto-nrw/project-phoenix/api/iot"
	"github.com/moto-nrw/project-phoenix/observability"
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
	target observability.SentryTarget
	client *http.Client
}

// NewErrorReportRelay builds the relay from the DSN of the pyreportal
// project; the DSN gives both the project allowlist and the target. Without
// a DSN the backend runs without Sentry, which serve allows only while
// SENTRY_DSN is empty too, and the route answers 503. A malformed DSN stops
// the start.
func NewErrorReportRelay(dsn string) (iotAPI.ErrorReportRelay, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, nil
	}
	target, err := observability.ParseSentryDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("SENTRY_PYREPORTAL_DSN: %w", err)
	}
	return &sentryForwarder{target: target, client: &http.Client{Timeout: sentryForwardTimeout}}, nil
}

func (f *sentryForwarder) Relay(ctx context.Context, envelope []byte, origin iotAPI.ErrorReportOrigin) (iotAPI.ErrorReportAnswer, error) {
	tagged, err := observability.TagSentryEnvelope(envelope, f.target.ProjectID, observability.SentryReporter{DeviceID: origin.DeviceID, SchoolID: origin.SchoolID})
	switch {
	case errors.Is(err, observability.ErrSentryProjectNotAllowed):
		return iotAPI.ErrorReportAnswer{}, fmt.Errorf("%w: %w", iotAPI.ErrErrorReportProjectNotAllowed, err)
	case err != nil:
		return iotAPI.ErrorReportAnswer{}, fmt.Errorf("%w: %w", iotAPI.ErrErrorReportInvalid, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.target.EnvelopeURL, bytes.NewReader(tagged))
	if err != nil {
		return iotAPI.ErrorReportAnswer{}, fmt.Errorf("%w: %w", iotAPI.ErrErrorReportUndelivered, err)
	}
	req.Header.Set("Content-Type", "application/x-sentry-envelope")
	resp, err := f.client.Do(req)
	if err != nil {
		return iotAPI.ErrorReportAnswer{}, fmt.Errorf("%w: %w", iotAPI.ErrErrorReportUndelivered, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return iotAPI.ErrorReportAnswer{}, fmt.Errorf("%w: read answer: %w", iotAPI.ErrErrorReportUndelivered, err)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return iotAPI.ErrorReportAnswer{}, fmt.Errorf("%w: sentry answered %d", iotAPI.ErrErrorReportUndelivered, resp.StatusCode)
	}
	answer := iotAPI.ErrorReportAnswer{Status: resp.StatusCode, Header: http.Header{}, Body: body}
	for _, key := range sentryAnswerHeaders {
		if value := resp.Header.Get(key); value != "" {
			answer.Header.Set(key, value)
		}
	}
	return answer, nil
}
