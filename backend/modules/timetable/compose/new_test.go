package compose

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type observationLog struct {
	mu   sync.Mutex
	seen []Observation
}

func (l *observationLog) record(observation Observation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, observation)
}

func buildModule(t *testing.T, db *bun.DB, observers ...func(Observation)) *timetable.Module {
	t.Helper()
	observe := func(Observation) {}
	if len(observers) > 0 {
		observe = observers[0]
	}
	module, err := New(Dependencies{DB: db, Observe: observe})
	require.NoError(t, err)
	return module
}

func createCategory(t *testing.T, ctx context.Context, module *timetable.Module, name string) timetable.Category {
	t.Helper()
	category, err := module.CreateCategory(ctx, timetable.CreateCategory{Name: name, Color: "#abc"})
	require.NoError(t, err)
	return category
}

func TestNewRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	_, err := New(Dependencies{})
	require.Error(t, err)
}

func TestModuleRunsCategoryLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)

	created := createCategory(t, ctx, module, "Werken")
	assert.Positive(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "#abc", created.Color)

	found, err := module.FindCategoryByName(ctx, "Werken")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)

	updated, err := module.UpdateCategory(ctx, timetable.UpdateCategory{
		ID: created.ID, Name: "Holzwerken", Description: "Werkstatt", Color: "#123456",
	})
	require.NoError(t, err)
	assert.Equal(t, "Holzwerken", updated.Name)

	archived, err := module.ArchiveCategory(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, archived.ArchivedAt)
	_, err = module.FindCategoryByName(ctx, "Holzwerken")
	require.ErrorIs(t, err, timetable.ErrCategoryNotFound)
	_, err = module.UpdateCategory(ctx, timetable.UpdateCategory{ID: created.ID, Name: "Nein", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrCategoryArchived)

	restored, err := module.RestoreCategory(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, restored.ArchivedAt)

	listed, err := module.ListCategories(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	require.NotEmpty(t, log.seen)
	assert.Equal(t, "create_category", log.seen[0].Operation)
	assert.EqualValues(t, 1, log.seen[0].Stats.Rows)
	assert.Positive(t, log.seen[0].Stats.StatementDuration)
}

func TestModuleEnforcesActiveNameUniquenessPerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)

	first := createCategory(t, ctx, module, "Musik")
	_, err := module.CreateCategory(ctx, timetable.CreateCategory{Name: "MUSIK"})
	require.ErrorIs(t, err, timetable.ErrCategoryNameExists)
	assert.Equal(t, "category_name_exists", timetable.ErrorCode(err))

	_, err = module.ArchiveCategory(ctx, first.ID)
	require.NoError(t, err)
	second := createCategory(t, ctx, module, "musik")
	assert.NotEqual(t, first.ID, second.ID)
	_, err = module.RestoreCategory(ctx, first.ID)
	require.ErrorIs(t, err, timetable.ErrCategoryNameExists)

	var conflicts int64
	for _, observation := range log.seen {
		conflicts += observation.Stats.DuplicatePreventionConflicts
	}
	assert.EqualValues(t, 2, conflicts)
}

func TestModuleTenantIsolationHidesForeignCategories(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)

	own := createCategory(t, ctx, module, "Eigene Kategorie")
	foreign := createCategory(t, foreignCtx, module, "Fremde Kategorie")
	assert.Equal(t, foreignTenantID, foreign.TenantID)

	_, err := module.FindCategory(foreignCtx, own.ID)
	require.ErrorIs(t, err, timetable.ErrCategoryNotFound)
	_, err = module.UpdateCategory(foreignCtx, timetable.UpdateCategory{ID: own.ID, Name: "Gekapert", Color: "#abc"})
	require.ErrorIs(t, err, timetable.ErrCategoryNotFound)
	listed, err := module.ListCategories(foreignCtx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, foreign.ID, listed[0].ID)
}

func TestModuleWritesRollBackWithOuterTransactionAndRetryCleanly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	wantErr := errors.New("abort transaction")
	var categoryID int64

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		categoryID = createCategory(t, txCtx, module, "Rollback").ID
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	_, err = module.FindCategory(ctx, categoryID)
	require.ErrorIs(t, err, timetable.ErrCategoryNotFound)

	retried := createCategory(t, ctx, module, "Rollback")
	assert.Positive(t, retried.ID)
}

