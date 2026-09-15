package inbound

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbsenceTypeReaderUsesNativeOwner(t *testing.T) {
	t.Parallel()
	_, module := testutil.SetupAbsenceTypeModule(t)
	ctx := testpkg.Ctx(t)
	typ, err := module.Catalog.CreateAbsenceType(ctx, workforce.CreateAbsenceType{Name: "Reader contract", AllowanceEnabled: true})
	require.NoError(t, err)
	reader := services.AbsenceTypes(module.Catalog)
	value, err := reader.GetAbsenceType(ctx, typ.ID)
	require.NoError(t, err)
	require.NotNil(t, value)
	assert.Equal(t, typ.ID, value.ID)
	assert.Equal(t, typ.TenantID, value.TenantID)
	assert.Equal(t, typ.Name, value.Name)
	assert.Equal(t, typ.BaseType, value.BaseType)
	assert.Equal(t, typ.AllowanceEnabled, value.AllowanceEnabled)
	assert.Equal(t, typ.OverrunPolicy, value.OverrunPolicy)
	assert.Equal(t, typ.CreatedAt, value.CreatedAt)
	assert.Equal(t, typ.UpdatedAt, value.UpdatedAt)
	_, err = reader.ResolveForAbsence(ctx, typ.ID)
	require.NoError(t, err)
	inactive := false
	_, err = module.Catalog.UpdateAbsenceType(ctx, workforce.UpdateAbsenceType{ID: typ.ID, IsActive: &inactive})
	require.NoError(t, err)
	value, err = reader.ResolveForAbsence(ctx, typ.ID)
	require.EqualError(t, err, "diese Abwesenheitsart ist deaktiviert")
	assert.Nil(t, value, "retired types must reject new bookings")
	labels, err := reader.LabelsByID(ctx)
	require.NoError(t, err)
	assert.Equal(t, typ.Name, labels[typ.ID], "retirement must preserve historical labels")
	_, err = reader.GetAbsenceType(ctx, 0)
	require.EqualError(t, err, "diese Abwesenheitsart gibt es nicht")
	_, err = reader.ResolveForAbsence(ctx, 0)
	require.EqualError(t, err, "diese Abwesenheitsart gibt es nicht")
	failure := errors.New("read unavailable")
	broken := services.AbsenceTypes(nativeAbsenceTypeReadFailure{AbsenceTypeQuery: module.Catalog, failure: failure})
	value, err = broken.GetAbsenceType(ctx, typ.ID)
	assert.Nil(t, value)
	require.ErrorIs(t, err, failure)
	assert.EqualError(t, err, "find absence type: database error during find by id: read unavailable")
	value, err = broken.ResolveForAbsence(ctx, typ.ID)
	assert.Nil(t, value)
	require.ErrorIs(t, err, failure)
	assert.EqualError(t, err, "lock absence type: database error during lock staff absence type: read unavailable")
	labels, err = broken.LabelsByID(ctx)
	assert.Nil(t, labels)
	require.ErrorIs(t, err, failure)
	assert.EqualError(t, err, "database error during list all staff absence types: read unavailable")
}

type nativeAbsenceTypeReadFailure struct {
	workforce.AbsenceTypeQuery
	failure error
}

func (c nativeAbsenceTypeReadFailure) FindStaffAbsenceType(context.Context, int64) (workforce.StaffAbsenceType, error) {
	return workforce.StaffAbsenceType{}, c.failure
}

func (c nativeAbsenceTypeReadFailure) LockStaffAbsenceType(context.Context, int64) (workforce.StaffAbsenceType, error) {
	return workforce.StaffAbsenceType{}, c.failure
}

func (c nativeAbsenceTypeReadFailure) ListStaffAbsenceTypes(context.Context) ([]workforce.StaffAbsenceType, error) {
	return nil, c.failure
}

func TestAbsenceTypeAdministrationNativeList(t *testing.T) {
	t.Parallel()

	_, module := testutil.SetupAbsenceTypeModule(t)

	t.Run("returns native rows without a legacy read", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		rows := []workforce.StaffAbsenceType{{Name: "Training", IsActive: false, AllowanceEnabled: true}}
		admin := services.AbsenceTypeAdministration(nativeAbsenceTypeCatalog{Capability: module.Catalog, list: func(got context.Context) ([]workforce.StaffAbsenceType, error) {
			assert.Same(t, ctx, got)
			return rows, nil
		}}, slog.Default())
		got, err := admin.ListAbsenceTypes(ctx)
		require.NoError(t, err)
		assert.Equal(t, rows, got)
	})

	t.Run("empty response remains an array", func(t *testing.T) {
		admin := services.AbsenceTypeAdministration(nativeAbsenceTypeCatalog{Capability: module.Catalog, list: func(context.Context) ([]workforce.StaffAbsenceType, error) {
			return nil, nil
		}}, slog.Default())
		got, err := admin.ListAbsenceTypes(context.Background())
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("failure discards partial rows and preserves cause and wording", func(t *testing.T) {
		failure := errors.New("query failed")
		admin := services.AbsenceTypeAdministration(nativeAbsenceTypeCatalog{Capability: module.Catalog, list: func(context.Context) ([]workforce.StaffAbsenceType, error) {
			return []workforce.StaffAbsenceType{{Name: "Partial"}}, failure
		}}, slog.Default())
		got, err := admin.ListAbsenceTypes(context.Background())
		assert.Nil(t, got)
		require.ErrorIs(t, err, failure)
		assert.EqualError(t, err, "database error during list all staff absence types: query failed")
	})
}

type nativeAbsenceTypeCatalog struct {
	workforce.Capability
	list func(context.Context) ([]workforce.StaffAbsenceType, error)
}

func (c nativeAbsenceTypeCatalog) ListStaffAbsenceTypes(ctx context.Context) ([]workforce.StaffAbsenceType, error) {
	return c.list(ctx)
}
