package compose_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestResolveForAbsenceRejectsDeactivatedArt(t *testing.T) {
	t.Parallel()
	_, module := setupAbsenceTypeModule(t)
	ctx := testpkg.Ctx(t)
	typ, err := module.Catalog.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Sonderurlaub"})
	require.NoError(t, err)
	inactive := false
	_, err = module.Catalog.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: typ.ID, IsActive: &inactive})
	require.NoError(t, err)
	value, err := timetracking.AbsenceTypes(module.Catalog).ResolveForAbsence(ctx, typ.ID)
	require.Nil(t, value)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeInactive)
	require.EqualError(t, err, "diese Abwesenheitsart ist deaktiviert")
}

func TestResolveForAbsenceRejectsUnknownID(t *testing.T) {
	t.Parallel()
	db, module := setupAbsenceTypeModule(t)
	ctx := testpkg.Ctx(t)
	typ, err := module.Catalog.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Foreign type"})
	require.NoError(t, err)
	foreignID, _ := testpkg.CreateTestTenant(t, db)
	value, err := timetracking.AbsenceTypes(module.Catalog).ResolveForAbsence(testpkg.TenantContext(foreignID), typ.ID)
	require.Nil(t, value)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNotFound)
	require.EqualError(t, err, "diese Abwesenheitsart gibt es nicht")
}
