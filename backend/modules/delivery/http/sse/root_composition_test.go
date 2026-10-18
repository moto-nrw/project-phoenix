package sse_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/delivery/http/sse"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupUserContextModule(t *testing.T) (*bun.DB, *api.CallerRows) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewUserContextTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

// The root imports this package, so the internal tests reach its caller rows
// through this binding.
func init() {
	sse.ComposeSubscriptionResolver = func(t *testing.T) (*bun.DB, sse.SubscriptionResolver, sse.Runtime[realtime.Event, *realtime.Client]) {
		t.Helper()
		db, module := setupUserContextModule(t)
		return db, module, api.NewSSEHTTPRuntime()
	}
}
