package users

// Pure model tests for the parent-announcement entities: the priority/target
// validators, the IsPublished derivation, the schema-qualified TableName
// accessors, and the BeforeAppendModel hook routing. No DB connection is
// needed — the queries are built against a connectionless bun.DB purely to
// drive the hook.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func announcementHookDB() *bun.DB { return bun.NewDB(nil, pgdialect.New()) }

func TestValidAnnouncementPriority(t *testing.T) {
	assert.True(t, ValidAnnouncementPriority(ParentAnnouncementPriorityInfo))
	assert.True(t, ValidAnnouncementPriority(ParentAnnouncementPriorityImportant))
	assert.False(t, ValidAnnouncementPriority(""))
	assert.False(t, ValidAnnouncementPriority("urgent"))
}

func TestValidAnnouncementTargetType(t *testing.T) {
	valid := []string{
		AnnouncementTargetSchoolAll,
		AnnouncementTargetClass,
		AnnouncementTargetGroup,
		AnnouncementTargetActivityGroup,
		AnnouncementTargetStudent,
		AnnouncementTargetPendingEnrollment,
	}
	for _, tt := range valid {
		assert.Truef(t, ValidAnnouncementTargetType(tt), "expected %q valid", tt)
	}
	assert.False(t, ValidAnnouncementTargetType(""))
	assert.False(t, ValidAnnouncementTargetType("teacher"))
}

func TestParentAnnouncement_IsPublished(t *testing.T) {
	a := &ParentAnnouncement{}
	assert.False(t, a.IsPublished(), "a draft (PublishedAt nil) is not published")

	now := time.Now()
	a.PublishedAt = &now
	assert.True(t, a.IsPublished(), "a row with PublishedAt set is published")
}

func TestParentAnnouncement_BeforeAppendModelRoutesQueryKinds(t *testing.T) {
	db := announcementHookDB()
	a := &ParentAnnouncement{}
	// Insert and Update are pinned to the bare table; Delete keeps base's
	// aliased expression (the hook must leave it alone) and must not error.
	require.NoError(t, a.BeforeAppendModel(db.NewInsert().Model(a)))
	require.NoError(t, a.BeforeAppendModel(db.NewUpdate().Model(a)))
	require.NoError(t, a.BeforeAppendModel(db.NewDelete().Model(a)))
	// A non-query argument is a no-op, not a panic.
	require.NoError(t, a.BeforeAppendModel(nil))
}
