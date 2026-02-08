package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAnnouncement_Validate_EmptyTitle(t *testing.T) {
	a := &Announcement{
		Title:     "",
		Content:   "Content",
		Type:      TypeAnnouncement,
		Severity:  SeverityInfo,
		CreatedBy: 1,
	}
	err := a.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title is required")
}

func TestAnnouncement_Validate_TitleTooLong(t *testing.T) {
	longTitle := ""
	for i := 0; i < 201; i++ {
		longTitle += "a"
	}
	a := &Announcement{
		Title:     longTitle,
		Content:   "Content",
		Type:      TypeAnnouncement,
		Severity:  SeverityInfo,
		CreatedBy: 1,
	}
	err := a.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title must not exceed 200 characters")
}

func TestAnnouncement_Validate_EmptyContent(t *testing.T) {
	a := &Announcement{
		Title:     "Title",
		Content:   "",
		Type:      TypeAnnouncement,
		Severity:  SeverityInfo,
		CreatedBy: 1,
	}
	err := a.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content is required")
}

func TestAnnouncement_Validate_InvalidType(t *testing.T) {
	a := &Announcement{
		Title:     "Title",
		Content:   "Content",
		Type:      "invalid",
		Severity:  SeverityInfo,
		CreatedBy: 1,
	}
	err := a.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid announcement type")
}

func TestAnnouncement_Validate_InvalidSeverity(t *testing.T) {
	a := &Announcement{
		Title:     "Title",
		Content:   "Content",
		Type:      TypeAnnouncement,
		Severity:  "invalid",
		CreatedBy: 1,
	}
	err := a.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid severity")
}

func TestAnnouncement_Validate_MissingCreatedBy(t *testing.T) {
	a := &Announcement{
		Title:     "Title",
		Content:   "Content",
		Type:      TypeAnnouncement,
		Severity:  SeverityInfo,
		CreatedBy: 0,
	}
	err := a.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "created_by is required")
}

func TestAnnouncement_Validate_NegativeCreatedBy(t *testing.T) {
	a := &Announcement{
		Title:     "Title",
		Content:   "Content",
		Type:      TypeAnnouncement,
		Severity:  SeverityInfo,
		CreatedBy: -1,
	}
	err := a.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "created_by is required")
}

func TestAnnouncement_Validate_VersionTooLong(t *testing.T) {
	longVersion := ""
	for i := 0; i < 51; i++ {
		longVersion += "1"
	}
	a := &Announcement{
		Title:     "Title",
		Content:   "Content",
		Type:      TypeAnnouncement,
		Severity:  SeverityInfo,
		CreatedBy: 1,
		Version:   &longVersion,
	}
	err := a.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version must not exceed 50 characters")
}

func TestAnnouncement_Validate_Valid(t *testing.T) {
	version := "1.0.0"
	a := &Announcement{
		Title:     "Title",
		Content:   "Content",
		Type:      TypeAnnouncement,
		Severity:  SeverityInfo,
		CreatedBy: 1,
		Version:   &version,
	}
	err := a.Validate()
	assert.NoError(t, err)
}

func TestAnnouncement_Validate_TrimSpaces(t *testing.T) {
	a := &Announcement{
		Title:     "  Title  ",
		Content:   "  Content  ",
		Type:      TypeAnnouncement,
		Severity:  SeverityInfo,
		CreatedBy: 1,
	}
	err := a.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "Title", a.Title)
	assert.Equal(t, "Content", a.Content)
}

func TestIsValidAnnouncementType_Announcement(t *testing.T) {
	assert.True(t, IsValidAnnouncementType(TypeAnnouncement))
}

func TestIsValidAnnouncementType_Release(t *testing.T) {
	assert.True(t, IsValidAnnouncementType(TypeRelease))
}

func TestIsValidAnnouncementType_Maintenance(t *testing.T) {
	assert.True(t, IsValidAnnouncementType(TypeMaintenance))
}

func TestIsValidAnnouncementType_Invalid(t *testing.T) {
	assert.False(t, IsValidAnnouncementType("invalid"))
}

func TestIsValidSeverity_Info(t *testing.T) {
	assert.True(t, IsValidSeverity(SeverityInfo))
}

func TestIsValidSeverity_Warning(t *testing.T) {
	assert.True(t, IsValidSeverity(SeverityWarning))
}

func TestIsValidSeverity_Critical(t *testing.T) {
	assert.True(t, IsValidSeverity(SeverityCritical))
}

func TestIsValidSeverity_Invalid(t *testing.T) {
	assert.False(t, IsValidSeverity("invalid"))
}

func TestAnnouncement_IsPublished_Nil(t *testing.T) {
	a := &Announcement{}
	assert.False(t, a.IsPublished())
}

func TestAnnouncement_IsPublished_Future(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	a := &Announcement{
		PublishedAt: &future,
	}
	assert.False(t, a.IsPublished())
}

func TestAnnouncement_IsPublished_Past(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	a := &Announcement{
		PublishedAt: &past,
	}
	assert.True(t, a.IsPublished())
}

func TestAnnouncement_IsExpired_Nil(t *testing.T) {
	a := &Announcement{}
	assert.False(t, a.IsExpired())
}

func TestAnnouncement_IsExpired_Future(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	a := &Announcement{
		ExpiresAt: &future,
	}
	assert.False(t, a.IsExpired())
}

func TestAnnouncement_IsExpired_Past(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	a := &Announcement{
		ExpiresAt: &past,
	}
	assert.True(t, a.IsExpired())
}

func TestAnnouncement_IsDraft_Nil(t *testing.T) {
	a := &Announcement{}
	assert.True(t, a.IsDraft())
}

func TestAnnouncement_IsDraft_NotNil(t *testing.T) {
	published := time.Now()
	a := &Announcement{
		PublishedAt: &published,
	}
	assert.False(t, a.IsDraft())
}

func TestAnnouncement_TableName(t *testing.T) {
	a := &Announcement{}
	assert.Equal(t, "platform.announcements", a.TableName())
}

func TestAnnouncement_GetID(t *testing.T) {
	a := &Announcement{}
	a.ID = 123
	assert.Equal(t, int64(123), a.GetID())
}

func TestAnnouncement_GetCreatedAt(t *testing.T) {
	now := time.Now()
	a := &Announcement{}
	a.CreatedAt = now
	assert.Equal(t, now, a.GetCreatedAt())
}

func TestAnnouncement_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	a := &Announcement{}
	a.UpdatedAt = now
	assert.Equal(t, now, a.GetUpdatedAt())
}
