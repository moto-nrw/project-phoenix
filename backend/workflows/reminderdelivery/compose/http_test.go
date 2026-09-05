package compose_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	remindersHTTP "github.com/moto-nrw/project-phoenix/api/reminders"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureQuery struct {
	admin  bool
	called bool
	result *reminder.Result
	err    error
}

func (q *captureQuery) ComputeForCaller(_ context.Context, admin bool) (*reminder.Result, error) {
	q.called, q.admin = true, admin
	return q.result, q.err
}

// Scope decisions use the production permission resolver, including wildcard
// admins whose JWT does not carry the literal admin role.
func TestListRemindersAdminScope(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		admin       bool
		permissions []string
		wantAdmin   bool
	}{
		{"wildcard admin without staff row", false, []string{"admin:*"}, true},
		{"full-access wildcard", false, []string{"*:*"}, true},
		{"literal admin", true, []string{"users:read"}, true},
		{"caregiver", false, []string{"users:read"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			query := &captureQuery{result: &reminder.Result{Enabled: true}}
			runtime := compose.HTTPRuntime(nil)
			// These scope tests start with authenticated claims, as the former
			// direct-handler tests did. Keep the real permission middleware.
			runtime.Protected = func(router chi.Router, register func(chi.Router, remindersHTTP.Middleware)) {
				register(router, func(next http.Handler) http.Handler { return next })
			}
			request := requestWithPrincipal(t, jwt.AppClaims{IsAdmin: tt.admin, Permissions: tt.permissions})
			recorder := httptest.NewRecorder()
			remindersHTTP.NewResource(query, runtime).Router().ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.True(t, query.called)
			assert.Equal(t, tt.wantAdmin, query.admin)
		})
	}
}

func TestReminderHTTPErrorContract(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		err    error
		status int
	}{
		{"unlinked caregiver", reminder.ErrNotLinkedToStaff, http.StatusForbidden},
		{"read failure", errors.New("identity read failed"), http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			query := &captureQuery{err: tt.err}
			runtime := compose.HTTPRuntime(nil)
			runtime.Protected = func(router chi.Router, register func(chi.Router, remindersHTTP.Middleware)) {
				register(router, func(next http.Handler) http.Handler { return next })
			}
			request := requestWithPrincipal(t, jwt.AppClaims{Permissions: []string{"users:read"}})
			recorder := httptest.NewRecorder()
			remindersHTTP.NewResource(query, runtime).Router().ServeHTTP(recorder, request)
			require.Equal(t, tt.status, recorder.Code, recorder.Body.String())
			assert.Contains(t, recorder.Body.String(), tt.err.Error())
		})
	}
}

func requestWithPrincipal(t *testing.T, claims jwt.AppClaims) *http.Request {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "reminder-scope@example.com")
	claims.ID = int(account.ID)
	claims.TenantID = testpkg.Tenant(t)
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
		AccountID: account.ID, TenantID: claims.TenantID,
		Roles: claims.Roles, Permissions: claims.Permissions, Admin: claims.IsAdmin,
	})
	require.NoError(t, err)
	request := testutil.NewRequest(http.MethodGet, "/", nil, testutil.WithClaims(t, claims))
	return request.WithContext(permissions.WithPrincipal(request.Context(), principal))
}

func TestCallerQueryRejectsUnlinkedIdentity(t *testing.T) {
	t.Parallel()
	db, module := testutil.SetupRemindersModule(t)
	for _, personLinked := range []bool{false, true} {
		name := "account without person"
		if personLinked {
			name = "person without staff"
		}
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			account := testpkg.CreateTestAccount(t, db, "unlinked-reminder@example.com")
			if personLinked {
				testpkg.CreateTestPersonWithAccountID(t, db, "Reminder", "Unlinked", account.ID)
			}
			request := testutil.NewRequest(http.MethodGet, "/", nil,
				testutil.WithClaims(t, jwt.AppClaims{ID: int(account.ID), TenantID: testpkg.Tenant(t)}))
			require.NoError(t, testpkg.WithinTenantContext(t, request.Context(), db, testpkg.Tenant(t), func(ctx context.Context) error {
				result, err := module.Reminders.ComputeForCaller(ctx, false)
				require.ErrorIs(t, err, reminder.ErrNotLinkedToStaff)
				assert.Nil(t, result)
				return nil
			}))
		})
	}
}
