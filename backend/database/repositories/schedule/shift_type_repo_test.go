package schedule_test

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
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var shiftTypeTestCounter atomic.Int64

// testUnique returns a per-run unique suffix so shift-type names never collide
// with rows left behind by an earlier interrupted run (names are unique per
// tenant).
func testUnique() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), shiftTypeTestCounter.Add(1))
}

func newShiftType(name, color string) *scheduleModels.ShiftType {
	return &scheduleModels.ShiftType{Name: name, Color: color, IsActive: true}
}

func cleanupShiftTypes(t *testing.T, repo scheduleModels.ShiftTypeRepository, ctx context.Context, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		_ = repo.Delete(ctx, id)
	}
}

func TestShiftTypeRepository_CreateNormalizesColorAndListsSorted(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ShiftType
	ctx := testpkg.Ctx(t)

	// Lower-case, missing-hash color must be normalized by Validate().
	zebra := newShiftType("Zebra-"+testUnique(), "83cd2d")
	alpha := newShiftType("Alpha-"+testUnique(), "#5080d8")
	require.NoError(t, repo.Create(ctx, zebra))
	require.NoError(t, repo.Create(ctx, alpha))
	defer cleanupShiftTypes(t, repo, ctx, zebra.ID, alpha.ID)

	require.NotZero(t, zebra.ID)
	assert.Equal(t, testpkg.Tenant(t), zebra.TenantID, "tenant_id must be stamped from context")
	assert.Equal(t, "#83CD2D", zebra.Color, "color must be normalized to upper-case hex with hash")
	assert.Equal(t, "#5080D8", alpha.Color)

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)

	// ListAll is ordered by name; find our two rows and assert Alpha precedes Zebra.
	var alphaIdx, zebraIdx = -1, -1
	for i, st := range all {
		switch st.ID {
		case alpha.ID:
			alphaIdx = i
		case zebra.ID:
			zebraIdx = i
		}
	}
	require.NotEqual(t, -1, alphaIdx)
	require.NotEqual(t, -1, zebraIdx)
	assert.Less(t, alphaIdx, zebraIdx, "ListAll must order by name ascending")
}

func TestShiftTypeRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ShiftType
	ctx := testpkg.Ctx(t)

	st := newShiftType("Update-"+testUnique(), "#83CD2D")
	require.NoError(t, repo.Create(ctx, st))
	defer cleanupShiftTypes(t, repo, ctx, st.ID)

	st.Name = "Updated-" + testUnique()
	st.Color = "#f78c10"
	st.Description = "Neue Beschreibung"
	st.IsActive = false
	require.NoError(t, repo.Update(ctx, st))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	var got *scheduleModels.ShiftType
	for _, candidate := range all {
		if candidate.ID == st.ID {
			got = candidate
		}
	}
	require.NotNil(t, got)
	assert.Equal(t, st.Name, got.Name)
	assert.Equal(t, "#F78C10", got.Color, "update must persist the normalized color")
	assert.Equal(t, "Neue Beschreibung", got.Description)
	assert.False(t, got.IsActive)
}

func TestShiftTypeRepository_UniqueNamePerTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ShiftType
	ctx := testpkg.Ctx(t)

	name := "Dup-" + testUnique()
	first := newShiftType(name, "#83CD2D")
	require.NoError(t, repo.Create(ctx, first))
	defer cleanupShiftTypes(t, repo, ctx, first.ID)

	// Case-insensitive unique index must reject a same-name row for the tenant.
	dup := newShiftType(name, "#5080D8")
	require.Error(t, repo.Create(ctx, dup), "unique(tenant_id, lower(name)) must reject the duplicate")
}

func TestShiftTypeRepository_CreateIfAbsent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ShiftType
	ctx := testpkg.Ctx(t)

	name := "Absent-" + testUnique()

	// First insert wins.
	first := newShiftType(name, "#83CD2D")
	created, err := repo.CreateIfAbsent(ctx, first)
	require.NoError(t, err)
	assert.True(t, created, "the first insert must create a row")
	require.NotZero(t, first.ID)
	defer cleanupShiftTypes(t, repo, ctx, first.ID)

	// A second insert of the same name (different case + color) must be a clean
	// no-op via ON CONFLICT (tenant_id, LOWER(name)) DO NOTHING — never an error
	// that would abort a surrounding tenant transaction.
	second := newShiftType(strings.ToUpper(name), "#5080D8")
	createdAgain, err := repo.CreateIfAbsent(ctx, second)
	require.NoError(t, err, "conflict must not surface as an error")
	assert.False(t, createdAgain, "the case-insensitive duplicate must be a no-op")

	// The clause must not have overwritten the original row.
	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	var matches int
	for _, st := range all {
		if strings.EqualFold(st.Name, name) {
			matches++
			assert.Equal(t, "#83CD2D", st.Color, "the original row must be preserved")
		}
	}
	assert.Equal(t, 1, matches, "exactly one row must exist for the name")
}

func TestShiftTypeRepository_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ShiftType
	tenant1 := testpkg.Ctx(t)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	tenant2 := tenant.WithTenantID(context.Background(), otherTenantID)

	st := newShiftType("Isolated-"+testUnique(), "#83CD2D")
	require.NoError(t, repo.Create(tenant1, st))
	defer cleanupShiftTypes(t, repo, tenant1, st.ID)

	rows, err := repo.ListAll(tenant2)
	require.NoError(t, err)
	for _, row := range rows {
		assert.NotEqual(t, st.ID, row.ID, "tenant 2 must not see tenant 1's shift types")
	}

	rows, err = repo.ListAll(tenant1)
	require.NoError(t, err)
	found := false
	for _, row := range rows {
		if row.ID == st.ID {
			found = true
		}
	}
	assert.True(t, found, "tenant 1 must see its own shift type")
}

func TestShiftTypeRepository_GuardBranches(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ShiftType
	ctx := testpkg.Ctx(t)

	// nil entity is rejected before any DB access.
	require.Error(t, repo.Create(ctx, nil), "Create(nil) must error")
	_, err := repo.CreateIfAbsent(ctx, nil)
	require.Error(t, err, "CreateIfAbsent(nil) must error")
	require.Error(t, repo.Update(ctx, nil), "Update(nil) must error")

	// Invalid model (bad color) fails validation before the insert/update.
	bad := &scheduleModels.ShiftType{Name: "Bad-" + testUnique(), Color: "not-a-color"}
	require.Error(t, repo.Create(ctx, bad), "Create with invalid color must error")
	_, err = repo.CreateIfAbsent(ctx, bad)
	require.Error(t, err, "CreateIfAbsent with invalid color must error")
	bad.ID = 1
	require.Error(t, repo.Update(ctx, bad), "Update with invalid color must error")
}

func TestShiftTypeRepository_ListWithOptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ShiftType
	ctx := testpkg.Ctx(t)

	st := newShiftType("ListOpts-"+testUnique(), "#83CD2D")
	require.NoError(t, repo.Create(ctx, st))
	defer cleanupShiftTypes(t, repo, ctx, st.ID)

	// nil options exercises the base List override without extra clauses.
	rows, err := repo.List(ctx, nil)
	require.NoError(t, err)
	found := false
	for _, row := range rows {
		if row.ID == st.ID {
			found = true
		}
	}
	assert.True(t, found, "List must return the tenant's shift type")
}
