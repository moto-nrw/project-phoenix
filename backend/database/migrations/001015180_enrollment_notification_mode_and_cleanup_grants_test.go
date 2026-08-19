package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func insertNotificationModeTestPhase(t *testing.T, db *bun.DB, tenantID int64, suffix string) int64 {
	t.Helper()
	var phaseID int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO enrollment.phases
			(tenant_id, name, service_start_date, service_end_date, is_active)
		VALUES (?, ?, CURRENT_DATE, CURRENT_DATE + 180, TRUE)
		RETURNING id
	`, tenantID, "notification-mode-migration-"+suffix).Scan(&phaseID)
	require.NoError(t, err)
	return phaseID
}

func insertNotificationModeTestRequest(t *testing.T, db *bun.DB, tenantID, phaseID int64, token, childStatus string) int64 {
	t.Helper()
	var requestID int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name,
			 guardian_email, consent_flags, custom_data, status_token, submitted_at)
		VALUES (?, ?, 'Migration', 'Test', ?, '{}'::jsonb, '{}'::jsonb, ?, NOW())
		RETURNING id
	`, tenantID, phaseID, token+"@example.test", token).Scan(&requestID)
	require.NoError(t, err)

	if childStatus != "" {
		_, err = db.ExecContext(context.Background(), `
			INSERT INTO enrollment.request_children
				(tenant_id, request_id, first_name, last_name, date_of_birth,
				 status, activation_mode, sort_order, custom_data)
			VALUES (?, ?, 'Kind', 'Test', '2018-04-15', ?, 'scheduled', 0, '{}'::jsonb)
		`, tenantID, requestID, childStatus)
		require.NoError(t, err)
	}
	return requestID
}

func insertNotificationModeTestOutbox(t *testing.T, db *bun.DB, tenantID, requestID int64, kind string, createdAt time.Time) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO platform.email_outbox
			(tenant_id, kind, related_entity_type, related_entity_id, payload,
			 status, attempts, next_retry_at, created_at)
		VALUES (?, ?, 'enrollment_request', ?, '{}'::jsonb, 'pending', 0, NOW(), ?)
	`, tenantID, kind, requestID, createdAt)
	require.NoError(t, err)
}

func notificationModeColumnExists(t *testing.T, db *bun.DB) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'enrollment'
			  AND table_name = 'requests'
			  AND column_name = 'decision_notification_mode'
		)
	`).Scan(context.Background(), &exists))
	return exists
}

func roleHasDeletePrivilege(t *testing.T, db *bun.DB, relation string) bool {
	t.Helper()
	var hasPrivilege bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', ?, 'DELETE')`, relation).
		Scan(context.Background(), &hasPrivilege))
	return hasPrivilege
}

