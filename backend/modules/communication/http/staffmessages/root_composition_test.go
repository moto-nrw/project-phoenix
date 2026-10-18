package staffmessaging_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

// setupStaffMessagingModule returns the module with the tenant setting
// switch the suites pin the chat with.
func setupStaffMessagingModule(t *testing.T) (*bun.DB, api.StaffMessagingTestModule, func(context.Context, string, bool) error) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, settings, err := api.NewStaffMessagingTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	setSetting := func(ctx context.Context, key string, value bool) error {
		return settings.Settings.SetValue(ctx, key, value, nil, nil)
	}
	return db, module, setSetting
}
