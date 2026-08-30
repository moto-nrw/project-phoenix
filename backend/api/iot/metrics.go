package iot

import (
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func iotMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &iotStatusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		deviceType := "unknown"
		if deviceCtx := device.DeviceFromCtx(r.Context()); deviceCtx != nil {
			deviceType = deviceCtx.DeviceType
		}
		observability.ObserveIoTRequest(
			tenant.FromContext(r.Context()),
			r.Method,
			common.RoutePattern(r),
			recorder.status,
			time.Since(start),
			deviceType,
		)
	})
}

type iotStatusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *iotStatusWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *iotStatusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.status = http.StatusOK
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(body)
}

func (w *iotStatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *iotStatusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
