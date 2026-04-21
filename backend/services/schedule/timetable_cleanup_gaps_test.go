package schedule_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	auditRepoPkg "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// --- SettingsService stub ---

// stubSettingsService is a minimal configSvc.SettingsService that lets tests
// exercise the resolveRetentionDays branches without a real settings DB.
type stubSettingsService struct {
	hasOverride    bool
	hasOverrideErr error
	intVal         int
	intErr         error
}

func (s *stubSettingsService) GetSchema(context.Context, []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (s *stubSettingsService) GetSchemaForOperator(context.Context, []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (s *stubSettingsService) Resolve(context.Context, string) (any, error) { return nil, nil }
func (s *stubSettingsService) ResolveString(context.Context, string) (string, error) {
	return "", nil
}
func (s *stubSettingsService) ResolveStringForTenant(context.Context, int64, string) (string, error) {
	return "", nil
}
func (s *stubSettingsService) ResolveBool(context.Context, string) (bool, error) { return false, nil }
func (s *stubSettingsService) ResolveInt(_ context.Context, _ string) (int, error) {
	return s.intVal, s.intErr
}
func (s *stubSettingsService) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return s.hasOverride, s.hasOverrideErr
}
func (s *stubSettingsService) SetValue(context.Context, string, any, *int64, []string) error {
	return nil
}
func (s *stubSettingsService) ResetValue(context.Context, string, *int64, []string) error {
	return nil
}
func (s *stubSettingsService) GetLoginImageURL(context.Context, int64) (string, error) {
	return "", nil
}
func (s *stubSettingsService) SetLoginImageURL(context.Context, int64, string) (string, error) {
	return "", nil
}
func (s *stubSettingsService) ClearLoginImageURL(context.Context, int64) (string, error) {
	return "", nil
}

// buildSvc wires a TimetableCleanupService with the given settings stub.
func buildSvc(db *bun.DB, settings configSvc.SettingsService) scheduleSvc.TimetableCleanupService {
	return scheduleSvc.NewTimetableCleanupService(
		db,
		auditRepoPkg.NewDataDeletionRepository(db),
		settings,
		slog.Default(),
	)
}

// -----------------------------------------------------------------------------
// Missing-tenant-context guards for all three public methods.
// -----------------------------------------------------------------------------

func TestCleanupExpiredTimetableData_NoTenantInContext_ReturnsError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	svc := buildSvc(db, nil)

	_, err := svc.CleanupExpiredTimetableData(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant in context")
}

func TestPreviewExpiredTimetableData_NoTenantInContext_ReturnsError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	svc := buildSvc(db, nil)

	_, err := svc.PreviewExpiredTimetableData(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant in context")
}

func TestGetStats_NoTenantInContext_ReturnsError(t *testing.T) {
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
	f, roomID := setupFixture(t)
	svc := buildSvc(f.db, nil)

	old := time.Now().UTC().AddDate(0, 0, -400)
	recent := time.Now().UTC().AddDate(0, 0, -30)

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
	f, roomID := setupFixture(t)
	svc := buildSvc(f.db, nil)

	// Seed: 2 instances (1 old, 1 recent), both count toward totals.
	old := time.Now().UTC().AddDate(0, 0, -400)
	recent := time.Now().UTC().AddDate(0, 0, -30)
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
// env-var / nil-settings tests.
// -----------------------------------------------------------------------------

func TestResolveRetentionDays_TenantOverride_UsesOverriddenValue(t *testing.T) {
	// HasTenantOverride = true, ResolveInt returns a positive value → use it.
	f, roomID := setupFixture(t)
	settings := &stubSettingsService{hasOverride: true, intVal: 42}
	svc := buildSvc(f.db, settings)

	// Nothing old in this tenant → instances=0, but retention must reflect override.
	// Insert a 50-day-old instance to verify: 42-day window deletes it, 400-day
	// registry default would too but 42 is specific.
	fiftyDaysOld := time.Now().UTC().AddDate(0, 0, -50)
	f.newInstance(t, fiftyDaysOld, scheduleModels.InstanceStatusCompleted, roomID, nil)

	result, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 42, result.RetentionDays, "tenant override must win over env/default")
	assert.Equal(t, 1, result.InstancesDeleted, "50-day row past 42-day window")
}

func TestResolveRetentionDays_HasOverrideError_FallsBackToDefault(t *testing.T) {
	// HasTenantOverride returns err — Warn-log, skip settings.ResolveInt, fall
	// through to env var (unset) and finally the final `if s.settings != nil`
	// block which calls ResolveInt without the gate.
	f, roomID := setupFixture(t)
	// Final `if s.settings != nil` path: ResolveInt returns 365 (simulates
	// registry default). Even though HasTenantOverride errors, resolveInt is
	// still consulted as a last-ditch resort in the post-env-var block.
	settings := &stubSettingsService{
		hasOverrideErr: errors.New("settings DB down"),
		intVal:         365,
	}
	svc := buildSvc(f.db, settings)

	f.newInstance(t, time.Now().UTC().AddDate(0, 0, -400), scheduleModels.InstanceStatusCompleted, roomID, nil)

	result, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 365, result.RetentionDays, "override-check error must fall through to settings default")
}

