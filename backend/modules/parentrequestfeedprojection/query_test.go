package parentrequestfeedprojection

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestListNewParentRequestsReturnsOnlyRequestsAwaitingReview(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)

	type expectedEntry struct {
		Kind string `bun:"kind"`
		ID   int64  `bun:"id"`
	}
	expected := []expectedEntry{}
	require.NoError(t, db.NewRaw(`
		WITH enrollment_request AS (
			INSERT INTO enrollment.requests
				(tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token)
			VALUES (?, ?, 'Feed', 'Guardian', ?, ?)
			RETURNING id
		), request_child AS (
			INSERT INTO enrollment.request_children
				(tenant_id, request_id, first_name, last_name, date_of_birth, status, created_student_id)
			SELECT ?, id, 'Feed', 'Child', CURRENT_DATE - 2500, 'approved', ?
			FROM enrollment_request
			RETURNING id, request_id
		), master_data AS (
			INSERT INTO users.student_data_change_requests
				(tenant_id, student_id, submitted_by, target, field_key, new_value, status)
			VALUES
				(?, ?, ?, 'student', 'school_class', '"2a"'::jsonb, 'pending'),
				(?, ?, ?, 'student', 'school_class', '"2b"'::jsonb, 'approved')
			RETURNING id, status
		), care_schedule AS (
			INSERT INTO schedule.care_schedule_change_requests
				(tenant_id, student_id, submitted_by, payload, status)
			VALUES
				(?, ?, ?, '{}'::jsonb, 'pending'),
				(?, ?, ?, '{}'::jsonb, 'rejected')
			RETURNING id, status
		), offering AS (
			INSERT INTO enrollment.offering_change_requests
				(tenant_id, student_id, request_child_id, submitted_by, payload, effective_from, status)
			SELECT ?, ?, id, ?, '{"offerings":[]}'::jsonb, CURRENT_DATE, status
			FROM request_child
			CROSS JOIN (VALUES ('pending'), ('withdrawn')) AS statuses(status)
			RETURNING id, status
		), excused_absence AS (
			INSERT INTO active.excused_absence_requests
				(tenant_id, student_id, submitted_by, dates, note, status, absence_status)
			VALUES
				(?, ?, ?, to_jsonb(ARRAY[CURRENT_DATE::TEXT]), 'Feed test', 'pending', 'excused'),
				(?, ?, ?, to_jsonb(ARRAY[CURRENT_DATE::TEXT]), 'Feed test', 'done', 'excused')
			RETURNING id, status
		), enrollment_change AS (
			INSERT INTO enrollment.change_requests
				(tenant_id, request_id, request_child_id, origin, status, base_snapshot, proposed_snapshot, diff_json)
			SELECT ?, request_id, id, 'parent', status, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb
			FROM request_child
			CROSS JOIN (VALUES ('pending_review'), ('approved')) AS statuses(status)
			RETURNING id, status
		)
		SELECT 'master_data'::text AS kind, id FROM master_data WHERE status = 'pending'
		UNION ALL SELECT 'care_schedule', id FROM care_schedule WHERE status = 'pending'
		UNION ALL SELECT 'offering', id FROM offering WHERE status = 'pending'
		UNION ALL SELECT 'excused_absence', id FROM excused_absence WHERE status = 'pending'
		UNION ALL SELECT 'enrollment', id FROM enrollment_change WHERE status = 'pending_review'
	`,
		chain.TenantID, phase.ID, chain.Email, "feed-"+chain.Email,
		chain.TenantID, chain.StudentID,
		chain.TenantID, chain.StudentID, chain.AccountID,
		chain.TenantID, chain.StudentID, chain.AccountID,
		chain.TenantID, chain.StudentID, chain.AccountID,
		chain.TenantID, chain.StudentID, chain.AccountID,
		chain.TenantID, chain.StudentID, chain.AccountID,
		chain.TenantID, chain.StudentID, chain.AccountID,
		chain.TenantID, chain.StudentID, chain.AccountID,
		chain.TenantID,
	).Scan(testpkg.Ctx(t), &expected))

	var entries []Entry
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, chain.TenantID, func(ctx context.Context, tx bun.Tx) error {
		var listErr error
		entries, listErr = ListNewParentRequests(ctx, tx, chain.TenantID, time.Now().Add(-time.Hour), true, true)
		return listErr
	})
	require.NoError(t, err)

	actual := make([]expectedEntry, 0, len(entries))
	for _, entry := range entries {
		actual = append(actual, expectedEntry{Kind: entry.Kind, ID: entry.ID})
	}
	require.ElementsMatch(t, expected, actual)
}

func TestResolveAccessRequiresSchoolWideRightsAndActiveMembership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "request-feed-access@example.test")
	tenantID := testpkg.Tenant(t)

	var access Access
	var found bool
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		var resolveErr error
		access, found, resolveErr = ResolveAccess(ctx, tx, tenantID, account.ID)
		return resolveErr
	})
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, access.GeneralRequests, "a plain account must not get a school-wide feed")
	require.False(t, access.EnrollmentRequests)

	result, err := db.ExecContext(testpkg.Ctx(t), `
		INSERT INTO auth.account_permissions (tenant_id, account_id, permission_id, granted)
		SELECT ?, ?, id, TRUE
		FROM auth.permissions
		WHERE name = 'config:manage'
	`, tenantID, account.ID)
	require.NoError(t, err)
	rows, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, rows, "the seeded config:manage permission must exist")

	err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		var resolveErr error
		access, found, resolveErr = ResolveAccess(ctx, tx, tenantID, account.ID)
		return resolveErr
	})
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, access.GeneralRequests)
	require.True(t, access.EnrollmentRequests, "enrollment managers receive only enrollment requests")

	adminRole := testpkg.GetOrCreateTestRole(t, db, "admin")
	_, err = db.ExecContext(testpkg.Ctx(t), `
		INSERT INTO auth.account_roles (tenant_id, account_id, role_id)
		VALUES (?, ?, ?)
	`, tenantID, account.ID, adminRole.ID)
	require.NoError(t, err)

	err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		var resolveErr error
		access, found, resolveErr = ResolveAccess(ctx, tx, tenantID, account.ID)
		return resolveErr
	})
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, access.GeneralRequests)
	require.NotEmpty(t, access.SchoolName)
	require.NotEmpty(t, access.Subdomain)

	_, err = db.ExecContext(testpkg.Ctx(t), `
		UPDATE auth.account_tenants
		SET status = 'inactive', deactivated_at = NOW()
		WHERE tenant_id = ? AND account_id = ?
	`, tenantID, account.ID)
	require.NoError(t, err)

	err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		_, found, err = ResolveAccess(ctx, tx, tenantID, account.ID)
		return err
	})
	require.NoError(t, err)
	require.False(t, found)
}

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
