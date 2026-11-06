//go:build testhooks

package users_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	appAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Invitations are outside this read-only route's graph. The embedded port
// deliberately fails if the route unexpectedly starts using them.
type previewUnusedInvitations struct {
	appAPI.GuardianDirectoryInvitations
}

// Use regular production constructors, not the shared test service graph.
func previewLockRouter(t *testing.T, db *bun.DB) chi.Router {
	t.Helper()
	database := testpkg.NewDatabase(t, db)
	guardians := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{
		GuardianProfileRepo:     owners.NewGuardianProfileRepository(database),
		GuardianPhoneNumberRepo: peopleCompose.NewGuardianPhoneNumberRepository(db),
		StudentGuardianRepo:     owners.NewStudentGuardianRepository(database),
		StudentRepo:             owners.NewStudentRepository(database, nil, nil),
		PersonRepo:              peopleCompose.NewPersonRepository(db, nil),
	}, db)
	runtime := peopleCompose.NewLegacyRepositoryRuntime(db)
	people, err := owners.NewPeopleDirectoryWithObserver(database, func(peopleCompose.Observation) {},
		&peopleCompose.GuardianDirectoryProvider{Guardians: guardians, Runtime: runtime, PermissionGranted: securityruntime.GuardianPermissionGranted}, nil)
	require.NoError(t, err)
	resource := appAPI.NewGuardiansResource(people, appAPI.NewGuardianDirectoryRuntime(guardians, lists.NewRendererService(),
		previewUnusedInvitations{}, func(context.Context) (bool, error) { return false, nil }, runtime), "", slog.Default())
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/api/guardians", resource.Router())
	return router
}

func TestGuardianDeletePreviewPhoneLockTimeout(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	router := previewLockRouter(t, db)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "preview-phone-lock")
	student := testpkg.CreateTestStudent(t, db, "Preview", "Child", "2a")
	testpkg.CreateTestStudentGuardianLink(t, db, student.ID, guardian.ID, "primary_guardian")
	token := testutil.MintTestJWT(t, testutil.AdminTestClaims(999))
	ctx := testpkg.Ctx(t)
	locker, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, locker.Rollback()) }()
	_, err = locker.ExecContext(ctx, "LOCK TABLE users.guardian_phone_numbers IN ACCESS EXCLUSIVE MODE")
	require.NoError(t, err)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		_, setErr := tx.ExecContext(txCtx, "SET LOCAL lock_timeout = '100ms'")
		require.NoError(t, setErr)
		_, writeErr := tx.ExecContext(txCtx, "UPDATE users.guardian_profiles SET notes = 'rollback-witness' WHERE id = ?", guardian.ID)
		require.NoError(t, writeErr)
		req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/api/guardians/%d/delete-preview", guardian.ID), nil, testutil.WithJWTBearer(token))
		req = req.WithContext(counter.Context(txCtx))
		counter.Reset()
		counter.Start()
		rr := testutil.ExecuteRequest(router, req)
		counter.Stop()
		t.Logf("status=%d body=%s", rr.Code, rr.Body.String())
		for i, query := range counter.Queries() {
			t.Logf("SQL %d: %s", i+1, query)
		}
		require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
		// The phone failure must leave the SAME ambient tenant transaction
		// aborted, even though GetGuardianByID suppresses its local error.
		_, probeErr := tx.ExecContext(txCtx, "SELECT 1")
		require.Error(t, probeErr)
		t.Logf("ambient transaction rejects subsequent work: %v", probeErr)
		return probeErr
	})
	require.Error(t, err)
	var notes *string
	require.NoError(t, db.NewRaw("SELECT notes FROM users.guardian_profiles WHERE id = ?", guardian.ID).Scan(ctx, &notes))
	require.Nil(t, notes, "the outer tenant transaction rolled back its write")
}
