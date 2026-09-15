package schedule_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	auditRepoPkg "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	scheduleRepoPkg "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// --- SettingsService stub ---

// newStubSettingsService builds a configtest.Mock that lets tests exercise
// the resolveRetentionDays branches without a real settings DB: HasTenantOverride
// and ResolveInt/ResolveIntForTenant report the given canned values/errors.
func newStubSettingsService(hasOverride bool, hasOverrideErr error, intVal int, intErr error) *configtest.Mock {
	return &configtest.Mock{
		HasTenantOverrideFn: func(context.Context, string) (bool, error) {
			return hasOverride, hasOverrideErr
		},
		ResolveIntFn: func(context.Context, string) (int, error) {
			return intVal, intErr
		},
		ResolveIntForTenantFn: func(context.Context, int64, string) (int, error) {
			return intVal, intErr
		},
	}
}

// buildSvc wires a TimetableCleanupService with the given settings stub.
func buildSvc(db *bun.DB, settings configSvc.SettingsService) scheduleSvc.TimetableCleanupService {
	return scheduleSvc.NewTimetableCleanupService(
		scheduleRepoPkg.NewActivityInstanceRepository(db),
		scheduleRepoPkg.NewActivityExceptionRepository(db),
		testInstanceStudents(db),
		auditRepoPkg.NewDataDeletionRepository(auditRepoPkg.NewRuntime(db, auditModels.TenantIDFromContext)),
		auditRepoPkg.NewDeviationEventRepository(auditRepoPkg.NewRuntime(db, auditModels.TenantIDFromContext)),
		settings,
		slog.Default(),
	)
}

// -----------------------------------------------------------------------------
// Missing-tenant-context guards for all three public methods.
// -----------------------------------------------------------------------------

func TestCleanupExpiredTimetableData_NoTenantInContext_ReturnsError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildSvc(db, nil)

	_, err := svc.CleanupExpiredTimetableData(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant in context")
}

func TestPreviewExpiredTimetableData_NoTenantInContext_ReturnsError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildSvc(db, nil)

	_, err := svc.PreviewExpiredTimetableData(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant in context")
}

func TestGetStats_NoTenantInContext_ReturnsError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	svc := buildSvc(db, nil)

	_, err := svc.GetStats(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant in context")
}

// -----------------------------------------------------------------------------
// PreviewExpiredTimetableData happy path — counts without deleting, returns
// oldest timestamps. Exercises scanOldestTimestamp.
// -----------------------------------------------------------------------------

func TestPreviewExpiredTimetableData_CountsOldRows(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := buildSvc(f.db, nil)

	old := timezone.TodayDate().AddDays(-400)
	recent := timezone.TodayDate().AddDays(-30)

	oldInstID := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)
	recentInstID := f.newInstance(t, recent, scheduleModels.InstanceStatusPlanned, roomID, nil)

	preview, err := svc.PreviewExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, preview.InstancesToDelete, "1 old instance counted")
	assert.Equal(t, 0, preview.ExceptionsToDelete)
	assert.Equal(t, 365, preview.RetentionDays)
	require.NotNil(t, preview.OldestInstance, "oldest instance timestamp must be populated when rows match")

	// Preview MUST NOT mutate the DB.
	assertRowExists(t, f, "schedule.activity_instances", oldInstID, true, "preview must not delete")
	assertRowExists(t, f, "schedule.activity_instances", recentInstID, true)
}

func TestPreviewExpiredTimetableData_EmptyTenant_OldestIsNil(t *testing.T) {
	t.Parallel()

	f, _ := setupFixture(t)
	svc := buildSvc(f.db, nil)

	// No data → counts are 0, oldest pointers are nil.
	preview, err := svc.PreviewExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, preview.InstancesToDelete)
	assert.Equal(t, 0, preview.ExceptionsToDelete)
	assert.Nil(t, preview.OldestInstance, "no matching rows → nil oldest")
	assert.Nil(t, preview.OldestException)
}

