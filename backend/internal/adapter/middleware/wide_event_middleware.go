package middleware

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// WideEventMiddleware emits a single structured log line per request.
func WideEventMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := chimiddleware.GetReqID(r.Context())
		if requestID == "" {
			requestID = strings.TrimSpace(r.Header.Get(chimiddleware.RequestIDHeader))
		}
		if requestID != "" && w.Header().Get(chimiddleware.RequestIDHeader) == "" {
			w.Header().Set(chimiddleware.RequestIDHeader, requestID)
		}

		event := &WideEvent{
			Timestamp:   start,
			RequestID:   requestID,
			Method:      r.Method,
			Path:        r.URL.Path,
			Service:     strings.TrimSpace(viper.GetString("service_name")),
			Version:     strings.TrimSpace(viper.GetString("service_version")),
			Environment: strings.TrimSpace(viper.GetString("app_env")),
		}

		ctx := withWideEvent(r.Context(), event)
		wrapped := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		defer func() {
			event.StatusCode = wrapped.statusCode
			event.DurationMS = time.Since(start).Milliseconds()
			fields := buildWideEventFields(event)
			emitWideEventLog(fields, event.StatusCode)
		}()

		next.ServeHTTP(wrapped, r.WithContext(ctx))
	})
}

// buildWideEventFields creates logrus.Fields from WideEvent, only including non-empty values.
// This reduces cyclomatic complexity by centralizing field mapping logic.
func buildWideEventFields(event *WideEvent) logrus.Fields {
	fields := logrus.Fields{
		"timestamp":   event.Timestamp.Format(time.RFC3339Nano),
		"method":      event.Method,
		"path":        event.Path,
		"status_code": event.StatusCode,
		"duration_ms": event.DurationMS,
	}

	// Helper to add optional string fields
	addOptional := func(key string, value string) {
		if value != "" {
			fields[key] = value
		}
	}

	addOptional("request_id", event.RequestID)
	addOptional("service", event.Service)
	addOptional("version", event.Version)
	addOptional("environment", event.Environment)
	addOptional("user_id", event.UserID)
	addOptional("user_role", event.UserRole)
	addOptional("account_id", event.AccountID)
	addOptional("student_id", event.StudentID)
	addOptional("group_id", event.GroupID)
	addOptional("activity_id", event.ActivityID)
	addOptional("room_id", event.RoomID)
	addOptional("action", event.Action)
	addOptional("resource_id", event.ResourceID)

	// Add error fields if present
	if event.ErrorType != "" {
		fields["error_type"] = event.ErrorType
		addOptional("error_code", event.ErrorCode)
		addOptional("error_message", event.ErrorMessage)
	}

	// Add warning fields if present
	if event.WarningType != "" {
		fields["warning_type"] = event.WarningType
		addOptional("warning_code", event.WarningCode)
		addOptional("warning_message", event.WarningMessage)
	}

	return fields
}

// emitWideEventLog logs the wide event with appropriate level based on status code.
func emitWideEventLog(fields logrus.Fields, statusCode int) {
	entry := logger.Logger.WithFields(fields)
	switch {
	case statusCode >= http.StatusInternalServerError:
		entry.Error("request_completed")
	case statusCode >= http.StatusBadRequest:
		entry.Warn("request_completed")
	default:
		entry.Info("request_completed")
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusRecorder) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *statusRecorder) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *statusRecorder) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (rw *statusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := rw.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (rw *statusRecorder) ReadFrom(reader io.Reader) (int64, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	if rf, ok := rw.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(reader)
	}
	return io.Copy(rw.ResponseWriter, reader)
}
