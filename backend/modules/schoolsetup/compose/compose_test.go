package compose_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup/compose"
	schoolsetuphttp "github.com/moto-nrw/project-phoenix/modules/schoolsetup/http"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type harness struct {
	db       *bun.DB
	router   chi.Router
	settings configSvc.SettingsService
	// openAttendance answers the presence-mode guard.
	openAttendance bool
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	runtime := testpkg.SettingsRuntime(t, db)
	settings := configSvc.NewSettingsService(
		configRepo.NewSettingValueRepository(runtime),
		configRepo.NewSettingAuditRepository(runtime),
		nil, runtime, slog.Default(),
	)
	testpkg.SetTenantRuntime(t, settings, db)
	h := &harness{db: db, settings: settings}
	service, err := compose.New(compose.Dependencies{
		Settings:       settings,
		OpenAttendance: openAttendance{harness: h},
		SideEffect: func(context.Context, int64, string, any) (func(), error) {
			return nil, nil
		},
	})
	require.NoError(t, err)
	resource := schoolsetuphttp.NewResource(service, schoolsetuphttp.Runtime{
		Protected: func(r chi.Router, fn func(chi.Router, schoolsetuphttp.Middleware)) {
			r.Group(func(r chi.Router) { fn(r, testpkg.TenantTxMiddleware(db)) })
		},
		RequireWrite: func(next http.Handler) http.Handler { return next },
		Actor: func(ctx context.Context) (int64, int64) {
			account, _ := ctx.Value(accountKey{}).(int64)
			return tenant.FromContext(ctx), account
		},
		Respond: func(w http.ResponseWriter, _ *http.Request, status int, data any, _ string) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(data)
		},
		Failure: func(w http.ResponseWriter, _ *http.Request, status int, err error) {
			w.Header().Set("X-Conflict-Code", schoolsetup.ConflictCode(err))
			http.Error(w, err.Error(), status)
		},
	})
	h.router = resource.Router()
	return h
}

// openAttendance answers the presence-mode guard from the harness, as
// Student Presence does in production.
type openAttendance struct{ harness *harness }

func (o openAttendance) HasOpenAttendanceOn(context.Context, configModel.CalendarDate) (bool, error) {
	return o.harness.openAttendance, nil
}

// accountKey carries the acting account in these tests. Production takes it
// from the session principal.
type accountKey struct{}

// do sends one request as the account and decodes a 200 body into the status.
func (h *harness) do(t *testing.T, accountID int64, method, path string, body any) (*httptest.ResponseRecorder, schoolsetup.Status) {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	ctx := context.WithValue(testpkg.Ctx(t), accountKey{}, accountID)
	request := httptest.NewRequest(method, path, &payload).WithContext(ctx)
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	var status schoolsetup.Status
	if recorder.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &status))
	}
	return recorder, status
}

func (h *harness) presenceMode(t *testing.T) string {
	t.Helper()
	mode, err := h.settings.ResolveStringForTenant(context.Background(), testpkg.Tenant(t), configModel.KeyPresenceMode)
	require.NoError(t, err)
	return mode
}

func step(t *testing.T, status schoolsetup.Status, key string) schoolsetup.Step {
	t.Helper()
	for _, candidate := range status.Steps {
		if candidate.Key == key {
			return candidate
		}
	}
	t.Fatalf("step %q missing", key)
	return schoolsetup.Step{}
}