// -----------------------------------------------------------------------------
// GetStats — total row counts + oldest timestamps regardless of window.
// -----------------------------------------------------------------------------

func TestGetStats_ReportsTotals(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := buildSvc(f.db, nil)

	// Seed: 2 instances (1 old, 1 recent), both count toward totals.
	old := timezone.TodayDate().AddDays(-400)
	recent := timezone.TodayDate().AddDays(-30)
	f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)
	f.newInstance(t, recent, scheduleModels.InstanceStatusPlanned, roomID, nil)

	stats, err := svc.GetStats(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalInstances, "stats counts all instances regardless of age")
	assert.Equal(t, 0, stats.TotalExceptions)
	assert.Equal(t, 365, stats.RetentionDays, "retention reflects registry default")
	require.NotNil(t, stats.OldestInstance, "oldest must be populated when rows exist")
	assert.Nil(t, stats.OldestException, "no exceptions → nil oldest exception")
}

// -----------------------------------------------------------------------------
// resolveRetentionDays — cover the branches not exercised by the existing
// nil-settings tests. The chain is: tenant DB override → registry default
// (both via SettingsService) → literal 365 only when settings is nil.
// -----------------------------------------------------------------------------

func TestResolveRetentionDays_TenantOverride_UsesOverriddenValue(t *testing.T) {
	t.Parallel()

	// HasTenantOverride = true, ResolveInt returns a positive value → use it.
	f, roomID := setupFixture(t)
	settings := newStubSettingsService(true, nil, 42, nil)
	svc := buildSvc(f.db, settings)

	// Insert a 50-day-old instance to verify: 42-day window deletes it, a
	// 365-day default would have spared it.
	fiftyDaysOld := timezone.TodayDate().AddDays(-50)
	f.newInstance(t, fiftyDaysOld, scheduleModels.InstanceStatusCompleted, roomID, nil)

	result, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 42, result.RetentionDays, "tenant override must win over registry default")
	assert.Equal(t, 1, result.InstancesDeleted, "50-day row past 42-day window")
}

func TestResolveRetentionDays_HasOverrideError_FallsBackToDefault(t *testing.T) {
	t.Parallel()

	// HasTenantOverride returns err — service warn-logs and falls through to
	// ResolveInt, which returns the registry default even when no override
	// exists.
	f, roomID := setupFixture(t)
	settings := newStubSettingsService(false, errors.New("settings DB down"), 365, nil)
	svc := buildSvc(f.db, settings)

	f.newInstance(t, timezone.TodayDate().AddDays(-400), scheduleModels.InstanceStatusCompleted, roomID, nil)

	result, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 365, result.RetentionDays, "override-check error must fall through to settings default")
}

func TestResolveRetentionDays_NoOverride_UsesRegistryDefault(t *testing.T) {
	t.Parallel()

	// HasTenantOverride = false → service still calls ResolveInt, which
	// returns the registry default. No tenant override means no per-school
	// customization; the default 180 (via stub) is what the service should
	// use.
	f, _ := setupFixture(t)
	settings := newStubSettingsService(false, nil, 180, nil)
	svc := buildSvc(f.db, settings)

	// Call Preview to inspect RetentionDays without having to seed data.
	preview, err := svc.PreviewExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 180, preview.RetentionDays, "no tenant override → registry default via settings.ResolveInt")
}

func TestResolveRetentionDays_ResolveIntReturnsZero_FallsThroughToLiteral(t *testing.T) {
	t.Parallel()

	// HasTenantOverride = true but ResolveInt returns 0 (simulating a bad
	// stored value or misconfigured registry default). The `v > 0` guard
	// rejects it and the function falls through to the literal default 365.
	f, _ := setupFixture(t)
	settings := newStubSettingsService(true, nil, 0, nil)
	svc := buildSvc(f.db, settings)

	preview, err := svc.PreviewExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 365, preview.RetentionDays, "non-positive resolved value falls through to literal default")
}

