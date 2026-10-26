package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// iotTimingKey carries the request's *iotRequestTiming from the metric down
// to the markers behind device authentication and in the transaction.
type iotTimingKey struct{}

// iotRequestTiming follows one kiosk request from before device
// authentication to its response header (#5004). The unit-of-work observer
// may be reached from any goroutine that inherits the request context, so
// every field is read and written under mu.
type iotRequestTiming struct {
	mu            sync.Mutex
	start         time.Time
	authenticated time.Time
	transaction   bool
	handled       time.Time
	txEnded       time.Time
	committed     bool
	tenantID      int64
	deviceType    string
}

// serverTiming renders the sections that ran: "auth" until the device is
// authenticated (or rejected), "tx" until the request transaction committed
// or rolled back, and "after-commit" for the hooks after a commit. It holds
// durations only, never a device, person or credential.
func (t *iotRequestTiming) serverTiming(now time.Time) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	authEnd := t.authenticated
	if authEnd.IsZero() {
		authEnd = now
	}
	sections := []string{serverTimingSection("auth", authEnd.Sub(t.start))}
	if !t.transaction {
		return sections[0]
	}
	txEnd := t.txEnded
	if txEnd.IsZero() {
		txEnd = now
	}
	sections = append(sections, serverTimingSection("tx", txEnd.Sub(authEnd)))
	if t.committed {
		sections = append(sections, serverTimingSection("after-commit", now.Sub(t.txEnded)))
	}
	return strings.Join(sections, ", ")
}

func serverTimingSection(name string, duration time.Duration) string {
	return name + ";dur=" + strconv.FormatFloat(float64(duration.Microseconds())/1000, 'f', 3, 64)
}

// observeTransaction ends the "tx" section at the request transaction's
// outcome: the first transaction event after the handler returned. Units the
// handler ran on its own end before that, and after-commit hooks run after it.
func (t *iotRequestTiming) observeTransaction(event tenant.UnitOfWorkEvent) {
	if event.Kind != tenant.UnitOfWorkTransaction {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.handled.IsZero() || !t.txEnded.IsZero() {
		return
	}
	t.txEnded = time.Now()
	t.committed = event.Result == tenant.UnitOfWorkCommitted
}

func (t *iotRequestTiming) markAuthenticated(ctx context.Context) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.authenticated = time.Now()
	t.tenantID = tenant.FromContext(ctx)
	if _, _, _, deviceType, ok := deviceauth.ContextDevice(ctx); ok {
		t.deviceType = deviceType
	}
}

func (t *iotRequestTiming) labels() (int64, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.tenantID, t.deviceType
}

func iotTimingFrom(ctx context.Context) (*iotRequestTiming, bool) {
	timing, ok := ctx.Value(iotTimingKey{}).(*iotRequestTiming)
	return timing, ok && timing != nil
}

// iotMetricsMiddleware wraps device authentication, so the IoT request metric
// and the Server-Timing header include it (#5004). Tenant and device type
// come from iotAuthTiming, because the authenticated context never reaches
// back out here.
func iotMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timing := &iotRequestTiming{start: time.Now(), deviceType: "unknown"}
		recorder := &iotStatusWriter{ResponseWriter: w, status: http.StatusOK, timing: timing}

		next.ServeHTTP(recorder, r.WithContext(context.WithValue(r.Context(), iotTimingKey{}, timing)))

		tenantID, deviceType := timing.labels()
		observability.ObserveIoTRequest(
			tenantID,
			r.Method,
			common.RoutePattern(r),
			recorder.status,
			time.Since(timing.start),
			deviceType,
		)
	})
}

// iotAuthTiming runs right behind device authentication: it ends the "auth"
// section, keeps the device's tenant and type for the request metric, and
// adds the transaction observer next to the root's metrics observer.
func iotAuthTiming(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if timing, ok := iotTimingFrom(r.Context()); ok {
			timing.markAuthenticated(r.Context())
			r = r.WithContext(tenant.WithAdditionalUnitOfWorkObserver(r.Context(), timing.observeTransaction))
		}
		next.ServeHTTP(w, r)
	})
}

// iotTxTiming runs inside the request transaction and marks when its handler
// returned, so the next transaction event is the request's own outcome.
func iotTxTiming(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timing, ok := iotTimingFrom(r.Context())
		if ok {
			timing.mu.Lock()
			timing.transaction = true
			timing.mu.Unlock()
		}
		next.ServeHTTP(w, r)
		if ok {
			timing.mu.Lock()
			timing.handled = time.Now()
			timing.mu.Unlock()
		}
	})
}

type iotStatusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	timing      *iotRequestTiming
}

func (w *iotStatusWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
		w.Header().Set("Server-Timing", w.timing.serverTiming(time.Now()))
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *iotStatusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
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
