package middleware

import (
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

			fields := logrus.Fields{
				"timestamp":   event.Timestamp.Format(time.RFC3339Nano),
				"method":      event.Method,
				"path":        event.Path,
				"status_code": event.StatusCode,
				"duration_ms": event.DurationMS,
			}
			if event.RequestID != "" {
				fields["request_id"] = event.RequestID
			}
			if event.Service != "" {
				fields["service"] = event.Service
			}
			if event.Version != "" {
				fields["version"] = event.Version
			}
			if event.Environment != "" {
				fields["environment"] = event.Environment
			}
			if event.UserID != "" {
				fields["user_id"] = event.UserID
			}
			if event.UserRole != "" {
				fields["user_role"] = event.UserRole
			}
			if event.AccountID != "" {
				fields["account_id"] = event.AccountID
			}
			if event.StudentID != "" {
				fields["student_id"] = event.StudentID
			}
			if event.GroupID != "" {
				fields["group_id"] = event.GroupID
			}
			if event.ActivityID != "" {
				fields["activity_id"] = event.ActivityID
			}
			if event.RoomID != "" {
				fields["room_id"] = event.RoomID
			}
			if event.Action != "" {
				fields["action"] = event.Action
			}
			if event.ErrorType != "" {
				fields["error_type"] = event.ErrorType
				if event.ErrorCode != "" {
					fields["error_code"] = event.ErrorCode
				}
				if event.ErrorMessage != "" {
					fields["error_message"] = event.ErrorMessage
				}
			}

			entry := logger.Logger.WithFields(fields)
			switch {
			case event.StatusCode >= http.StatusInternalServerError:
				entry.Error("request_completed")
			case event.StatusCode >= http.StatusBadRequest:
				entry.Warn("request_completed")
			default:
				entry.Info("request_completed")
			}
		}()

		next.ServeHTTP(wrapped, r.WithContext(ctx))
	})
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