// Sanity check — the registered key constant is what the service reads.
func TestResolveRetentionDays_UsesCorrectConfigKey(t *testing.T) {
	t.Parallel()

	// Documents the expected key: a regression test would catch someone
	// renaming KeyGDPRTimetableRetentionDays without updating the service.
	assert.Equal(t, "gdpr.timetable_retention_days", configModels.KeyGDPRTimetableRetentionDays)
}

// -----------------------------------------------------------------------------
// writeStudentAuditRows — nil audit repo must error before any DB write.
// -----------------------------------------------------------------------------

func TestCleanup_NilAuditRepo_ReturnsError(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)

	// Seed one affected student so writeStudentAuditRows has work to do.
	old := timezone.TodayDate().AddDays(-400)
	instID := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)
	stud := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Nil", "AuditRepo", "3a")
	f.studentIDs = append(f.studentIDs, stud.ID)
	f.attachStudent(t, instID, stud.ID, nil)

	// Build service with nil audit repo.
	svc := scheduleSvc.NewTimetableCleanupService(
		scheduleRepoPkg.NewActivityInstanceRepository(f.db),
		scheduleRepoPkg.NewActivityExceptionRepository(f.db),
		testInstanceStudents(f.db),
		nil,
		nil,
		nil,
		slog.Default(),
	)

	_, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audit repo not configured")

	// The deletes never ran because the audit step failed first.
	assertRowExists(t, f, "schedule.activity_instances", instID, true)
}

// -----------------------------------------------------------------------------
// NewTimetableCleanupService — nil logger substitution path.
// -----------------------------------------------------------------------------

func TestNewTimetableCleanupService_NilLogger_FallsBackToDefault(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	// Pass nil logger — constructor must substitute slog.Default() so calls
	// inside the service do not panic.
	svc := scheduleSvc.NewTimetableCleanupService(
		scheduleRepoPkg.NewActivityInstanceRepository(db),
		scheduleRepoPkg.NewActivityExceptionRepository(db),
		testInstanceStudents(db),
		auditRepoPkg.NewDataDeletionRepository(auditRepoPkg.NewRuntime(db, auditModels.TenantIDFromContext)),
		nil,
		nil,
		nil,
	)
	require.NotNil(t, svc)

	// Exercise a codepath that uses the logger. A unique tenant keeps the
	// aggregate cleanup query isolated from other tests.
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	_, err := svc.CleanupExpiredTimetableData(ctx)
	// Real DB under an empty tenant → cleanup succeeds with zeros. The
	// critical assertion is "no panic".
	require.NoError(t, err)
}

// -----------------------------------------------------------------------------
// Metadata shape — ensure writeStudentAuditRows attaches the sample when there
// are enough instance IDs (exercises the `len(sample) > 0` branch in the
// already-partially-covered helper).
// -----------------------------------------------------------------------------

func TestCleanup_AuditMetadata_HasInstanceIDsSample(t *testing.T) {
	t.Parallel()

	f, roomID := setupFixture(t)
	svc := buildSvc(f.db, nil)

	old := timezone.TodayDate().AddDays(-400)
	inst := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)

	stud := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Sample", "Student", "3a")
	f.studentIDs = append(f.studentIDs, stud.ID)
	f.attachStudent(t, inst, stud.ID, nil)

	_, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)

	var row auditModels.DataDeletion
	err = f.db.NewSelect().Model(&row).
		Where("tenant_id = ? AND student_id = ? AND deletion_type = ?",
			f.tenantID, stud.ID, auditModels.DeletionTypeTimetableRetention).
		Scan(f.ctx)
	require.NoError(t, err)
	assert.Contains(t, row.Metadata, "instance_ids_sample")

	// The rows were deleted by the service; tell fixture tracker to skip them.
	f.instanceIDs = nil
	f.instStudentIDs = nil
}