func TestCategoryShiftLinksValidateBeforeMutation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	category := createCategory(t, ctx, module, "Dienstplan")
	shiftTypeID := insertShiftType(t, ctx, "Betreuung")

	require.NoError(t, module.SetCategoryShiftTypeLinks(ctx, shiftTypeID, []int64{category.ID, category.ID}))
	linked, err := module.FindCategory(ctx, category.ID)
	require.NoError(t, err)
	require.NotNil(t, linked.ShiftTypeID)
	assert.Equal(t, shiftTypeID, *linked.ShiftTypeID)

	err = module.SetCategoryShiftTypeLinks(ctx, shiftTypeID, []int64{category.ID, 9_223_372_036_854_775_000})
	require.ErrorIs(t, err, timetable.ErrUnknownCategoryIDs)
	stillLinked, findErr := module.FindCategory(ctx, category.ID)
	require.NoError(t, findErr)
	require.NotNil(t, stillLinked.ShiftTypeID, "failed validation must not clear existing links")
}

func insertShiftType(t *testing.T, ctx context.Context, name string) int64 {
	t.Helper()
	var id int64
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		transaction, ok := tenant.TransactionFromContext(txCtx)
		if !ok {
			return errors.New("test transaction missing")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return fmt.Errorf("unexpected test transaction %T", transaction)
		}
		return tx.NewRaw(`
			INSERT INTO schedule.shift_types (tenant_id, name, color, is_active)
			VALUES (?, ?, '#123456', TRUE)
			RETURNING id
		`, testpkg.Tenant(t), name).Scan(txCtx, &id)
	})
	require.NoError(t, err)
	return id
}

func TestModuleRefusesUnscopedPersistence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	unscoped := testpkg.WithPackageTenantRuntime(context.Background())

	_, err := module.CreateCategory(unscoped, timetable.CreateCategory{Name: "Unscoped"})
	require.ErrorContains(t, err, "tenant ID is required")
	_, err = module.ListCategories(unscoped)
	require.ErrorContains(t, err, "tenant is required")
}

func TestCategoryColorFallbackDoesNotChangeStoredValue(t *testing.T) {
	t.Parallel()
	category := timetable.Category{}
	assert.Equal(t, timetable.DefaultCategoryColor, category.ColorOrDefault())
	assert.Empty(t, category.Color)
	assert.WithinDuration(t, time.Time{}, category.CreatedAt, 0)
}

func TestCareExitEnrollmentLifecycleIsReversibleAndIdempotent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "CareExit", "Lifecycle", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "CareExitLifecycle")

	active := insertCareExitEnrollment(t, db, testpkg.Tenant(t), student.ID, group.ID, "2026-08-01", nil)
	futureEnd := "2026-12-01"
	future := insertCareExitEnrollment(t, db, testpkg.Tenant(t), student.ID, group.ID, "2026-09-10", &futureEnd)
	closedEnd := "2026-09-01"
	closed := insertCareExitEnrollment(t, db, testpkg.Tenant(t), student.ID, group.ID, "2026-08-01", &closedEnd)

	require.NoError(t, module.LockStudentEnrollmentsForCareExit(ctx, []int64{student.ID}, "2026-09-10"))
	changes, err := module.EndStudentEnrollmentsForCareExit(ctx, []int64{student.ID}, "2026-09-10")
	require.NoError(t, err)
	require.Len(t, changes.Deleted, 1)
	require.Len(t, changes.Capped, 1)
	assert.Equal(t, future, changes.Deleted[0].ID)
	assert.Equal(t, active, changes.Capped[0].ID)
	assert.Equal(t, "2026-09-10", careExitEnrollmentEnd(t, db, testpkg.Tenant(t), active))
	assert.False(t, careExitEnrollmentExists(t, db, testpkg.Tenant(t), future))
	assert.Equal(t, closedEnd, careExitEnrollmentEnd(t, db, testpkg.Tenant(t), closed))

	removals := careExitRemovals(changes)
	restored, err := module.RestoreStudentEnrollmentsForCareExit(ctx, []int64{student.ID}, nil, removals)
	require.NoError(t, err)
	assert.Equal(t, 2, restored)
	restored, err = module.RestoreStudentEnrollmentsForCareExit(ctx, []int64{student.ID}, nil, removals)
	require.NoError(t, err)
	assert.Zero(t, restored, "a retry must not report already-restored rows as new work")
	assert.Empty(t, careExitEnrollmentEnd(t, db, testpkg.Tenant(t), active))
	assert.Equal(t, futureEnd, careExitEnrollmentEnd(t, db, testpkg.Tenant(t), future))
	assert.Positive(t, observedDuplicateConflicts(log.seen))
}

