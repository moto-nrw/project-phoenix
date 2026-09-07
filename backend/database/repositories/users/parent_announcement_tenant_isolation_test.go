package users_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentAudience "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestParentAnnouncement_TenantIsolation pins that a school never reads or
// writes another school's announcement rows, across all five tables the
// Communication owner holds: users.parent_announcements, _targets, _options,
// _reads and _responses.
//
// It matters more since the SQL moved out of the generic repository base: the
// entity reads apply their own tenant filter, and the audience, feed, poll
// and stats reads run in a projection that joins People Directory rows. A
// missing filter there is a cross-school leak of a child's name and a family's
// poll answer, not a cosmetic bug.
func TestParentAnnouncement_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	homeCtx := tenantCtx(t)

	poll, options := pollAnnouncement(t, homeCtx, db, repo, chain.AccountID, chain.TenantID,
		usersModels.ParentAnnouncementResponseSingleChoice)
	live, err := repo.MarkRead(homeCtx, chain.TenantID, poll.ID, chain.AccountID, *poll.PublishedAt)
	require.NoError(t, err)
	require.True(t, live, "the fixture read must land in the owning school")
	applied, err := repo.SetResponse(homeCtx, chain.TenantID, poll.ID, chain.StudentID, chain.AccountID,
		[]int64{options[0]}, *poll.PublishedAt)
	require.NoError(t, err)
	require.True(t, applied, "the fixture answer must land in the owning school")

	// A second school that shares nothing with the first.
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreignCtx := testpkg.TenantContext(otherTenant)

	t.Run("announcement, target and option reads are scoped to the owning school", func(t *testing.T) {
		found, err := repo.FindByID(foreignCtx, poll.ID)
		require.NoError(t, err)
		assert.Nil(t, found, "another school must not load the announcement by id")

		locked, err := repo.FindByIDForUpdate(foreignCtx, poll.ID)
		require.NoError(t, err)
		assert.Nil(t, locked, "another school must not lock the announcement row")

		listed, err := repo.ListForTenant(foreignCtx, true)
		require.NoError(t, err)
		for _, row := range listed {
			assert.NotEqual(t, poll.ID, row.ID, "another school must not list the announcement")
		}

		targets, err := repo.ListTargets(foreignCtx, poll.ID)
		require.NoError(t, err)
		assert.Empty(t, targets, "another school must not read the audience selectors")

		listedOptions, err := repo.ListOptions(foreignCtx, poll.ID)
		require.NoError(t, err)
		assert.Empty(t, listedOptions, "another school must not read the poll options")
	})

	t.Run("audience, poll and read projections are scoped to the owning school", func(t *testing.T) {
		stats, err := repo.Stats(foreignCtx, otherTenant, poll.ID)
		require.NoError(t, err)
		assert.Equal(t, &usersModels.AnnouncementStats{}, stats, "another school must not count this audience or its reads")

		recipients, err := repo.AudienceRecipients(foreignCtx, otherTenant, poll.ID)
		require.NoError(t, err)
		assert.Empty(t, recipients, "another school must not read the guardians' names and read state")

		results, err := repo.PollResults(foreignCtx, otherTenant, poll.ID)
		require.NoError(t, err)
		assert.Zero(t, results.TargetChildCount)
		assert.Zero(t, results.AnsweredCount)
		assert.Empty(t, results.Options, "another school must not read the tally")

		children, err := repo.PollChildren(foreignCtx, otherTenant, poll.ID)
		require.NoError(t, err)
		assert.Empty(t, children, "another school must not read the children's answers")

		feed, err := repo.ListFeedForAccount(foreignCtx, chain.AccountID, usersModels.AnnouncementFeedScope{TenantIDs: []int64{otherTenant}})
		require.NoError(t, err)
		assert.Empty(t, feed, "a feed scoped to another school must not surface the announcement")

		matched, err := repo.AccountMatchesAnnouncement(foreignCtx, otherTenant, poll.ID, chain.AccountID)
		require.NoError(t, err)
		assert.False(t, matched, "another school must not place the guardian in this audience")
	})

	t.Run("read and answer writes are scoped to the owning school", func(t *testing.T) {
		live, err := repo.MarkAcknowledged(foreignCtx, otherTenant, poll.ID, chain.AccountID, *poll.PublishedAt)
		require.NoError(t, err)
		assert.False(t, live, "another school must not acknowledge on this announcement's behalf")

		may, err := repo.AccountMayAnswerForStudent(foreignCtx, otherTenant, poll.ID, chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		assert.False(t, may, "another school must not authorize an answer for this child")

		applied, err := repo.SetResponse(foreignCtx, otherTenant, poll.ID, chain.StudentID, chain.AccountID,
			[]int64{options[1]}, *poll.PublishedAt)
		require.NoError(t, err)
		assert.False(t, applied, "another school must not replace the child's answer")
		assert.Equal(t, []int64{options[0]}, storedAnswers(t, db, poll.ID, chain.StudentID),
			"the owning school's answer must survive the foreign attempt untouched")
	})

	t.Run("the owning school still reads its own rows", func(t *testing.T) {
		// The mirror assertion: without it a filter that excludes everything
		// would pass every check above.
		found, err := repo.FindByID(homeCtx, poll.ID)
		require.NoError(t, err)
		require.NotNil(t, found)

		listedOptions, err := repo.ListOptions(homeCtx, poll.ID)
		require.NoError(t, err)
		assert.Len(t, listedOptions, 2)

		stats, err := repo.Stats(homeCtx, chain.TenantID, poll.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, stats.TargetCount)
		assert.Equal(t, 1, stats.ReadCount)

		results, err := repo.PollResults(homeCtx, chain.TenantID, poll.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, results.AnsweredCount)
	})
}
