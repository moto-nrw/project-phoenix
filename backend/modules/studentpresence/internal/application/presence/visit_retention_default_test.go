package presence_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	activeService "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// privacyConsentRetentionKey is models/config.KeyPrivacyConsentRetentionDays,
// spelled out like the other setting overrides in this package.
const privacyConsentRetentionKey = "gdpr.privacy_consent_retention_days"

func setVisitRetentionOverride(t *testing.T, db *testpkg.DB, days int) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		VALUES (?, ?, ?::jsonb, NULL)
		ON CONFLICT (tenant_id, setting_key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, testpkg.Tenant(t), privacyConsentRetentionKey, strconv.Itoa(days)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

func insertConsent(t *testing.T, db *testpkg.DB, studentID int64, accepted bool, retentionDays int) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO users.privacy_consents (student_id, policy_version, accepted, renewal_required, data_retention_days, tenant_id, created_at, updated_at)
		VALUES (?, 'v1.0', ?, false, ?, ?, NOW(), NOW())
	`, studentID, accepted, retentionDays, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

// insertRetentionVisit stores a visit created daysAgo days back; completed
// visits end an hour after they began, open ones have no exit time.
func insertRetentionVisit(t *testing.T, db *testpkg.DB, studentID, activeGroupID int64, daysAgo int, completed bool) int64 {
	t.Helper()
	createdAt := time.Now().AddDate(0, 0, -daysAgo)
	var exit *time.Time
	if completed {
		value := createdAt.Add(time.Hour)
		exit = &value
	}
	var id int64
	err := db.NewRaw(`
		INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at, tenant_id)
		VALUES (?, ?, ?, ?, ?, NOW(), ?)
		RETURNING id
	`, studentID, activeGroupID, createdAt, exit, createdAt, testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &id)
	require.NoError(t, err)
	return id
}

func visitExists(t *testing.T, db *testpkg.DB, visitID int64) bool {
	t.Helper()
	exists, err := db.NewSelect().Table("active.visits").Where("id = ?", visitID).Exists(testpkg.Ctx(t))
	require.NoError(t, err)
	return exists
}

type visitRetentionAudit struct {
	RecordsDeleted int    `bun:"records_deleted"`
	Metadata       []byte `bun:"metadata"`
}

func visitRetentionAudits(t *testing.T, db *testpkg.DB, studentID int64) []visitRetentionAudit {
	t.Helper()
	var rows []visitRetentionAudit
	err := db.NewSelect().Table("audit.data_deletions").Column("records_deleted", "metadata").
		Where("student_id = ?", studentID).Where("deletion_type = 'visit_retention'").
		Scan(testpkg.Ctx(t), &rows)
	require.NoError(t, err)
	return rows
}

func auditRetentionDays(t *testing.T, audit visitRetentionAudit) int {
	t.Helper()
	var metadata struct {
		RetentionDays int `json:"retention_days"`
	}
	require.NoError(t, json.Unmarshal(audit.Metadata, &metadata))
	return metadata.RetentionDays
}

// TestCleanupExpiredVisits_DeletesVisitsOfChildWithoutConsent is the #3835
// regression: a child without any privacy_consents row had its completed
// visits kept forever, because the cleanup only walked accepted consents.
// Without a tenant override the registry default (30 days) applies.
func TestCleanupExpiredVisits_DeletesVisitsOfChildWithoutConsent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	cleanup := setupCleanupService(t, db)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Ohne", "Einwilligung", "3a")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	expired := insertRetentionVisit(t, db, student.ID, group.ID, 45, true)
	recent := insertRetentionVisit(t, db, student.ID, group.ID, 10, true)
	open := insertRetentionVisit(t, db, student.ID, group.ID, 45, false)

	preview, err := testpkg.TenantQuery0(t, ctx, db, cleanup.PreviewCleanup)
	require.NoError(t, err)
	assert.Equal(t, 1, preview.StudentVisitCounts[student.ID], "the preview must count the child without consent")
	assert.EqualValues(t, 1, preview.TotalVisits)

	stats, err := testpkg.TenantQuery0(t, ctx, db, cleanup.GetRetentionStatistics)
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.TotalExpiredVisits)
	assert.Equal(t, 1, stats.StudentsAffected)

	result, err := cleanup.CleanupExpiredVisits(ctx)
	require.NoError(t, err)
	require.True(t, result.Success, "errors: %v", result.Errors)
	assert.EqualValues(t, 1, result.RecordsDeleted)

	assert.False(t, visitExists(t, db, expired), "a completed visit past the default window must be deleted")
	assert.True(t, visitExists(t, db, recent), "a visit inside the default window must stay")
	assert.True(t, visitExists(t, db, open), "an open visit must stay regardless of its age")

	audits := visitRetentionAudits(t, db, student.ID)
	require.Len(t, audits, 1)
	assert.Equal(t, 1, audits[0].RecordsDeleted)
	assert.Equal(t, 30, auditRetentionDays(t, audits[0]))
}

// TestCleanupExpiredVisits_TenantSettingAppliesWithoutAcceptedConsent pins
// which window applies: an accepted consent's own window, otherwise the
// tenant's gdpr.privacy_consent_retention_days. A declined consent grants no
// window of its own.
func TestCleanupExpiredVisits_TenantSettingAppliesWithoutAcceptedConsent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	cleanup := setupCleanupService(t, db)
	ctx := testpkg.Ctx(t)
	setVisitRetentionOverride(t, db, 7)

	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	withoutConsent := testpkg.CreateTestStudent(t, db, "Ohne", "Datensatz", "3b")
	declined := testpkg.CreateTestStudent(t, db, "Abgelehnt", "Einwilligung", "3b")
	insertConsent(t, db, declined.ID, false, 31)
	accepted := testpkg.CreateTestStudent(t, db, "Erteilt", "Einwilligung", "3b")
	insertConsent(t, db, accepted.ID, true, 31)
	insertConsent(t, db, accepted.ID, true, 20)

	withoutConsentVisit := insertRetentionVisit(t, db, withoutConsent.ID, group.ID, 10, true)
	withoutConsentRecent := insertRetentionVisit(t, db, withoutConsent.ID, group.ID, 3, true)
	declinedVisit := insertRetentionVisit(t, db, declined.ID, group.ID, 10, true)
	acceptedInside := insertRetentionVisit(t, db, accepted.ID, group.ID, 10, true)
	acceptedPast := insertRetentionVisit(t, db, accepted.ID, group.ID, 25, true)

	result, err := testpkg.TenantQuery0(t, ctx, db, cleanup.CleanupExpiredVisits)
	require.NoError(t, err)
	require.True(t, result.Success, "errors: %v", result.Errors)
	assert.EqualValues(t, 3, result.RecordsDeleted)

	assert.False(t, visitExists(t, db, withoutConsentVisit), "the tenant window (7 days) applies without a consent row")
	assert.True(t, visitExists(t, db, withoutConsentRecent))
	assert.False(t, visitExists(t, db, declinedVisit), "a declined consent falls back to the tenant window")
	assert.True(t, visitExists(t, db, acceptedInside), "an accepted consent's window wins over the tenant window")
	assert.False(t, visitExists(t, db, acceptedPast), "the shortest accepted window applies")

	for _, tc := range []struct {
		studentID int64
		days      int
	}{{withoutConsent.ID, 7}, {declined.ID, 7}, {accepted.ID, 20}} {
		audits := visitRetentionAudits(t, db, tc.studentID)
		require.Len(t, audits, 1, "student %d", tc.studentID)
		assert.Equal(t, 1, audits[0].RecordsDeleted)
		assert.Equal(t, tc.days, auditRetentionDays(t, audits[0]))
	}
}

// TestCleanupExpiredVisits_OverrideEqualToDefaultIsUsed keeps an explicit
// override authoritative even when it equals the registry default.
func TestCleanupExpiredVisits_OverrideEqualToDefaultIsUsed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	cleanup := setupCleanupService(t, db)
	setVisitRetentionOverride(t, db, 30)

	student := testpkg.CreateTestStudent(t, db, "Gleich", "Standard", "3c")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	expired := insertRetentionVisit(t, db, student.ID, group.ID, 31, true)
	kept := insertRetentionVisit(t, db, student.ID, group.ID, 29, true)

	result, err := testpkg.TenantQuery0(t, testpkg.Ctx(t), db, cleanup.CleanupExpiredVisits)
	require.NoError(t, err)
	require.True(t, result.Success, "errors: %v", result.Errors)
	assert.False(t, visitExists(t, db, expired))
	assert.True(t, visitExists(t, db, kept))
}

type visitRetentionSetting struct {
	days int
	err  error
}

func (s visitRetentionSetting) VisitRetentionDays(context.Context) (int, error) {
	return s.days, s.err
}

// TestCleanupExpiredVisits_RefusesWithoutUsableTenantWindow fails the run
// instead of deleting with a guessed window.
func TestCleanupExpiredVisits_RefusesWithoutUsableTenantWindow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	presence := testSchoolPresence(t, db)
	student := testpkg.CreateTestStudent(t, db, "Fehler", "Einstellung", "3d")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	visit := insertRetentionVisit(t, db, student.ID, group.ID, 45, true)

	injected := errors.New("settings unavailable")
	for _, tc := range []struct {
		name    string
		setting activeService.VisitRetentionSettings
		cause   error
	}{
		{name: "resolution error", setting: visitRetentionSetting{err: injected}, cause: injected},
		{name: "zero window", setting: visitRetentionSetting{days: 0}},
		{name: "missing settings", setting: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := activeService.NewCleanupService(presence, nil, nil, tc.setting)
			result, err := svc.CleanupExpiredVisits(testpkg.Ctx(t))
			require.Error(t, err)
			if tc.cause != nil {
				require.ErrorIs(t, err, tc.cause)
			}
			require.NotNil(t, result)
			assert.False(t, result.Success)

			_, err = svc.PreviewCleanup(testpkg.Ctx(t))
			require.Error(t, err)
			_, err = svc.GetRetentionStatistics(testpkg.Ctx(t))
			require.Error(t, err)
		})
	}
	assert.True(t, visitExists(t, db, visit), "no visit may be deleted without a resolved window")
}
