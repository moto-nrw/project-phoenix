package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// iotTimingTenants gives every case its own tenant label, so the cumulative
// metric series a case reads are its own.
var iotTimingTenants atomic.Int64

const iotTimingAuthDelay = 20 * time.Millisecond

var serverTimingShape = regexp.MustCompile(`^auth;dur=\d+\.\d{3}(, tx;dur=\d+\.\d{3}(, after-commit;dur=\d+\.\d{3})?)?$`)

func iotTimingUnitOfWork(t *testing.T) tenant.UnitOfWork {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	return runtime
}

// slowDeviceAuthentication stands in for the device middleware: it takes
// iotTimingAuthDelay, then binds the device, its tenant and the tenant
// runtime the production root binds router-wide.
func slowDeviceAuthentication(t *testing.T, tenantID int64) func(http.Handler) http.Handler {
	t.Helper()
	runtime := iotTimingUnitOfWork(t)
	device := &testpkg.Device{ID: tenantID, TenantID: tenantID, DeviceID: "timing-kiosk", DeviceType: "timing_reader", Status: "active"}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(iotTimingAuthDelay)
			authenticated := r.WithContext(tenant.WithUnitOfWork(r.Context(), runtime))
			testutil.WithDeviceContext(device)(authenticated)
			next.ServeHTTP(w, authenticated)
		})
	}
}

func serverTimingSections(t *testing.T, header string) map[string]time.Duration {
	t.Helper()
	require.Regexp(t, serverTimingShape, header)
	sections := make(map[string]time.Duration)
	for _, section := range strings.Split(header, ", ") {
		name, value, ok := strings.Cut(section, ";dur=")
		require.True(t, ok, section)
		milliseconds, err := strconv.ParseFloat(value, 64)
		require.NoError(t, err)
		sections[name] = time.Duration(milliseconds * float64(time.Millisecond))
	}
	return sections
}

// iotDurationSeries returns the duration histogram of one tenant label.
func iotDurationSeries(t *testing.T, tenantID int64) (count uint64, sum float64) {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != "phoenix_iot_request_duration_seconds" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["tenant_id"] == strconv.FormatInt(tenantID, 10) {
				assert.Equal(t, "timing_reader", labels["device_type"])
				return metric.GetHistogram().GetSampleCount(), metric.GetHistogram().GetSampleSum()
			}
		}
	}
	return 0, 0
}

// The kiosk request metric and the Server-Timing header start before device
// authentication (#5004): the composed IoT router mounts the metric in
// front of the authenticator of every device group.
func TestIoTRequestTiming_CoversDeviceAuthentication(t *testing.T) {
	t.Parallel()
	tenantID := 990_000_000 + iotTimingTenants.Add(1)
	resource := newIoTRouterTestResource()
	resource.DeviceAuthenticator = slowDeviceAuthentication(t, tenantID)
	router := chi.NewRouter()
	router.Mount("/api/iot", resource.Router())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/iot/ping", nil))

	sections := serverTimingSections(t, rec.Header().Get("Server-Timing"))
	assert.GreaterOrEqual(t, sections["auth"], iotTimingAuthDelay, "the auth section covers the device authentication")
	assert.Contains(t, sections, "tx")
	assert.Contains(t, sections, "after-commit", "the 4xx answer of the fake scan commits")
	count, sum := iotDurationSeries(t, tenantID)
	assert.Equal(t, uint64(1), count, "the request metric carries the authenticated tenant")
	assert.GreaterOrEqual(t, sum, iotTimingAuthDelay.Seconds(), "the request metric includes the device authentication")
}

// A request the device authentication rejects is measured too and shows
// only the auth section.
func TestIoTRequestTiming_RejectedDeviceShowsOnlyAuth(t *testing.T) {
	t.Parallel()
	router := chi.NewRouter()
	router.Mount("/api/iot", newIoTRouterTestResource().Router())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/iot/checkin", nil)
	req.Header.Set("Authorization", "Bearer kiosk-timing-key")
	req.Header.Set("X-Staff-PIN", "4711")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	header := rec.Header().Get("Server-Timing")
	assert.Contains(t, serverTimingSections(t, header), "auth")
	assert.NotContains(t, header, "tx;")
	assert.NotContains(t, header, "kiosk-timing-key", "Server-Timing carries no credential")
	assert.NotContains(t, header, "4711", "Server-Timing carries no credential")
}

// The sections split at the commit: the transaction ends with the commit,
// after-commit covers the hooks behind it, a unit the handler runs on its
// own does not end the section, and a rolled-back transaction has no
// after-commit section.
func TestIoTRequestTiming_SplitsTransactionAtCommit(t *testing.T) {
	t.Parallel()
	const commitDelay, hookDelay, workDelay = 15 * time.Millisecond, 25 * time.Millisecond, 30 * time.Millisecond
	tenantID := 990_000_000 + iotTimingTenants.Add(1)
	chain := func(t *testing.T, handler http.HandlerFunc) http.Handler {
		runtime, err := tenant.NewUnitOfWork(
			func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
				err := fn(ctx, struct{}{})
				time.Sleep(commitDelay)
				return err
			},
			func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
			func(context.Context, tenant.SavepointAction) error { return nil },
			func(error) bool { return false },
		)
		require.NoError(t, err)
		authenticate := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := tenant.WithUnitOfWork(tenant.WithTenantID(r.Context(), tenantID), runtime)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
		return iotMetricsMiddleware(authenticate(iotAuthTiming(common.TenantTxMiddleware(iotTxTiming(handler)))))
	}
	t.Run("commit", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		chain(t, func(w http.ResponseWriter, r *http.Request) {
			tenant.RegisterAfterCommit(r.Context(), func() { time.Sleep(hookDelay) })
			w.WriteHeader(http.StatusOK)
		}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/checkin", nil))

		sections := serverTimingSections(t, rec.Header().Get("Server-Timing"))
		assert.GreaterOrEqual(t, sections["tx"], commitDelay, "the transaction section ends after the commit")
		assert.GreaterOrEqual(t, sections["after-commit"], hookDelay, "the after-commit section covers the hooks")
	})
	t.Run("a unit of the handler's own", func(t *testing.T) {
		t.Parallel()
		id, err := tenant.NewTenantID(tenantID)
		require.NoError(t, err)
		rec := httptest.NewRecorder()
		chain(t, func(w http.ResponseWriter, r *http.Request) {
			own := tenant.ContextWithoutAfterCommitHooks(r.Context())
			require.NoError(t, tenant.WithinTenant(own, id, func(context.Context) error { return nil }))
			time.Sleep(workDelay)
			w.WriteHeader(http.StatusOK)
		}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/checkin", nil))

		sections := serverTimingSections(t, rec.Header().Get("Server-Timing"))
		assert.GreaterOrEqual(t, sections["tx"], workDelay+commitDelay, "the request transaction, not the handler's own unit, ends the section")
	})
	t.Run("rollback", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		chain(t, func(w http.ResponseWriter, r *http.Request) {
			tenant.MarkRollback(r.Context())
			w.WriteHeader(http.StatusConflict)
		}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/checkin", nil))

		header := rec.Header().Get("Server-Timing")
		assert.Contains(t, serverTimingSections(t, header), "tx")
		assert.NotContains(t, header, "after-commit")
	})
}
