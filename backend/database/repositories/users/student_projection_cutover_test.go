package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentAudience "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestCareExitWithoutStudentCompatibilityView(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Owner", "Care", "3b")
	ctx := tenantCtx(t)
	today := timezone.NewDate(2026, 9, 19)
	yesterday := today.AddDays(-1)
	testpkg.SetStudentLifecycle(t, db, student.ID, usersModels.StudentStatusActive, nil, &yesterday)
	membershipID := separateStudentMembership(t, db, student.ID)
	repos, err := repositories.NewCareLifecycleTestRepositories(db, nil)
	require.NoError(t, err)
	assertStudentCompatibilityStorageAbsent(t, db)

	rows, total, err := repos.CareExit.ListEnded(ctx, today, usersModels.CareExitListFilter{Search: "3b"})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, student.ID, rows[0].StudentID)
	facts, err := repos.CareExitCleanup.ListCareBookingFacts(ctx, today, []int64{student.ID})
	require.NoError(t, err)
	require.Len(t, facts, 1)
	require.Equal(t, student.ID, facts[0].StudentID)
	require.Equal(t, "3b", facts[0].SchoolClass)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		return repos.CareExitCleanup.LockImpactRowsForCareExit(txCtx, []int64{student.ID})
	}))
	assertStudentProjectionTenants(t, db, func(txCtx context.Context, visible bool) {
		rows, _, err := repos.CareExit.ListEnded(txCtx, today, usersModels.CareExitListFilter{})
		require.NoError(t, err)
		require.Equal(t, visible, len(rows) == 1)
	})

	assertMissingStudentProjectionStates(t, db, membershipID, func() {
		rows, total, err := repos.CareExit.ListEnded(ctx, today, usersModels.CareExitListFilter{})
		require.NoError(t, err)
		require.Zero(t, total)
		require.Empty(t, rows)
		facts, err := repos.CareExitCleanup.ListCareBookingFacts(ctx, today, []int64{student.ID})
		require.NoError(t, err)
		require.Empty(t, facts)
	})
}

