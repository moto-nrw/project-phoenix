package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentAudience "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// SetResponse is the one write in the announcement feature that carries a
// hand-rolled concurrency guard: a transaction advisory lock plus a guard CTE
// that re-checks liveness, version, the guardian relationship and the selection
// on BOTH the delete and the insert. Until now nothing exercised it — the only
// mention of SetResponse in any test was a mock method — so every rule below
// held by inspection alone.
//
// These tests pin the rules that guard must keep, whatever shape it takes:
// only a guardian with parent_portal.poll.response may answer, only for a child
// the poll actually reaches, only against the version the client loaded, and
// only while the poll is open.

// pollAnnouncement publishes a poll with two options and returns it plus the
// option ids in display order.
func pollAnnouncement(
	t *testing.T,
	ctx context.Context,
	db *bun.DB,
	repo usersModels.ParentAnnouncementRepository,
	createdBy, tenantID int64,
	responseType string,
) (*usersModels.ParentAnnouncement, []int64) {
	t.Helper()
	a := &usersModels.ParentAnnouncement{
		Title:        "Umfrage",
		Body:         "Bitte antworten",
		Priority:     usersModels.ParentAnnouncementPriorityInfo,
		Active:       true,
		CreatedBy:    createdBy,
		ResponseType: responseType,
	}
	a.SetTenantID(tenantID)
	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.ReplaceTargets(ctx, tenantID, a.ID,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}}))
	require.NoError(t, repo.ReplaceOptions(ctx, tenantID, a.ID, []*usersModels.ParentAnnouncementOption{
		{Label: "Ja"}, {Label: "Nein"},
	}))
	options, err := repo.ListOptions(ctx, a.ID)
	require.NoError(t, err)
	require.Len(t, options, 2)
	now := databaseTimestamp(t, db)
	require.NoError(t, repo.SetPublished(ctx, a.ID, &now))
	a.PublishedAt = &now
	t.Cleanup(func() { _ = repo.Delete(ctx, a.ID) })
	return a, []int64{options[0].ID, options[1].ID}
}

// storedAnswers returns the option ids stored for one child, so a test asserts
// on the rows rather than on the reported outcome alone.
func storedAnswers(t *testing.T, db *bun.DB, announcementID, studentID int64) []int64 {
	t.Helper()
	var ids []int64
	require.NoError(t, db.NewSelect().
		TableExpr("users.parent_announcement_responses").
		ColumnExpr("option_id").
		Where("announcement_id = ? AND student_id = ?", announcementID, studentID).
		OrderExpr("option_id ASC").
		Scan(context.Background(), &ids))
	return ids
}

func TestParentAnnouncementSetResponse_StoresReplacesAndWithdraws(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	ctx := tenantCtx(t)

	poll, options := pollAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		usersModels.ParentAnnouncementResponseMultiChoice)

	applied, err := repo.SetResponse(ctx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		[]int64{options[0]}, *poll.PublishedAt)
	require.NoError(t, err)
	require.True(t, applied)
	assert.Equal(t, []int64{options[0]}, storedAnswers(t, db, poll.ID, chain.StudentID))

	// A replacement is a full swap, not an append: the previous choice goes.
	applied, err = repo.SetResponse(ctx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		[]int64{options[1]}, *poll.PublishedAt)
	require.NoError(t, err)
	require.True(t, applied)
	assert.Equal(t, []int64{options[1]}, storedAnswers(t, db, poll.ID, chain.StudentID))

	// An empty selection withdraws the answer entirely.
	applied, err = repo.SetResponse(ctx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		nil, *poll.PublishedAt)
	require.NoError(t, err)
	require.True(t, applied)
	assert.Empty(t, storedAnswers(t, db, poll.ID, chain.StudentID))
}

