package planning_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-ogs/project-phoenix/modules/workforce/compose"
	planning "github.com/moto-ogs/project-phoenix/modules/workforce/internal/planning"
	api "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var svcTestCounter atomic.Int64

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), svcTestCounter.Add(1))
}

func TestShiftTypeService_CreateValidationAndNameTaken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := newShiftTypeTestRows(db)
	svc := planning.NewShiftTypeService(repos.ShiftType, slog.Default())
	ctx := testpkg.Ctx(t)

	name := uniqueName("Betreuung")
	created, err := svc.CreateShiftType(ctx, &workforce.ShiftTypeRecord{Name: name, Color: "#83CD2D", IsActive: true})
	require.NoError(t, err)
	defer func() { _, _ = svc.DeleteShiftType(ctx, created.ID) }()
	assert.Equal(t, "#83CD2D", created.Color)

	// Same name (different case) must be rejected before hitting the DB.
	_, err = svc.CreateShiftType(ctx, &workforce.ShiftTypeRecord{Name: " " + name + " ", Color: "#5080D8"})
	require.ErrorIs(t, err, planning.ErrShiftTypeNameTaken)

	// Empty name is an input error.
	_, err = svc.CreateShiftType(ctx, &workforce.ShiftTypeRecord{Name: "   ", Color: "#5080D8"})
	require.ErrorIs(t, err, planning.ErrShiftTypeInvalid)
}

func TestShiftTypeService_UpdateAndDelete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := newShiftTypeTestRows(db)
	svc := planning.NewShiftTypeService(repos.ShiftType, slog.Default())
	ctx := testpkg.Ctx(t)

	created, err := svc.CreateShiftType(ctx, &workforce.ShiftTypeRecord{Name: uniqueName("Pause"), Color: "#6B7280", IsActive: true})
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

	removal, err := svc.DeleteShiftType(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, workforce.ShiftTypeRemovalDeleted, removal, "an unused type is deleted, not archived")
	_, err = svc.GetShiftType(ctx, created.ID)
	require.ErrorIs(t, err, planning.ErrShiftTypeNotFound)
}