func TestCareExitEnrollmentWritesRespectTenantAndOuterRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Own", "Student", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "OwnGroup")
	own := insertCareExitEnrollment(t, db, testpkg.Tenant(t), student.ID, group.ID, "2026-08-01", nil)

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreignTenantID, "Foreign", "Student", "1b")
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "ForeignGroup")
	foreign := insertCareExitEnrollment(t, db, foreignTenantID, foreignStudent.ID, foreignGroup.ID, "2026-08-01", nil)

	wantErr := errors.New("roll back care-exit mutation")
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		changes, endErr := module.EndStudentEnrollmentsForCareExit(txCtx, []int64{student.ID, foreignStudent.ID}, "2026-09-10")
		require.NoError(t, endErr)
		require.Len(t, changes.Capped, 1)
		assert.Equal(t, own, changes.Capped[0].ID)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
	assert.Empty(t, careExitEnrollmentEnd(t, db, testpkg.Tenant(t), own))
	assert.Empty(t, careExitEnrollmentEnd(t, db, foreignTenantID, foreign))
}

func insertCareExitEnrollment(t *testing.T, db *bun.DB, tenantID, studentID, groupID int64, validFrom string, validUntil *string) int64 {
	t.Helper()
	var id int64
	err := db.NewRaw(`INSERT INTO activities.student_enrollments
		(tenant_id, student_id, activity_group_id, valid_from, valid_until)
		VALUES (?, ?, ?, ?::date, ?::date) RETURNING id`,
		tenantID, studentID, groupID, validFrom, validUntil).Scan(testpkg.WithPackageTenantRuntime(context.Background()), &id)
	require.NoError(t, err)
	return id
}

func careExitEnrollmentEnd(t *testing.T, db *bun.DB, tenantID, enrollmentID int64) string {
	t.Helper()
	var value string
	require.NoError(t, db.NewRaw(`SELECT COALESCE(valid_until::text, '') FROM activities.student_enrollments WHERE tenant_id = ? AND id = ?`, tenantID, enrollmentID).Scan(context.Background(), &value))
	return value
}

func careExitEnrollmentExists(t *testing.T, db *bun.DB, tenantID, enrollmentID int64) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM activities.student_enrollments WHERE tenant_id = ? AND id = ?)`, tenantID, enrollmentID).Scan(context.Background(), &exists))
	return exists
}

func careExitRemovals(changes timetable.CareExitEnrollmentChanges) []timetable.CareExitEnrollmentRemoval {
	result := make([]timetable.CareExitEnrollmentRemoval, 0, len(changes.Deleted)+len(changes.Capped))
	for _, enrollment := range changes.Deleted {
		result = append(result, timetable.CareExitEnrollmentRemoval{
			CareExitEnrollment: enrollment, WasDeleted: true, PreviousValidUntil: enrollment.ValidUntil,
		})
	}
	for _, enrollment := range changes.Capped {
		result = append(result, timetable.CareExitEnrollmentRemoval{
			CareExitEnrollment: timetable.CareExitEnrollment{
				ID: enrollment.ID, TenantID: enrollment.TenantID, StudentID: enrollment.StudentID,
			},
			PreviousValidUntil: enrollment.PreviousValidUntil,
		})
	}
	return result
}

func observedDuplicateConflicts(observations []Observation) int64 {
	var total int64
	for _, observation := range observations {
		total += observation.Stats.DuplicatePreventionConflicts
	}
	return total
}
