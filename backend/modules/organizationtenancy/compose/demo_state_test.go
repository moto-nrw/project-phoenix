package compose

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoSchoolStateSurvivesNewProcessAndExcludesConcurrentRunner(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.Tenant(t)
	name := fmt.Sprintf("demo-%d", schoolID)
	first, err := NewDemoSchools(db)
	require.NoError(t, err)
	second, err := NewDemoSchools(db)
	require.NoError(t, err)
	ctx := t.Context()
	require.NoError(t, first.WithDemoLease(ctx, name, func(ctx context.Context) error {
		missing, err := first.LoadDemoSchool(ctx, name)
		require.NoError(t, err)
		require.Nil(t, missing)
		err = second.WithDemoLease(ctx, name, func(context.Context) error {
			t.Error("a second runner must not provision or simulate this school")
			return nil
		})
		require.ErrorIs(t, err, organizationtenancy.ErrDemoAlreadyRunning)
		return first.RememberDemoSchool(ctx, name, organizationtenancy.DemoSchoolState{
			SchoolID: schoolID, SeedJSON: []byte(`{"profile":"vollbetrieb"}`),
		})
	}))
	require.NoError(t, second.WithDemoLease(ctx, name, func(ctx context.Context) error {
		state, err := second.LoadDemoSchool(ctx, name)
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.Equal(t, schoolID, state.SchoolID)
		assert.JSONEq(t, `{"profile":"vollbetrieb"}`, string(state.SeedJSON))
		require.Error(t, second.RememberDemoSchool(ctx, name, *state), "must not overwrite existing credentials")
		return nil
	}))
}

func TestDemoSchoolCredentialsAreNotReadableByHTTPRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, role := range []string{"phoenix_auth", "phoenix_tenant", "phoenix_admin"} {
		var readable bool
		err := db.NewRaw(`SELECT has_table_privilege(?, 'platform.demo_school_states', 'SELECT')`, role).Scan(t.Context(), &readable)
		require.NoError(t, err)
		assert.False(t, readable, role)
	}
}

func TestDemoSchoolLeaseIsReleasedAfterCancellation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	demo, err := NewDemoSchools(db)
	require.NoError(t, err)
	name := fmt.Sprintf("cancel-demo-%d", testpkg.Tenant(t))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err = demo.WithDemoLease(ctx, name, func(ctx context.Context) error {
		cancel()
		return ctx.Err()
	})
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, demo.WithDemoLease(t.Context(), name, func(context.Context) error { return nil }))
}
