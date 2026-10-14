package compose_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
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
	value, err := workforceCompose.AbsenceTypes(module.Catalog).ResolveForAbsence(ctx, typ.ID)
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
	value, err := workforceCompose.AbsenceTypes(module.Catalog).ResolveForAbsence(testpkg.TenantContext(foreignID), typ.ID)
	require.Nil(t, value)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNotFound)
	require.EqualError(t, err, "diese Abwesenheitsart gibt es nicht")
}