func TestEnrollmentNotificationModeAndCleanupGrantsMigration_PostSchemaAndScopedBackfill(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	assert.True(t, notificationModeColumnExists(t, db))
	assert.True(t, roleHasDeletePrivilege(t, db, "platform.email_outbox"))
	assert.True(t, roleHasDeletePrivilege(t, db, "enrollment.late_invites"))

	historyTenant := testpkg.UniqueTestTenantID(t)
	changedSettingTenant := testpkg.UniqueTestTenantID(t)
	invalidSettingTenant := testpkg.UniqueTestTenantID(t)
	defaultTenant := testpkg.UniqueTestTenantID(t)
	tenantIDs := []int64{historyTenant, changedSettingTenant, invalidSettingTenant, defaultTenant}
	for _, tenantID := range tenantIDs {
		testpkg.EnsureTestTenant(t, db, tenantID)
	}
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("platform.schools").Where("id IN (?)", bun.List(tenantIDs)).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("platform.organizations").Where("id IN (?)", bun.List(tenantIDs)).Exec(context.Background())
	})

	historyPhase := insertNotificationModeTestPhase(t, db, historyTenant, "history")
	changedSettingPhase := insertNotificationModeTestPhase(t, db, changedSettingTenant, "changed-setting")
	invalidSettingPhase := insertNotificationModeTestPhase(t, db, invalidSettingTenant, "invalid-setting")
	defaultPhase := insertNotificationModeTestPhase(t, db, defaultTenant, "default")

	historyRequest := insertNotificationModeTestRequest(t, db, historyTenant, historyPhase, fmt.Sprintf("history-%d", historyTenant), "approved")
	reopenedHistoryRequest := insertNotificationModeTestRequest(t, db, historyTenant, historyPhase, fmt.Sprintf("reopened-history-%d", historyTenant), "under_review")
	changedSettingRequest := insertNotificationModeTestRequest(t, db, changedSettingTenant, changedSettingPhase, fmt.Sprintf("changed-setting-%d", changedSettingTenant), "waitlisted")
	undecidedRequest := insertNotificationModeTestRequest(t, db, changedSettingTenant, changedSettingPhase, fmt.Sprintf("undecided-%d", changedSettingTenant), "submitted")
	invalidSettingRequest := insertNotificationModeTestRequest(t, db, invalidSettingTenant, invalidSettingPhase, fmt.Sprintf("invalid-%d", invalidSettingTenant), "rejected")
	defaultRequest := insertNotificationModeTestRequest(t, db, defaultTenant, defaultPhase, fmt.Sprintf("default-%d", defaultTenant), "approved")

	now := time.Now()
	insertNotificationModeTestOutbox(t, db, historyTenant, historyRequest, "enrollment_approved", now.Add(-2*time.Hour))
	insertNotificationModeTestOutbox(t, db, historyTenant, historyRequest, "enrollment_decision_digest", now.Add(-time.Hour))
	insertNotificationModeTestOutbox(t, db, historyTenant, reopenedHistoryRequest, "enrollment_decision_digest", now.Add(-time.Hour))
	_, err := db.ExecContext(ctx, `
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES
			(?, 'enrollment.notify_per_decision', '"immediate"'::jsonb),
			(?, 'enrollment.notify_per_decision', '"invalid"'::jsonb)
	`, changedSettingTenant, invalidSettingTenant)
	require.NoError(t, err)

	require.NoError(t, backfillEnrollmentDecisionNotificationModes(
		ctx,
		db,
		"r.tenant_id IN (?)",
		bun.List(tenantIDs),
	))

	tests := []struct {
		name      string
		requestID int64
		want      sql.NullString
	}{
		{name: "earliest outbox history wins", requestID: historyRequest, want: sql.NullString{String: "immediate", Valid: true}},
		{name: "reopened request retains prior outbox mode", requestID: reopenedHistoryRequest, want: sql.NullString{String: "digest", Valid: true}},
		{name: "missing history stays digest after live setting changed", requestID: changedSettingRequest, want: sql.NullString{String: "digest", Valid: true}},
		{name: "undecided stays unpinned", requestID: undecidedRequest},
		{name: "missing history ignores invalid live setting", requestID: invalidSettingRequest, want: sql.NullString{String: "digest", Valid: true}},
		{name: "missing history defaults to digest", requestID: defaultRequest, want: sql.NullString{String: "digest", Valid: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got sql.NullString
			require.NoError(t, db.NewRaw(`
				SELECT decision_notification_mode
				FROM enrollment.requests
				WHERE id = ?
			`, tt.requestID).Scan(ctx, &got))
			assert.Equal(t, tt.want, got)
		})
	}

	_, err = db.ExecContext(ctx, `
		UPDATE enrollment.requests
		SET decision_notification_mode = 'batch'
		WHERE id = ?
	`, historyRequest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_enrollment_requests_decision_notification_mode")
}

func TestEnrollmentNotificationModeAndCleanupGrantsMigration_DownPreservesInheritedLateInviteDelete(t *testing.T) {
	source, err := os.ReadFile("001015180_enrollment_notification_mode_and_cleanup_grants.go")
	require.NoError(t, err)
	assert.Contains(t, string(source), "REVOKE DELETE ON platform.email_outbox FROM phoenix_tenant")
	assert.NotContains(t, string(source), "REVOKE DELETE ON enrollment.late_invites FROM phoenix_tenant")
}
