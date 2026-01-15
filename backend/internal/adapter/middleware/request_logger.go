package middleware

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

// RequestLogger returns a Chi middleware that logs HTTP requests via logrus.
func RequestLogger(logger *logrus.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return chimiddleware.RequestLogger(&logrusFormatter{logger: logger})
}

type logrusFormatter struct {
	logger *logrus.Logger
}

func (f *logrusFormatter) NewLogEntry(r *http.Request) chimiddleware.LogEntry {
	return &logrusEntry{
		logger:  f.logger,
		request: r,
		reqID:   chimiddleware.GetReqID(r.Context()),
	}
}

type logrusEntry struct {
	logger  *logrus.Logger
	request *http.Request
	reqID   string
}

func (e *logrusEntry) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ interface{}) {
	scheme := "http"
	if e.request.TLS != nil {
		scheme = "https"
	}

	fields := logrus.Fields{
		"status":       status,
		"bytes":        bytes,
		"duration_ms":  float64(elapsed.Microseconds()) / 1000.0,
		"method":       e.request.Method,
		"scheme":       scheme,
		"host":         e.request.Host,
		"request_uri":  e.request.RequestURI,
		"remote_addr":  e.request.RemoteAddr,
		"user_agent":   e.request.UserAgent(),
		"proto":        e.request.Proto,
		"content_type": e.request.Header.Get("Content-Type"),
	}
	if e.reqID != "" {
		fields["request_id"] = e.reqID
	}

	entry := e.logger.WithFields(fields)
	switch {
	case status >= 500:
		entry.Error("request completed")
	case status >= 400:
		entry.Warn("request completed")
	default:
		entry.Info("request completed")
	}
}

func (e *logrusEntry) Panic(v interface{}, stack []byte) {
	fields := logrus.Fields{
		"panic": v,
		"stack": string(stack),
	}
	if e.reqID != "" {
		fields["request_id"] = e.reqID
	}
	e.logger.WithFields(fields).Error("request panic")
}