func TestParentAnnouncementSetResponse_RejectsWithoutPollPermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	ctx := tenantCtx(t)

	poll, options := pollAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		usersModels.ParentAnnouncementResponseSingleChoice)

	// Sanity: the seeded primary guardian may answer.
	applied, err := repo.SetResponse(ctx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		[]int64{options[0]}, *poll.PublishedAt)
	require.NoError(t, err)
	require.True(t, applied, "sanity: a primary guardian may answer")

	// Withdraw poll.response while keeping portal access. Seeing the poll and
	// answering it are separate grants, and only the second one authorizes a
	// write — a guardian downgraded to view-only must stop being able to answer.
	revokePollResponse(t, db, chain)

	// The guardian must still SEE the poll. Without this the refusal below could
	// pass for the wrong reason — a guardian who lost portal access entirely
	// would also be refused, and the test would no longer be about poll.response.
	visible, err := repo.AccountMatchesAnnouncement(ctx, chain.TenantID, poll.ID, chain.AccountID)
	require.NoError(t, err)
	require.True(t, visible, "the guardian keeps portal access; only poll.response is gone")

	applied, err = repo.SetResponse(ctx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		[]int64{options[1]}, *poll.PublishedAt)
	require.NoError(t, err)
	assert.False(t, applied, "a guardian without poll.response must not answer")
	assert.Equal(t, []int64{options[0]}, storedAnswers(t, db, poll.ID, chain.StudentID),
		"the refused write must leave the earlier answer untouched")

	// The refusal must also hold for a withdrawal: losing the grant must not let
	// the guardian delete an answer either.
	applied, err = repo.SetResponse(ctx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		nil, *poll.PublishedAt)
	require.NoError(t, err)
	assert.False(t, applied, "a guardian without poll.response must not withdraw")
	assert.Equal(t, []int64{options[0]}, storedAnswers(t, db, poll.ID, chain.StudentID))
}

func TestParentAnnouncementSetResponse_RejectsStaleVersionAndForeignOptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	ctx := tenantCtx(t)

	poll, options := pollAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		usersModels.ParentAnnouncementResponseMultiChoice)
	other, otherOptions := pollAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		usersModels.ParentAnnouncementResponseMultiChoice)
	require.NotEqual(t, poll.ID, other.ID)

	// A version the announcement never had: the client loaded a wording that has
	// since been corrected, so its answer belongs to text nobody is showing.
	stale := poll.PublishedAt.Add(-time.Hour)
	applied, err := repo.SetResponse(ctx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		[]int64{options[0]}, stale)
	require.NoError(t, err)
	assert.False(t, applied, "a stale version must not store an answer")
	assert.Empty(t, storedAnswers(t, db, poll.ID, chain.StudentID))

	// An option belonging to a different poll must not be storable against this
	// one, or the tally would count a choice nobody was offered.
	applied, err = repo.SetResponse(ctx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		[]int64{otherOptions[0]}, *poll.PublishedAt)
	require.NoError(t, err)
	assert.False(t, applied, "an option from another poll must be refused")
	assert.Empty(t, storedAnswers(t, db, poll.ID, chain.StudentID))
}

func TestParentAnnouncementSetResponse_SingleChoiceRejectsMultipleOptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	ctx := tenantCtx(t)

	poll, options := pollAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		usersModels.ParentAnnouncementResponseSingleChoice)

	applied, err := repo.SetResponse(ctx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		options, *poll.PublishedAt)
	require.NoError(t, err)
	assert.False(t, applied, "single_choice must refuse two options")
	assert.Empty(t, storedAnswers(t, db, poll.ID, chain.StudentID))
}

// revokePollResponse strips parent_portal.poll.response from the chain's
// guardian link while leaving parent_portal.access in place.
func revokePollResponse(t *testing.T, db *bun.DB, chain testpkg.ParentChain) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("users.students_guardians").
		Set("permissions = permissions - ?", authorize.GuardianPermissionPollResponse).
		Where("student_id = ? AND tenant_id = ?", chain.StudentID, chain.TenantID).
		Exec(context.Background())
	require.NoError(t, err)
}
