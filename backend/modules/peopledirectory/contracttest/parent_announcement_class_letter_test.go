package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestParentFeedNamesTheSenderOfAClassLetter pins #4800 for parents: the
// feed carries the Lehrkraft and her class of a class letter, and nothing of
// that kind on a letter of the OGS.
func TestParentFeedNamesTheSenderOfAClassLetter(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a"
	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)
	publish := func(title string) int64 {
		return api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
			return databaseTimestamp(t, db)
		}, repo, chain.AccountID, chain.TenantID, title, api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetClass, TargetRefText: strp("1a")},
		})).ID
	}
	letter := publish("Wandertag")
	ogs := publish("Sommerfest")
	_, err := db.ExecContext(context.Background(), `
		UPDATE users.parent_announcements SET sender_name = 'Lena Lehrerin', sender_school_class = '1a'
		WHERE id = ?`, letter)
	require.NoError(t, err)

	feed, err := testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, chain.AccountID,
		api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: []int64{chain.TenantID}}))
	require.NoError(t, err)
	seen := 0
	for _, item := range feed {
		switch item.ID {
		case letter:
			seen++
			require.NotNil(t, item.SenderName)
			require.NotNil(t, item.SenderSchoolClass)
			assert.Equal(t, "Lena Lehrerin", *item.SenderName)
			assert.Equal(t, "1a", *item.SenderSchoolClass)
		case ogs:
			seen++
			assert.Nil(t, item.SenderName, "a letter of the OGS names no Lehrkraft")
			assert.Nil(t, item.SenderSchoolClass)
		}
	}
	assert.Equal(t, 2, seen, "both letters reach the guardian of a 1a child")
}
