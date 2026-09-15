package compose

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var absenceTypeTestCounter atomic.Int64

// absenceTypeUnique keeps names collision-free across runs — they are unique
// per tenant, so a row left behind by an interrupted run would poison the next.
func absenceTypeUnique() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), absenceTypeTestCounter.Add(1))
}

func TestStaffAbsenceTypeRepository_CreateAndListSorted(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := buildWorkforce(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	suffix := absenceTypeUnique()
	zebra := workforce.StaffAbsenceTypeFields{Name: "Zebra-" + suffix, IsActive: true}
	alpha := workforce.StaffAbsenceTypeFields{Name: "  Alpha-" + suffix + "  ", IsActive: true}
	zebraRow, zebraErr := repo.CreateStaffAbsenceType(ctx, zebra)
	require.NoError(t, zebraErr)
	alphaRow, alphaErr := repo.CreateStaffAbsenceType(ctx, alpha)
	require.NoError(t, alphaErr)

	require.NotZero(t, zebraRow.ID)
	assert.Equal(t, tenantID, zebraRow.TenantID, "tenant_id must be stamped from context")
	assert.Equal(t, "Alpha-"+suffix, alphaRow.Name, "name must be trimmed on create")
	assert.Equal(t, workforce.AbsenceTypeOther, zebraRow.BaseType,
		"a school-defined art inherits the calculation of Sonstige")

	all, err := repo.ListStaffAbsenceTypes(ctx)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := buildWorkforce(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	suffix := absenceTypeUnique()
	first := workforce.StaffAbsenceTypeFields{Name: "Regenerationstag-" + suffix, IsActive: true}
	_, firstErr := repo.CreateStaffAbsenceType(ctx, first)
	require.NoError(t, firstErr)

	duplicate := workforce.StaffAbsenceTypeFields{Name: "REGENERATIONSTAG-" + suffix, IsActive: true}
	_, err := repo.CreateStaffAbsenceType(ctx, duplicate)
	require.Error(t, err, "the same name in different case must not be storable twice")
	var postgresErr interface {
		error
		Field(byte) string
	}
	require.ErrorAs(t, err, &postgresErr)
	assert.Equal(t, "23505", postgresErr.Field('C'))
	assert.Equal(t, "uniq_staff_absence_types_tenant_name", postgresErr.Field('n'),
		"expected the case-insensitive per-tenant name index to reject it, got %v", err)
}

func TestStaffAbsenceTypeRepository_IsolatesTenants(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := buildWorkforce(t, db)
	otherTenantID, _ := testpkg.CreateTestTenant(t, db)

	tenantAID, _ := testpkg.CreateTestTenant(t, db)
	ctxA := testpkg.TenantContext(tenantAID)
	ctxB := testpkg.TenantContext(otherTenantID)

	suffix := absenceTypeUnique()
	own := workforce.StaffAbsenceTypeFields{Name: "Ferienzeit-" + suffix, IsActive: true}
	ownRow, ownErr := repo.CreateStaffAbsenceType(ctxA, own)
	require.NoError(t, ownErr)

	// Same name, other school: not a duplicate — the uniqueness is per tenant.
	other := workforce.StaffAbsenceTypeFields{Name: "Ferienzeit-" + suffix, IsActive: true}
	_, otherErr := repo.CreateStaffAbsenceType(ctxB, other)
	require.NoError(t, otherErr)

	listB, err := repo.ListStaffAbsenceTypes(ctxB)
	require.NoError(t, err)
	for _, at := range listB {
		assert.NotEqual(t, ownRow.ID, at.ID, "school B must not see school A's Abwesenheitsarten")
	}
}

func TestStaffAbsenceTypeRepository_DeactivateKeepsRowReadable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := buildWorkforce(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	suffix := absenceTypeUnique()
	at := workforce.StaffAbsenceTypeFields{Name: "Sonderurlaub-" + suffix, IsActive: true}
	atRow, atErr := repo.CreateStaffAbsenceType(ctx, at)
	require.NoError(t, atErr)

	atRow.IsActive = false
	_, updateErr := repo.UpdateStaffAbsenceType(ctx, atRow)
	require.NoError(t, updateErr)

	all, err := repo.ListStaffAbsenceTypes(ctx)
	require.NoError(t, err)

	var found *workforce.StaffAbsenceType
	for _, candidate := range all {
		if candidate.ID == atRow.ID {
			found = &candidate
		}
	}
	require.NotNil(t, found, "a deactivated art must stay listed so historical absences keep resolving")
	assert.False(t, found.IsActive)
}
