package presence

import (
	"context"
	"log/slog"
	"testing"

	settingsFacts "github.com/moto-nrw/project-phoenix/modules/settings"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resourceForTest initializes policies and logging while allowing each handler test to
// supply only the collaborators its path calls.
func resourceForTest(resource Resource) *Resource {
	resource.logger = slog.Default()
	resource.displaySettings = settingsFacts.NewDisplaySettings(resource.SettingsService)
	resource.authorization = authorizationForTest()
	resource.runtime.TenantID = tenant.FromContext
	resource.runtime.MarkRollback = tenant.MarkRollback
	return &resource
}

func authorizeVisitForTest(ctx context.Context, accountID int64, readAll bool, visitID int64, facts VisitAccessQuery) (bool, error) {
	return securityruntime.CanViewVisit(ctx, accountID, readAll, visitID, facts)
}

func authorizationForTest() Authorization {
	return Authorization{
		Visit: authorizeVisitForTest,
		StudentDataAccess: func(ctx context.Context, lookup func(context.Context) (bool, error)) (bool, bool) {
			return securityruntime.StudentDataAccess(ctx, testpkg.PermissionsFromContext(ctx), lookup)
		},
		OperationalOverview: func(ctx context.Context, settings Settings, staff StaffAccess, assignmentBound, admin bool) (bool, error) {
			return securityruntime.CanViewOperationalOverview(ctx, settings, staff, assignmentBound, admin)
		},
	}
}

func requestRuntimeForTest(withStaff func(context.Context, int64, int64) context.Context) RequestRuntime {
	return RequestRuntime{WithStaff: withStaff, TenantID: tenant.FromContext, MarkRollback: tenant.MarkRollback}
}

func TestResourceRequiresLogger(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "active API: logger is required", func() {
		NewResource(nil, nil, nil, nil, nil, nil, nil, nil, RequestRuntime{}, Authorization{}, settingsFacts.NewDisplaySettings(nil))
	})
}

func TestResourceUsesInjectedLogger(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.DiscardHandler)
	resource := NewResource(nil, nil, nil, nil, nil,
		func(testutil.Router, func(testutil.Router, common.Middleware)) {}, logger, mappingReadQueries{},
		requestRuntimeForTest(func(ctx context.Context, _, _ int64) context.Context { return ctx }), authorizationForTest(), settingsFacts.NewDisplaySettings(nil))
	assert.Same(t, logger, resource.getLogger())
}

func TestResourceRequiresVisitAuthorization(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "active API: visit authorization is required", func() {
		NewResource(nil, nil, nil, nil, nil,
			func(testutil.Router, func(testutil.Router, common.Middleware)) {}, slog.Default(), mappingReadQueries{},
			requestRuntimeForTest(func(ctx context.Context, _, _ int64) context.Context { return ctx }), Authorization{}, settingsFacts.NewDisplaySettings(nil))
	})
}

func TestResourceRequiresOverviewAuthorization(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "active API: overview authorization is required", func() {
		NewResource(nil, nil, nil, nil, nil,
			func(testutil.Router, func(testutil.Router, common.Middleware)) {}, slog.Default(), mappingReadQueries{},
			requestRuntimeForTest(func(ctx context.Context, _, _ int64) context.Context { return ctx }), Authorization{Visit: authorizeVisitForTest}, settingsFacts.NewDisplaySettings(nil))
	})
}

func TestResourceRequiresTenantRequestRuntime(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"tenant ID", "rollback marker"} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()
			runtime := requestRuntimeForTest(func(ctx context.Context, _, _ int64) context.Context { return ctx })
			if missing == "tenant ID" {
				runtime.TenantID = nil
			} else {
				runtime.MarkRollback = nil
			}
			require.PanicsWithValue(t, "active API: tenant request runtime is required", func() {
				NewResource(nil, nil, nil, nil, nil,
					func(testutil.Router, func(testutil.Router, common.Middleware)) {}, slog.Default(), mappingReadQueries{},
					runtime, authorizationForTest(), settingsFacts.NewDisplaySettings(nil))
			})
		})
	}
}

func TestResourceRequiresStudentDataAuthorization(t *testing.T) {
	t.Parallel()
	authorization := authorizationForTest()
	authorization.StudentDataAccess = nil
	require.PanicsWithValue(t, "active API: student data authorization is required", func() {
		NewResource(nil, nil, nil, nil, nil,
			func(testutil.Router, func(testutil.Router, common.Middleware)) {}, slog.Default(), mappingReadQueries{},
			requestRuntimeForTest(func(ctx context.Context, _, _ int64) context.Context { return ctx }), authorization, settingsFacts.NewDisplaySettings(nil))
	})
}

func TestResourceRequiresDisplaySettings(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "active API: display settings facts are required", func() {
		NewResource(nil, nil, nil, nil, nil,
			func(testutil.Router, func(testutil.Router, common.Middleware)) {}, slog.Default(), mappingReadQueries{},
			requestRuntimeForTest(func(ctx context.Context, _, _ int64) context.Context { return ctx }), authorizationForTest(), nil)
	})
}