func TestResolveRetentionDays_NoOverrideNoEnv_UsesRegistryDefault(t *testing.T) {
	// HasTenantOverride = false, no env var → the final `if s.settings != nil`
	// block returns the registry default via ResolveInt.
	f, _ := setupFixture(t)
	settings := &stubSettingsService{hasOverride: false, intVal: 180}
	svc := buildSvc(f.db, settings)

	// Call Preview to inspect RetentionDays without having to seed data.
	preview, err := svc.PreviewExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 180, preview.RetentionDays, "no tenant override + no env var → registry default via settings.ResolveInt")
}

func TestResolveRetentionDays_HasOverrideTrue_InvalidValue_FallsThrough(t *testing.T) {
	// HasTenantOverride = true but ResolveInt returns 0 (simulating a bad
	// stored value). The service should skip the override and fall through to
	// the env/default chain — we use the final `if s.settings != nil` block
	// with intVal > 0 after the bad branch.
	//
	// This specifically covers the `v > 0` guard inside the HasTenantOverride
	// branch (line 331 in the source).
	f, _ := setupFixture(t)
	settings := &stubSettingsService{hasOverride: true, intVal: 0}
	svc := buildSvc(f.db, settings)

	// Override resolves to 0 → both inner guards fail → final `if s.settings != nil`
	// block is reached. intVal is still 0, so the literal `timetableRetentionDefaultDays`
	// (365) is the final value.
	preview, err := svc.PreviewExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 365, preview.RetentionDays, "override with non-positive value falls through to literal default")
}

// Sanity check — the registered key constant is what the service reads.
func TestResolveRetentionDays_UsesCorrectConfigKey(t *testing.T) {
	// Documents the expected key: a regression test would catch someone
	// renaming KeyGDPRTimetableRetentionDays without updating the service.
	assert.Equal(t, "gdpr.timetable_retention_days", configModels.KeyGDPRTimetableRetentionDays)
}

// -----------------------------------------------------------------------------
// writeStudentAuditRows — nil audit repo must error before any DB write.
// -----------------------------------------------------------------------------

func TestCleanup_NilAuditRepo_ReturnsError(t *testing.T) {
	f, roomID := setupFixture(t)

	// Seed one affected student so writeStudentAuditRows has work to do.
	old := time.Now().UTC().AddDate(0, 0, -400)
	instID := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)
	stud := testpkg.CreateTestStudent(t, f.db, "Nil", "AuditRepo", "3a")
	f.studentIDs = append(f.studentIDs, stud.ID)
	f.attachStudent(t, instID, stud.ID, nil)

	// Build service with nil audit repo.
	svc := scheduleSvc.NewTimetableCleanupService(f.db, nil, nil, slog.Default())

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
	db := testpkg.SetupTestDB(t)

	// Pass nil logger — constructor must substitute slog.Default() so calls
	// inside the service do not panic.
	svc := scheduleSvc.NewTimetableCleanupService(
		db,
		auditRepoPkg.NewDataDeletionRepository(db),
		nil,
		nil,
	)
	require.NotNil(t, svc)

	// Exercise a codepath that uses the logger (no tenant → early error return
	// avoids logger use, so we use a tenant ctx that makes it into slog.Info).
	ctx := tenant.WithTenantID(context.Background(), 1)
	_, err := svc.CleanupExpiredTimetableData(ctx)
	// Real DB under tenant_id=1 with no seeded rows → cleanup succeeds with
	// zeros. The critical assertion is "no panic".
	require.NoError(t, err)
}

// -----------------------------------------------------------------------------
// Metadata shape — ensure writeStudentAuditRows attaches the sample when there
// are enough instance IDs (exercises the `len(sample) > 0` branch in the
// already-partially-covered helper).
// -----------------------------------------------------------------------------

func TestCleanup_AuditMetadata_HasInstanceIDsSample(t *testing.T) {
	f, roomID := setupFixture(t)
	svc := buildSvc(f.db, nil)

	old := time.Now().UTC().AddDate(0, 0, -400)
	inst := f.newInstance(t, old, scheduleModels.InstanceStatusCompleted, roomID, nil)

	stud := testpkg.CreateTestStudent(t, f.db, "Sample", "Student", "3a")
	f.studentIDs = append(f.studentIDs, stud.ID)
	f.attachStudent(t, inst, stud.ID, nil)

	_, err := svc.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)

	var row auditModels.DataDeletion
	err = f.db.NewSelect().Model(&row).
		Where("tenant_id = ? AND student_id = ? AND deletion_type = ?",
			testTenantID, stud.ID, auditModels.DeletionTypeTimetableRetention).
		Scan(f.ctx)
	require.NoError(t, err)
	assert.Contains(t, row.Metadata, "instance_ids_sample")

	// The rows were deleted by the service; tell fixture tracker to skip them.
	f.instanceIDs = nil
	f.instStudentIDs = nil
}
