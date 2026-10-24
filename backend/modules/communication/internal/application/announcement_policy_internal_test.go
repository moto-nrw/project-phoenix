package application

// Pure model tests for the parent-announcement entities: the priority/target
// validators and the IsPublished derivation. No DB connection is needed.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidAnnouncementPriority(t *testing.T) {
	t.Parallel()

	assert.True(t, ValidAnnouncementPriority(ParentAnnouncementPriorityInfo))
	assert.True(t, ValidAnnouncementPriority(ParentAnnouncementPriorityImportant))
	assert.False(t, ValidAnnouncementPriority(""))
	assert.False(t, ValidAnnouncementPriority("urgent"))
}

func TestValidAnnouncementTargetType(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	a := &ParentAnnouncement{}
	assert.False(t, AnnouncementIsPublished(a), "a draft (PublishedAt nil) is not published")

	now := time.Now()
	a.PublishedAt = &now
	assert.True(t, AnnouncementIsPublished(a), "a row with PublishedAt set is published")
}
