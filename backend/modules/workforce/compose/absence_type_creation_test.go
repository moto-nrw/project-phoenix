package compose

import (
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCustomAbsenceTypeContract(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	capability := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)

	for _, name := range []string{"Urlaub", " KRANK ", "Krankmeldung", "Fortbildung", "Sonstige", "Sonstiges", "Freizeitausgleich", "Sonstige Abwesenheit"} {
		_, err := capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: name})
		require.ErrorIs(t, err, workforce.ErrAbsenceTypeNameReserved, name)
	}
	for _, name := range []string{" ", strings.Repeat("ä", 101)} {
		_, err := capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: name})
		require.ErrorIs(t, err, workforce.ErrAbsenceTypeInvalid)
	}
	rows, err := capability.ListStaffAbsenceTypes(ctx)
	require.NoError(t, err)
	require.Empty(t, rows, "rejected requests must not write a row")

	created, err := capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{
		Name: "  Regenerationstag ", AllowanceEnabled: true,
	})
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "Regenerationstag", created.Name)
	assert.Equal(t, workforce.AbsenceTypeOther, created.BaseType)
	assert.True(t, created.IsActive)
	assert.True(t, created.AllowanceEnabled)
	_, err = capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "REGENERATIONSTAG"})
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNameTaken)
	rows, err = capability.ListStaffAbsenceTypes(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
