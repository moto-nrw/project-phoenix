package iot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"golang.org/x/time/rate"
)

// MaxErrorReportBytes is the largest envelope one request may carry.
const MaxErrorReportBytes = 1 << 20

// errorReportsPerMinute is the per-device quota of the relay. The burst is
// the same size, so a kiosk that comes back online can send its offline
// queue at once.
const errorReportsPerMinute = 60

// Error texts of POST /api/iot/error-reports beyond the device-key ones. They
// are part of the IoT contract with PyrePortal (docs/agents/contracts.md).
var (
	ErrErrorReportInvalid           = errors.New("invalid error report")
	ErrErrorReportProjectNotAllowed = errors.New("error report project is not allowed")
	ErrErrorReportTooLarge          = errors.New("error report too large")
	ErrErrorReportRateLimited       = errors.New("too many error reports")
	// ErrErrorReportUndelivered means Sentry did not take the envelope: no
	// connection, a timeout, or a 5xx answer.
	ErrErrorReportUndelivered   = errors.New("error reporting service unavailable")
	ErrErrorReportsUnconfigured = errors.New("error reporting is not configured")
)

// ErrorReportOrigin is the device an envelope came from.
type ErrorReportOrigin struct {
	DeviceID string
	SchoolID int64
}

// ErrorReportAnswer is Sentry's answer to a forwarded envelope, below 500.
type ErrorReportAnswer struct {
	Status int
	Header http.Header
	Body   []byte
}

// ErrorReportRelay forwards a kiosk's Sentry envelope. It wraps
// ErrErrorReportInvalid or ErrErrorReportProjectNotAllowed for an envelope it
// refuses and ErrErrorReportUndelivered when Sentry does not take it.
type ErrorReportRelay interface {
	Relay(ctx context.Context, envelope []byte, origin ErrorReportOrigin) (ErrorReportAnswer, error)
}

// ErrorReportsRuntime supplies the device principal and the error envelope.
type ErrorReportsRuntime struct {
	// Device returns the authenticated device: its primary key, its device
	// ID and its school.
	Device  func(context.Context) (id int64, deviceID string, schoolID int64, ok bool)
	Failure func(w http.ResponseWriter, r *http.Request, status int, err error)
	// SkipServerErrorReport keeps the relay's 502 out of Sentry.
	SkipServerErrorReport func(context.Context)
}

// ErrorReports is the Sentry tunnel of the kiosks (#3645): PyrePortal's SDK
// posts its envelopes here with the device key, and the relay forwards them
// to Sentry with the device and school of that key.
type ErrorReports struct {
	relay   ErrorReportRelay
	runtime ErrorReportsRuntime
	logger  *slog.Logger

	mu       sync.Mutex
	limiters map[int64]*rate.Limiter
}

// NewErrorReports builds the relay endpoint. A nil relay answers 503: the
// backend runs without Sentry, which only local development does.
func NewErrorReports(relay ErrorReportRelay, runtime ErrorReportsRuntime, logger *slog.Logger) *ErrorReports {
	if logger == nil {
		logger = slog.Default()
	}
	return &ErrorReports{relay: relay, runtime: runtime, logger: logger, limiters: map[int64]*rate.Limiter{}}
}

// Post handles POST /api/iot/error-reports.
func (rs *ErrorReports) Post(w http.ResponseWriter, r *http.Request) {
	id, deviceID, schoolID, ok := rs.runtime.Device(r.Context())
	if !ok {
		rs.runtime.Failure(w, r, http.StatusUnauthorized, errors.New(devicescan.MessageDeviceAPIKeyRequired))
		return
	}
	if rs.relay == nil {
		rs.runtime.Failure(w, r, http.StatusServiceUnavailable, ErrErrorReportsUnconfigured)
		return
	}
	if !rs.allow(id) {
		w.Header().Set("Retry-After", "60")
		rs.runtime.Failure(w, r, http.StatusTooManyRequests, ErrErrorReportRateLimited)
		return
	}
	envelope, status, err := readErrorReport(w, r)
	if err != nil {
		rs.runtime.Failure(w, r, status, err)
		return
	}

	answer, err := rs.relay.Relay(r.Context(), envelope, ErrorReportOrigin{DeviceID: deviceID, SchoolID: schoolID})
	switch {
	case errors.Is(err, ErrErrorReportInvalid):
		rs.runtime.Failure(w, r, http.StatusBadRequest, ErrErrorReportInvalid)
		return
	case errors.Is(err, ErrErrorReportProjectNotAllowed):
		rs.runtime.Failure(w, r, http.StatusBadRequest, ErrErrorReportProjectNotAllowed)
		return
	case err != nil:
		rs.logger.WarnContext(r.Context(), "error report relay failed",
			slog.String("device_id", deviceID),
			slog.Int64("school_id", schoolID),
			slog.String("error", err.Error()),
		)
		rs.runtime.SkipServerErrorReport(r.Context())
		rs.runtime.Failure(w, r, http.StatusBadGateway, err)
		return
	}
	maps.Copy(w.Header(), answer.Header)
	w.WriteHeader(answer.Status)
	_, _ = w.Write(answer.Body)
}

// readErrorReport reads the envelope up to MaxErrorReportBytes. A larger one
// answers 429 as the relay contract asks, so the SDK backs off.
func readErrorReport(w http.ResponseWriter, r *http.Request) ([]byte, int, error) {
	if r.ContentLength > MaxErrorReportBytes {
		return nil, http.StatusTooManyRequests, ErrErrorReportTooLarge
	}
	envelope, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxErrorReportBytes))
	if maxBytes := new(http.MaxBytesError); errors.As(err, &maxBytes) {
		return nil, http.StatusTooManyRequests, ErrErrorReportTooLarge
	}
	if err != nil {
		return nil, http.StatusBadRequest, ErrErrorReportInvalid
	}
	return envelope, 0, nil
}

// allow takes one envelope from the device's quota. The limiters live as
// long as the process; there is one per device that ever reported.
func (rs *ErrorReports) allow(id int64) bool {
	return rs.allowAt(id, time.Now())
}

// allowAt keeps the burst boundary testable independently of HTTP latency.
func (rs *ErrorReports) allowAt(id int64, now time.Time) bool {
	rs.mu.Lock()
	limiter, ok := rs.limiters[id]
	if !ok {
		limiter = rate.NewLimiter(rate.Limit(float64(errorReportsPerMinute)/60), errorReportsPerMinute)
		rs.limiters[id] = limiter
	}
	rs.mu.Unlock()
	return limiter.AllowN(now, 1)
}
