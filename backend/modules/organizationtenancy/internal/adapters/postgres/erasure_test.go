package postgres

import (
	"context"
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

func TestErasureSchoolLockIsReleasedBeforeReturn(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	id := testpkg.Tenant(t)
	store, err := NewErasureStore(db)
	require.NoError(t, err)
	// Keep the next caller connected so a new connection handshake cannot hide
	// a lock that PostgreSQL has not released yet after session disposal.
	next, err := db.Conn(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, next.Close()) })
	failed := errors.New("synthetic phase failure")
	for _, scenario := range []string{"success", "failure", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			for range 100 {
				ctx, cancel := context.WithCancel(t.Context())
				err = store.WithSchoolLock(ctx, id, func(context.Context) error {
					if scenario == "success" {
						return nil
					}
					if scenario == "cancelled" {
						cancel()
					}
					return failed
				})
				cancel()
				if scenario == "success" {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, failed)
				}
				var locked bool
				require.NoError(t, next.NewRaw(`SELECT pg_try_advisory_lock(hashtextextended('school-erasure-cli:' || ?::text,0))`, id).Scan(t.Context(), &locked))
				require.True(t, locked, "a connected caller must acquire the lock immediately after return")
				_, err = next.ExecContext(t.Context(), `SELECT pg_advisory_unlock(hashtextextended('school-erasure-cli:' || ?::text,0))`, id)
				require.NoError(t, err)
			}
		})
	}
}

func TestErasureSchoolLockRejectsOverlappingRunsOnlyForSameSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	id := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	store, err := NewErasureStore(db)
	require.NoError(t, err)
	ctx := t.Context()
	require.NoError(t, store.WithSchoolLock(ctx, id, func(context.Context) error {
		for range 2 {
			err := store.WithSchoolLock(ctx, id, func(context.Context) error {
				t.Error("an overlapping run must not enter the locked school")
				return nil
			})
			require.ErrorContains(t, err, "school already running")
		}
		return store.WithSchoolLock(ctx, other, func(context.Context) error { return nil })
	}))
	require.NoError(t, store.WithSchoolLock(ctx, id, func(context.Context) error { return nil }))
}
