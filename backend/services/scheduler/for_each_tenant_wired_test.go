// Integration coverage for the wired branches of forEachTenant and
// forEachTenantSettings — the paths that delegate to tenant.ForEachActive when
// both db and schoolRepo are set. The fallback branches (nil db / nil
// schoolRepo) are covered in setters_test.go.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// erroringSchoolRepo embeds the real repo and overrides only ListActive so the
// admin-tx listing step in tenant.ForEachActive returns an error. The rest of
// the interface is delegated — we only care about the one method ForEachActive
// exercises.
type erroringSchoolRepo struct {
	platform.SchoolRepository
	err error
}

func (e *erroringSchoolRepo) ListActive(_ context.Context) ([]platform.School, error) {
	return nil, e.err
}

func TestForEachTenant_Wired_InvokesFnPerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	s := &Scheduler{
		db:         db,
		schoolRepo: platformRepo.NewSchoolRepository(db),
		logger:     slog.Default(),
	}

	var invocations int
	err := s.forEachTenant(context.Background(), "wired-iter", func(_ context.Context) error {
		invocations++
		return nil
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, invocations, 1,
		"wired iteration must invoke fn at least once for the seeded test tenant")
}

func TestForEachTenantSettings_Wired_InvokesFnWithTenantID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	s := &Scheduler{
		db:         db,
		schoolRepo: platformRepo.NewSchoolRepository(db),
		logger:     slog.Default(),
	}

	var gotTenantID int64
	var invocations int
	s.forEachTenantSettings(context.Background(), "wired-settings", func(_ context.Context, tenantID int64) error {
		invocations++
		gotTenantID = tenantID
		return nil
	})

	assert.GreaterOrEqual(t, invocations, 1, "wired path must invoke fn for seeded tenant")
	assert.NotZero(t, gotTenantID, "wired path must supply the real tenant ID")
}

func TestForEachTenantSettings_Wired_ListerError_IsLogged(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	// Real repo provides the embedded type plus TxFromContext wiring; the
	// lister error comes from our override.
	s := &Scheduler{
		db: db,
		schoolRepo: &erroringSchoolRepo{
			SchoolRepository: platformRepo.NewSchoolRepository(db),
			err:              errors.New("listing exploded"),
		},
		logger: slog.Default(),
	}

	var invoked bool
	s.forEachTenantSettings(context.Background(), "wired-err", func(_ context.Context, _ int64) error {
		invoked = true
		return nil
	})
	// The error path logs and returns — fn is never invoked because the
	// tenant list failed before iteration could start.
	assert.False(t, invoked, "fn must not run when the admin listing fails")
}
