package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var svcTestCounter atomic.Int64

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), svcTestCounter.Add(1))
}

func TestShiftTypeService_CreateValidationAndNameTaken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := scheduleSvc.NewShiftTypeService(repos.ShiftType, slog.Default())
	ctx := testpkg.Ctx(t)

	name := uniqueName("Betreuung")
	created, err := svc.CreateShiftType(ctx, &scheduleModels.ShiftType{Name: name, Color: "#83CD2D", IsActive: true})
	require.NoError(t, err)
	defer func() { _ = svc.DeleteShiftType(ctx, created.ID) }()
	assert.Equal(t, "#83CD2D", created.Color)

	// Same name (different case) must be rejected before hitting the DB.
	_, err = svc.CreateShiftType(ctx, &scheduleModels.ShiftType{Name: " " + name + " ", Color: "#5080D8"})
	require.ErrorIs(t, err, scheduleSvc.ErrShiftTypeNameTaken)

	// Empty name is an input error.
	_, err = svc.CreateShiftType(ctx, &scheduleModels.ShiftType{Name: "   ", Color: "#5080D8"})
	require.ErrorIs(t, err, scheduleSvc.ErrShiftTypeInvalid)
}

func TestShiftTypeService_UpdateAndDelete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := scheduleSvc.NewShiftTypeService(repos.ShiftType, slog.Default())
	ctx := testpkg.Ctx(t)

	created, err := svc.CreateShiftType(ctx, &scheduleModels.ShiftType{Name: uniqueName("Pause"), Color: "#6B7280", IsActive: true})
	require.NoError(t, err)

	created.Name = uniqueName("Pause-neu")
	created.IsActive = false
	updated, err := svc.UpdateShiftType(ctx, created)
	require.NoError(t, err)
	assert.False(t, updated.IsActive)

	got, err := svc.GetShiftType(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Name, got.Name)
	assert.False(t, got.IsActive)

	require.NoError(t, svc.DeleteShiftType(ctx, created.ID))
	_, err = svc.GetShiftType(ctx, created.ID)
	require.ErrorIs(t, err, scheduleSvc.ErrShiftTypeNotFound)
}

func TestShiftTypeService_CreateDefaultsIsIdempotent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := scheduleSvc.NewShiftTypeService(repos.ShiftType, slog.Default())

	// Fresh tenant so the count is deterministic.
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)

	first, err := svc.CreateDefaultShiftTypes(ctx)
	require.NoError(t, err)
	defer func() {
		for _, st := range first {
			_ = svc.DeleteShiftType(ctx, st.ID)
		}
	}()
	require.Len(t, first, 5, "seeding must create the five example shift types")

	names := map[string]bool{}
	for _, st := range first {
		names[st.Name] = true
	}
	assert.True(t, names["Betreuung / Zeit am Kind"])
	assert.True(t, names["Pause"])
	assert.True(t, names["Vertretungsunterricht"])

	// Seeding again must not duplicate: same rows, same count.
	second, err := svc.CreateDefaultShiftTypes(ctx)
	require.NoError(t, err)
	assert.Len(t, second, 5, "re-seeding must be idempotent")
}

func TestShiftTypeService_DeleteNullsReferencingShift(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := scheduleSvc.NewShiftTypeService(repos.ShiftType, slog.Default())
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "ShiftType", "FKNull")

	st, err := svc.CreateShiftType(ctx, &scheduleModels.ShiftType{Name: uniqueName("ToDelete"), Color: "#F78C10", IsActive: true})
	require.NoError(t, err)

	// A shift that references the type.
	shift := &scheduleModels.StaffShift{
		StaffID:     staff.ID,
		Date:        scheduleModels.NewDate(2026, time.July, 6),
		StartTime:   time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
		EndTime:     time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
		ShiftTypeID: &st.ID,
		CreatedBy:   staff.ID,
	}
	require.NoError(t, repos.StaffShift.Create(ctx, shift))
	defer func() { _ = repos.StaffShift.Delete(ctx, shift.ID) }()
	require.NotNil(t, shift.ShiftTypeID)

	// Deleting the type must keep the shift but drop the reference (SET NULL).
	require.NoError(t, svc.DeleteShiftType(ctx, st.ID))

	reloaded, err := repos.StaffShift.FindByID(ctx, shift.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded, "shift must survive deletion of its type")
	assert.Nil(t, reloaded.ShiftTypeID, "ON DELETE SET NULL must clear the shift's shift_type_id")

	// The referenced error path: deleting a non-existent type.
	err = svc.DeleteShiftType(ctx, 0)
	require.ErrorIs(t, err, scheduleSvc.ErrShiftTypeNotFound)
}

// TestShiftTypeService_CreateInactivePersists guards the bun `default:true`
// gotcha: creating a shift type with IsActive=false must store false, not have
// the column DEFAULT TRUE silently win. (Regression for #1836 follow-up.)
func TestShiftTypeService_CreateInactivePersists(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := scheduleSvc.NewShiftTypeService(repos.ShiftType, slog.Default())
	ctx := testpkg.Ctx(t)

	created, err := svc.CreateShiftType(ctx, &scheduleModels.ShiftType{
		Name:     uniqueName("Inaktiv"),
		Color:    "#6B7280",
		IsActive: false,
	})
	require.NoError(t, err)
	defer func() { _ = svc.DeleteShiftType(ctx, created.ID) }()
	assert.False(t, created.IsActive, "create must return the inactive flag it was given")

	got, err := svc.GetShiftType(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, got.IsActive, "an inactive shift type must persist as inactive, not fall back to the column default TRUE")
}