func TestShiftTypeService_CreateDefaultsIsIdempotent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := newShiftTypeTestRows(db)
	svc := planning.NewShiftTypeService(repos.ShiftType, slog.Default())

	// Fresh tenant so the count is deterministic.
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)

	first, err := svc.CreateDefaultShiftTypes(ctx)
	require.NoError(t, err)
	defer func() {
		for _, st := range first {
			_, _ = svc.DeleteShiftType(ctx, st.ID)
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

// TestShiftTypeService_DeleteArchivesReferencedType guards #6845: removing a
// type that a planned shift and a series still carry must not clear their
// classification through ON DELETE SET NULL. The type is archived instead,
// leaves the choice and frees its name; the next removal of an unused type
// deletes it.
func TestShiftTypeService_DeleteArchivesReferencedType(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := newShiftTypeTestRows(db)
	svc := planning.NewShiftTypeService(repos.ShiftType, slog.Default())
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "ShiftType", "Archive")
	name := uniqueName("ToArchive")
	st, err := svc.CreateShiftType(ctx, &workforce.ShiftTypeRecord{Name: name, Color: "#F78C10", IsActive: true})
	require.NoError(t, err)

	shift := &workforce.ShiftRecord{
		StaffID:     staff.ID,
		Date:        calendar.NewDate(2026, time.July, 6),
		StartTime:   time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
		EndTime:     time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
		ShiftTypeID: &st.ID,
		CreatedBy:   staff.ID,
	}
	require.NoError(t, repos.StaffShift.Create(ctx, shift))
	period := testpkg.CreateTestCalendarPeriod(t, db, uniqueName("Archive period"), calendar.NewDate(2026, time.August, 1), calendar.NewDate(2027, time.July, 31))
	series, err := repos.ShiftType.CreateStaffShiftSeries(ctx, workforce.StaffShiftSeries{
		StaffID: staff.ID, Weekdays: []int{1}, StartTime: "08:00:00", EndTime: "12:00:00", ShiftTypeID: &st.ID,
		CalendarPeriodID: period.ID, ValidFrom: "2026-08-03", CreatedBy: staff.ID,
	})
	require.NoError(t, err)

	plan, err := repos.ShiftType.ShiftPlanReferences(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[int64]int{period.ID: 1}, plan.SeriesByCalendarPeriod, "the School Calendar preview counts the series per period")
	assert.Equal(t, workforce.ShiftTypeReferences{Shifts: 1, Series: 1}, plan.ByShiftType[st.ID])

	removal, err := svc.DeleteShiftType(ctx, st.ID)
	require.NoError(t, err)
	assert.Equal(t, workforce.ShiftTypeRemovalArchived, removal, "a type planned shifts carry must be archived")

	reloaded, err := repos.StaffShift.FindByID(ctx, shift.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.ShiftTypeID, "the shift must keep its type")
	assert.Equal(t, st.ID, *reloaded.ShiftTypeID)
	reloadedSeries, err := repos.ShiftType.FindStaffShiftSeries(ctx, series.ID)
	require.NoError(t, err)
	require.NotNil(t, reloadedSeries.ShiftTypeID, "the series must keep its type")
	assert.Equal(t, st.ID, *reloadedSeries.ShiftTypeID)

	archived, err := svc.GetShiftType(ctx, st.ID)
	require.NoError(t, err, "planned shifts still resolve the archived type")
	require.NotNil(t, archived.ArchivedAt)
	_, err = svc.UpdateShiftType(ctx, archived)
	require.ErrorIs(t, err, planning.ErrShiftTypeNotFound, "an archived type is not edited")

	successor, err := svc.CreateShiftType(ctx, &workforce.ShiftTypeRecord{Name: name, Color: "#5080D8", IsActive: true})
	require.NoError(t, err, "the archived type frees its name")
	removal, err = svc.DeleteShiftType(ctx, successor.ID)
	require.NoError(t, err)
	assert.Equal(t, workforce.ShiftTypeRemovalDeleted, removal)
	_, err = svc.GetShiftType(ctx, successor.ID)
	require.ErrorIs(t, err, planning.ErrShiftTypeNotFound)

	_, err = svc.DeleteShiftType(ctx, 0)
	require.ErrorIs(t, err, planning.ErrShiftTypeNotFound)
}

// TestShiftTypeService_CreateInactivePersists guards the bun `default:true`
// gotcha: creating a shift type with IsActive=false must store false, not have
// the column DEFAULT TRUE silently win. (Regression for #1836 follow-up.)
func TestShiftTypeService_CreateInactivePersists(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := newShiftTypeTestRows(db)
	svc := planning.NewShiftTypeService(repos.ShiftType, slog.Default())
	ctx := testpkg.Ctx(t)

	created, err := svc.CreateShiftType(ctx, &workforce.ShiftTypeRecord{
		Name:     uniqueName("Inaktiv"),
		Color:    "#6B7280",
		IsActive: false,
	})
	require.NoError(t, err)
	defer func() { _, _ = svc.DeleteShiftType(ctx, created.ID) }()
	assert.False(t, created.IsActive, "create must return the inactive flag it was given")

	got, err := svc.GetShiftType(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, got.IsActive, "an inactive shift type must persist as inactive, not fall back to the column default TRUE")
}

// shiftTypeTestRows serves the retained shift and shift-type rows over the
// Workforce capability, as the composition root does (#3418).
type shiftTypeTestRows struct {
	StaffShift *shiftFixtureStore
	ShiftType  workforceCompose.WorkTime
}

func newShiftTypeTestRows(db *bun.DB) shiftTypeTestRows {
	workforce := api.MustNewWorkforce(testpkg.MustDatabase(db))
	return shiftTypeTestRows{StaffShift: newShiftFixtureStore(workforce), ShiftType: workforce}
}
