package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCustomAbsenceTypeContract(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	capability := buildWorkforce(t, db)
	ctx := testpkg.Ctx(t)
	created, err := capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Original"})
	require.NoError(t, err)
	_, err = capability.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Taken"})
	require.NoError(t, err)
	disabled := false
	for _, tc := range []struct {
		name string
		want error
	}{
		{name: "  ", want: workforce.ErrAbsenceTypeInvalid},
		{name: "Urlaub", want: workforce.ErrAbsenceTypeNameReserved},
		{name: " taken ", want: workforce.ErrAbsenceTypeNameTaken},
	} {
		_, err := capability.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: created.ID, Name: &tc.name, IsActive: &disabled})
		require.ErrorIs(t, err, tc.want)
	}
	unchanged, err := capability.FindStaffAbsenceType(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, unchanged, "rejected renames must not apply any field")

	renamed := "  Renamed  "
	enabled := true
	block := workforce.AbsenceTypeOverrunBlock
	updated, err := capability.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: created.ID, Name: &renamed, AllowanceEnabled: &enabled, OverrunPolicy: &block})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Name)
	assert.True(t, updated.AllowanceEnabled)
	assert.Equal(t, block, updated.OverrunPolicy)
	assert.Equal(t, created.BaseType, updated.BaseType)

	staff := testpkg.CreateTestStaff(t, db, "Type", "Usage")
	absence := testAbsence(staff.ID, workforce.AbsenceTypeOther, workforce.AbsenceStatusApproved, timezone.TodayDate())
	absence.AbsenceTypeID = &created.ID
	_, err = capability.CreateStaffAbsence(ctx, absence)
	require.NoError(t, err)
	newName := "Changed history"
	_, err = capability.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: created.ID, Name: &newName, IsActive: &disabled})
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeInUse)
	updated, err = capability.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: created.ID, Name: &renamed, IsActive: &disabled})
	require.NoError(t, err, "an unchanged normalized name may be deactivated after use")
	assert.False(t, updated.IsActive)
	assert.Equal(t, "Renamed", updated.Name)

	foreignID, _ := testpkg.CreateTestTenant(t, db)
	_, err = capability.UpdateAbsenceType(testpkg.TenantContext(foreignID), workforce.UpdateAbsenceType{ID: created.ID, Name: &newName})
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNotFound)
}
