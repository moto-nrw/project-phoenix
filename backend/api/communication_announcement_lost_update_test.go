package api

import (
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The operator edit handler reads the announcement before the service locks
// it. An unpublish that commits in between must survive the edit: the edit
// writes the fields it changes, not the publication state of its stale copy.
func TestAnnouncementEditKeepsConcurrentUnpublish(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildCommunication(t, db, &auditRecorder{})
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	ownAnnouncementsByOperator(t, db, operator.ID)
	announcement := newAnnouncement("Before edit", testpkg.Tenant(t))
	require.NoError(t, module.CreateAnnouncement(ctx, announcement, operator.ID, nil))
	require.NoError(t, module.PublishAnnouncement(ctx, announcement.ID, operator.ID, nil))

	stale, err := module.GetAnnouncement(ctx, announcement.ID)
	require.NoError(t, err)
	require.NotNil(t, stale.PublishedAt)
	require.NoError(t, module.UnpublishAnnouncement(ctx, announcement.ID, operator.ID, nil))
	stale.Title = "After edit"
	require.NoError(t, module.UpdateAnnouncement(ctx, stale, operator.ID, nil))

	persisted, err := module.GetAnnouncement(ctx, announcement.ID)
	require.NoError(t, err)
	assert.Equal(t, "After edit", persisted.Title, "the edit itself lands")
	assert.Nil(t, persisted.PublishedAt, "the committed unpublish stays in force")
	assert.Nil(t, stale.PublishedAt, "the edit answers with the stored publication state")
}
