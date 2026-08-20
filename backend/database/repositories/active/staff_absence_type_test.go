package active_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var absenceTypeTestCounter atomic.Int64

// absenceTypeUnique keeps names collision-free across runs — they are unique
// per tenant, so a row left behind by an interrupted run would poison the next.
func absenceTypeUnique() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), absenceTypeTestCounter.Add(1))
}

func cleanupAbsenceTypes(t *testing.T, repo activeModels.StaffAbsenceTypeRepository, ctx context.Context, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		_ = repo.Delete(ctx, id)
	}
}

func TestStaffAbsenceTypeRepository_CreateAndListSorted(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsenceType
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, tenantID) })
	ctx := testpkg.TenantContext(tenantID)

	suffix := absenceTypeUnique()
	zebra := &activeModels.StaffAbsenceType{Name: "Zebra-" + suffix, IsActive: true}
	alpha := &activeModels.StaffAbsenceType{Name: "  Alpha-" + suffix + "  ", IsActive: true}
	require.NoError(t, repo.Create(ctx, zebra))
	require.NoError(t, repo.Create(ctx, alpha))
	defer cleanupAbsenceTypes(t, repo, ctx, zebra.ID, alpha.ID)

	require.NotZero(t, zebra.ID)
	assert.Equal(t, tenantID, zebra.TenantID, "tenant_id must be stamped from context")
	assert.Equal(t, "Alpha-"+suffix, alpha.Name, "name must be trimmed on create")
	assert.Equal(t, activeModels.AbsenceTypeOther, zebra.BaseType,
		"a school-defined art inherits the calculation of Sonstige")

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)

	var seen []string
	for _, at := range all {
		if strings.HasSuffix(at.Name, suffix) {
			seen = append(seen, at.Name)
		}
	}
	require.Equal(t, []string{"Alpha-" + suffix, "Zebra-" + suffix}, seen, "ListAll must sort by name")
}

func TestStaffAbsenceTypeRepository_RejectsDuplicateNameCaseInsensitively(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsenceType
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, tenantID) })
	ctx := testpkg.TenantContext(tenantID)

	suffix := absenceTypeUnique()
	first := &activeModels.StaffAbsenceType{Name: "Regenerationstag-" + suffix, IsActive: true}
	require.NoError(t, repo.Create(ctx, first))
	defer cleanupAbsenceTypes(t, repo, ctx, first.ID)

	duplicate := &activeModels.StaffAbsenceType{Name: "REGENERATIONSTAG-" + suffix, IsActive: true}
	err := repo.Create(ctx, duplicate)
	require.Error(t, err, "the same name in different case must not be storable twice")
	assert.True(t, base.IsUniqueViolationOn(err, "uniq_staff_absence_types_tenant_name"),
		"expected the case-insensitive per-tenant name index to reject it, got %v", err)
}

func TestStaffAbsenceTypeRepository_IsolatesTenants(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsenceType
	otherTenantID, _ := testpkg.CreateTestTenant(t, db)
	defer testpkg.CleanupTestTenant(t, db, otherTenantID)

	tenantAID, _ := testpkg.CreateTestTenant(t, db)
	defer testpkg.CleanupTestTenant(t, db, tenantAID)
	ctxA := testpkg.TenantContext(tenantAID)
	ctxB := testpkg.TenantContext(otherTenantID)

	suffix := absenceTypeUnique()
	own := &activeModels.StaffAbsenceType{Name: "Ferienzeit-" + suffix, IsActive: true}
	require.NoError(t, repo.Create(ctxA, own))
	defer cleanupAbsenceTypes(t, repo, ctxA, own.ID)

	// Same name, other school: not a duplicate — the uniqueness is per tenant.
	other := &activeModels.StaffAbsenceType{Name: "Ferienzeit-" + suffix, IsActive: true}
	require.NoError(t, repo.Create(ctxB, other))
	defer cleanupAbsenceTypes(t, repo, ctxB, other.ID)

	listB, err := repo.ListAll(ctxB)
	require.NoError(t, err)
	for _, at := range listB {
		assert.NotEqual(t, own.ID, at.ID, "school B must not see school A's Abwesenheitsarten")
	}
}

func TestStaffAbsenceTypeRepository_DeactivateKeepsRowReadable(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffAbsenceType
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, tenantID) })
	ctx := testpkg.TenantContext(tenantID)

	suffix := absenceTypeUnique()
	at := &activeModels.StaffAbsenceType{Name: "Sonderurlaub-" + suffix, IsActive: true}
	require.NoError(t, repo.Create(ctx, at))
	defer cleanupAbsenceTypes(t, repo, ctx, at.ID)

	at.IsActive = false
	require.NoError(t, repo.Update(ctx, at))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)

	var found *activeModels.StaffAbsenceType
	for _, candidate := range all {
		if candidate.ID == at.ID {
			found = candidate
		}
	}
	require.NotNil(t, found, "a deactivated art must stay listed so historical absences keep resolving")
	assert.False(t, found.IsActive)
}
