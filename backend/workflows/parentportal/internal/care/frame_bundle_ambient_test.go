package care_test

// Ambient entries of ADR 0161 point 7 for steps 3 and 4: called from no
// unit, from an open administrative unit and from a school unit of the same
// and of another school, the bundled reads end like the standalone reads
// they replace: read, refused or failed.

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// portalAccess is the stored parent-portal permission the read routes ask for.
const portalAccess = "parent_portal.access"

// ambientEntries opens each entry of point 7 around run.
func ambientEntries(t *testing.T, schoolA, schoolB int64) map[string]func(context.Context, func(context.Context)) {
	t.Helper()
	inSchool := func(id int64) func(context.Context, func(context.Context)) {
		return func(ctx context.Context, run func(context.Context)) {
			school, err := tenant.NewTenantID(id)
			require.NoError(t, err)
			require.NoError(t, tenant.WithinTenant(ctx, school, func(txCtx context.Context) error { run(txCtx); return nil }))
		}
	}
	return map[string]func(context.Context, func(context.Context)){
		"no unit": func(ctx context.Context, run func(context.Context)) { run(ctx) },
		"administrative unit": func(ctx context.Context, run func(context.Context)) {
			require.NoError(t, tenant.WithinAdmin(ctx, func(txCtx context.Context) error { run(txCtx); return nil }))
		},
		"unit of the same school": inSchool(schoolA),
		"unit of another school":  inSchool(schoolB),
	}
}

// outcome classifies a read as the caller sees it.
func outcome(tenantID int64, err error, readSchool string) string {
	switch {
	case err != nil:
		return "failed"
	case tenantID == 0:
		return "refused"
	default:
		return "read " + readSchool
	}
}

func schoolRead(t *testing.T, school *string) func(context.Context, int64) error {
	t.Helper()
	return func(ctx context.Context, _ int64) error {
		raw, ok := tenant.TransactionFromContext(ctx)
		require.True(t, ok)
		return raw.(bun.Tx).NewRaw("SELECT coalesce(current_setting('app.current_tenant_id', true), '')").Scan(ctx, school)
	}
}

func TestGuardianAnnouncementFirstPassEndsLikeTheStandaloneReads(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	seedCtx := tenant.WithTenantID(ctx, chain.TenantID)
	require.NoError(t, settings.Settings.SetValue(seedCtx, parentportal.KeyParentNewsEnabled, true, nil, nil))
	repos := api.NewRepositoryTestFactory(database)
	reposPeople := api.NewPeopleRepositorySuiteFactory(database)
	reposMessages := api.NewParentMessagingSuiteRepositories(database)
	svc := parentportal.NewMessagingFlows(parentportal.MessagingConfig{
		SharePolicy: parentportalcompose.NewRequestSharePolicy(), GuardianPolicy: owners.NewParentGuardianPolicy(),
		Fingerprints: owners.SecurityFingerprints{}, Declarations: owners.ParentDeclarationPolicy(),
		Signatures: owners.ParentDeclarationSignatures{Ledger: owners.SignatureLedger()}, Runtime: owners.ParentPortalRuntime{},
		Children: childResolver(db, repos, reposPeople), ChildRepo: repos.ParentChildReads(), EnrollmentRequestRepo: repos.ParentEnrollmentReads(),
		AnnouncementRepo: owners.BindParentAnnouncementAccess(reposMessages.ParentAnnouncement), StudentRepo: owners.NewParentStudentReads(reposPeople.Student),
		Settings: settings.Settings, Logger: slog.Default(),
	})
	ann := seedPublishedAnnouncement(t, seedCtx, api.NewParentAnnouncementFixtures(reposMessages.ParentAnnouncement), chain.AccountID, chain.TenantID, false)
	var schoolB int64
	t.Run("school B", func(t *testing.T) {
		schoolB = testpkg.OwnTenant(t)
		testpkg.EnsureTestTenant(t, db, schoolB)
	})

	for name, entry := range ambientEntries(t, chain.TenantID, schoolB) {
		var standalone, bundled string
		entry(ctx, func(ctx context.Context) {
			var school string
			tenantID, err := svc.GuardianAnnouncementTenant(ctx, chain.AccountID, ann.ID)
			if err == nil && tenantID > 0 {
				id, idErr := tenant.NewTenantID(tenantID)
				require.NoError(t, idErr)
				err = tenant.WithinTenant(ctx, id, func(txCtx context.Context) error { return schoolRead(t, &school)(txCtx, tenantID) })
			}
			standalone = outcome(tenantID, err, school)
		})
		entry(ctx, func(ctx context.Context) {
			var school string
			tenantID, err := svc.WithinGuardianAnnouncementTenant(ctx, chain.AccountID, ann.ID, schoolRead(t, &school))
			bundled = outcome(tenantID, err, school)
		})
		require.Equal(t, standalone, bundled, name)
		if name == "no unit" {
			require.Equal(t, "read "+fmt.Sprint(chain.TenantID), bundled)
		}
	}
}

func TestPermittedChildReadEndsLikeTheStandaloneReads(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	children := childResolver(db, api.NewRepositoryTestFactory(database), api.NewPeopleRepositorySuiteFactory(database))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var schoolB int64
	t.Run("school B", func(t *testing.T) {
		schoolB = testpkg.OwnTenant(t)
		testpkg.EnsureTestTenant(t, db, schoolB)
	})

	for name, entry := range ambientEntries(t, chain.TenantID, schoolB) {
		var standalone, bundled string
		entry(ctx, func(ctx context.Context) {
			var school string
			var tenantID int64
			child, err := children.ResolvePermittedChild(ctx, chain.AccountID, chain.StudentID, portalAccess)
			if err == nil {
				tenantID = child.TenantID
				id, idErr := tenant.NewTenantID(tenantID)
				require.NoError(t, idErr)
				err = tenant.WithinTenant(ctx, id, func(txCtx context.Context) error { return schoolRead(t, &school)(txCtx, tenantID) })
			}
			standalone = outcome(tenantID, err, school)
		})
		entry(ctx, func(ctx context.Context) {
			var school string
			var tenantID int64
			err := children.WithPermittedChild(ctx, chain.AccountID, chain.StudentID, portalAccess, nil, func(txCtx context.Context, child *parentportal.Child) error {
				tenantID = child.TenantID
				return schoolRead(t, &school)(txCtx, tenantID)
			})
			bundled = outcome(tenantID, err, school)
		})
		require.Equal(t, standalone, bundled, name)
		if name == "no unit" {
			require.Equal(t, "read "+fmt.Sprint(chain.TenantID), bundled)
		}
	}
}