func TestParentAudienceWithoutStudentCompatibilityView(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	ctx := tenantCtx(t)
	poll, _ := pollAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID, usersModels.ParentAnnouncementResponseSingleChoice)
	membershipID := separateStudentMembership(t, db, chain.StudentID)
	assertStudentCompatibilityStorageAbsent(t, db)

	count, err := repo.CountAudience(ctx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	feed, err := repo.ListFeedForAccount(ctx, chain.AccountID, usersModels.AnnouncementFeedScope{TenantIDs: []int64{chain.TenantID}})
	require.NoError(t, err)
	require.Len(t, feed, 1)
	children, err := repo.PollChildren(ctx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, chain.StudentID, children[0].StudentID)
	require.Equal(t, "1a", children[0].SchoolClass)
	may, err := repo.AccountMayAnswerForStudent(ctx, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.True(t, may)
	count, err = repo.CountReachableGuardiansForStudents(ctx, chain.TenantID, []int64{chain.StudentID})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	assertStudentProjectionTenants(t, db, func(txCtx context.Context, visible bool) {
		may, err := repo.AccountMayAnswerForStudent(txCtx, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		require.Equal(t, visible, may)
	})
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET permissions = '{}'::jsonb WHERE student_id = ?`, chain.StudentID)
	require.NoError(t, err)
	may, err = repo.AccountMayAnswerForStudent(ctx, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.False(t, may, "a relationship and active account are not parent authorization")
	// Restore the fixture's explicit permissions so the row-existence checks
	// below cannot pass merely because authorization was removed.
	_, err = db.ExecContext(ctx, `UPDATE users.students_guardians SET permissions = '{"parent_portal.access":true,"parent_portal.poll.response":true}'::jsonb WHERE student_id = ?`, chain.StudentID)
	require.NoError(t, err)

	assertMissingStudentProjectionStates(t, db, membershipID, func() {
		count, err := repo.CountAudience(ctx, chain.TenantID, poll.ID)
		require.NoError(t, err)
		require.Zero(t, count)
		children, err := repo.PollChildren(ctx, chain.TenantID, poll.ID)
		require.NoError(t, err)
		require.Empty(t, children)
		may, err := repo.AccountMayAnswerForStudent(ctx, chain.TenantID, poll.ID, chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		require.False(t, may)
	})
}

func TestParentInboxWithoutStudentCompatibilityView(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestAccount(t, db, "owner-inbox")
	repos := parentRepos(t, db)
	ctx := tenantCtx(t)
	thread := newThread(t, chain.StudentID, chain.AccountID)
	require.NoError(t, repos.Thread.Create(ctx, thread))
	message := newMessage(t, thread.ID, chain.StudentID, chain.AccountID, usersModels.ParentMessageSenderGuardian, "Owner read")
	require.NoError(t, repos.Message.Create(ctx, message))
	membershipID := separateStudentMembership(t, db, chain.StudentID)
	assertStudentCompatibilityStorageAbsent(t, db)

	rows, err := repos.Read.ListInboxForStaff(ctx, staff.ID, true, false)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, chain.StudentID, rows[0].StudentID)
	require.Equal(t, "1a", rows[0].SchoolClass)
	count, err := repos.Read.UnreadMessageCountForStaff(ctx, staff.ID, true)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	header, err := repos.Read.FindThreadHeader(ctx, thread.ID)
	require.NoError(t, err)
	require.NotNil(t, header)

	assertStudentProjectionTenants(t, db, func(txCtx context.Context, visible bool) {
		header, err := repos.Read.FindThreadHeader(txCtx, thread.ID)
		require.NoError(t, err)
		require.Equal(t, visible, header != nil)
	})

	assertMissingStudentProjectionStates(t, db, membershipID, func() {
		rows, err := repos.Read.ListInboxForStaff(ctx, staff.ID, true, false)
		require.NoError(t, err)
		require.Empty(t, rows)
		count, err := repos.Read.UnreadMessageCountForStaff(ctx, staff.ID, true)
		require.NoError(t, err)
		require.Zero(t, count)
		header, err := repos.Read.FindThreadHeader(ctx, thread.ID)
		require.NoError(t, err)
		require.Nil(t, header)
	})
}

func assertStudentProjectionTenants(t *testing.T, db *bun.DB, check func(context.Context, bool)) {
	t.Helper()
	foreign := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreign)
	for _, tenantID := range []int64{testpkg.Tenant(t), foreign} {
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			check(ctx, tenantID == testpkg.Tenant(t))
			return nil
		}))
	}
}

// Keep a historical membership and its care row, but give the live membership
// a sequence-generated ID distinct from the public student ID. This catches
// both an accidental ID equality join and a missing deleted_at predicate.
func separateStudentMembership(t *testing.T, db *bun.DB, studentID int64) int64 {
	t.Helper()
	ctx := tenantCtx(t)
	var oldID, nextID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_school_memberships
		WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`, testpkg.Tenant(t), studentID).Scan(ctx, &oldID))
	for nextID == 0 || nextID == studentID {
		require.NoError(t, db.NewRaw(`SELECT nextval('users.student_school_memberships_id_seq')`).Scan(ctx, &nextID))
	}
	_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE id = ?`, oldID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users.student_school_memberships
		(id, tenant_id, student_profile_id, school_class, group_id, status, enrolled_from, enrolled_until)
		SELECT ?, tenant_id, student_profile_id, school_class, group_id, status, enrolled_from, enrolled_until
		FROM users.student_school_memberships WHERE id = ?`, nextID, oldID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users.student_care_profiles (tenant_id, membership_id) VALUES (?, ?)`, testpkg.Tenant(t), nextID)
	require.NoError(t, err)
	require.NotEqual(t, studentID, nextID)
	return nextID
}

func assertStudentCompatibilityStorageAbsent(t *testing.T, db *bun.DB) {
	t.Helper()
	var absent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students') IS NULL
		AND to_regclass('users.students_legacy') IS NULL`).Scan(tenantCtx(t), &absent))
	require.True(t, absent, "current repositories must operate without rollback storage")
}

func assertMissingStudentProjectionStates(t *testing.T, db *bun.DB, membershipID int64, assertHidden func()) {
	t.Helper()
	ctx := tenantCtx(t)
	_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE id = ?`, membershipID)
	require.NoError(t, err)
	assertHidden()
	_, err = db.ExecContext(ctx, `UPDATE users.student_school_memberships SET deleted_at = NULL WHERE id = ?`, membershipID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM users.student_care_profiles WHERE membership_id = ?`, membershipID)
	require.NoError(t, err)
	assertHidden()
	_, err = db.ExecContext(ctx, `DELETE FROM users.student_school_memberships WHERE id = ?`, membershipID)
	require.NoError(t, err)
	assertHidden()
}
