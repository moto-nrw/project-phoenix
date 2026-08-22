package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAnnouncement_Validate_EmptyTitle(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	assert.True(t, IsValidAnnouncementType(TypeAnnouncement))
}

func TestIsValidAnnouncementType_Release(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidAnnouncementType(TypeRelease))
}

func TestIsValidAnnouncementType_Maintenance(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidAnnouncementType(TypeMaintenance))
}

func TestIsValidAnnouncementType_Invalid(t *testing.T) {
	t.Parallel()

	assert.False(t, IsValidAnnouncementType("invalid"))
}

func TestIsValidSeverity_Info(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidSeverity(SeverityInfo))
}

func TestIsValidSeverity_Warning(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidSeverity(SeverityWarning))
}

func TestIsValidSeverity_Critical(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidSeverity(SeverityCritical))
}

func TestIsValidSeverity_Invalid(t *testing.T) {
	t.Parallel()

	assert.False(t, IsValidSeverity("invalid"))
}

func TestAnnouncement_IsDraft_Nil(t *testing.T) {
	t.Parallel()

	a := &Announcement{}
	assert.True(t, a.IsDraft())
}

func TestAnnouncement_IsDraft_NotNil(t *testing.T) {
	t.Parallel()

	published := time.Now()
	a := &Announcement{
		PublishedAt: &published,
	}
	assert.False(t, a.IsDraft())
}

func TestAnnouncement_GetID(t *testing.T) {
	t.Parallel()

	a := &Announcement{}
	a.ID = 123
	assert.Equal(t, int64(123), a.GetID())
}

func TestAnnouncement_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	a := &Announcement{}
	a.CreatedAt = now
	assert.Equal(t, now, a.GetCreatedAt())
}

func TestAnnouncement_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	a := &Announcement{}
	a.UpdatedAt = now
	assert.Equal(t, now, a.GetUpdatedAt())
}

func TestAnnouncement_Validate_NilSliceNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		targetOrgIDs    []int64
		targetTenantIDs []int64
		targetRoles     []string
		wantOrgIDs      []int64
		wantTenantIDs   []int64
		wantRoles       []string
	}{
		{
			name:            "nil TargetOrgIDs normalized to empty",
			targetOrgIDs:    nil,
			targetTenantIDs: []int64{5},
			targetRoles:     []string{"admin"},
			wantOrgIDs:      []int64{},
			wantTenantIDs:   []int64{5},
			wantRoles:       []string{"admin"},
		},
		{
			name:            "nil TargetTenantIDs normalized to empty",
			targetOrgIDs:    []int64{1, 2},
			targetTenantIDs: nil,
			targetRoles:     []string{"user"},
			wantOrgIDs:      []int64{1, 2},
			wantTenantIDs:   []int64{},
			wantRoles:       []string{"user"},
		},
		{
			name:            "nil TargetRoles normalized to empty",
			targetOrgIDs:    []int64{1},
			targetTenantIDs: []int64{10},
			targetRoles:     nil,
			wantOrgIDs:      []int64{1},
			wantTenantIDs:   []int64{10},
			wantRoles:       []string{},
		},
		{
			name:            "all three nil normalized to empty",
			targetOrgIDs:    nil,
			targetTenantIDs: nil,
			targetRoles:     nil,
			wantOrgIDs:      []int64{},
			wantTenantIDs:   []int64{},
			wantRoles:       []string{},
		},
		{
			name:            "non-nil slices preserved",
			targetOrgIDs:    []int64{1, 2, 3},
			targetTenantIDs: []int64{10, 20},
			targetRoles:     []string{"admin", "user"},
			wantOrgIDs:      []int64{1, 2, 3},
			wantTenantIDs:   []int64{10, 20},
			wantRoles:       []string{"admin", "user"},
		},
		{
			name:            "empty non-nil slices preserved",
			targetOrgIDs:    []int64{},
			targetTenantIDs: []int64{},
			targetRoles:     []string{},
			wantOrgIDs:      []int64{},
			wantTenantIDs:   []int64{},
			wantRoles:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Announcement{
				Title:           "Title",
				Content:         "Content",
				Type:            TypeAnnouncement,
				Severity:        SeverityInfo,
				CreatedBy:       1,
				TargetOrgIDs:    tt.targetOrgIDs,
				TargetTenantIDs: tt.targetTenantIDs,
				TargetRoles:     tt.targetRoles,
			}
			err := a.Validate()
			assert.NoError(t, err)
			assert.NotNil(t, a.TargetOrgIDs)
			assert.NotNil(t, a.TargetTenantIDs)
			assert.NotNil(t, a.TargetRoles)
			assert.Equal(t, tt.wantOrgIDs, a.TargetOrgIDs)
			assert.Equal(t, tt.wantTenantIDs, a.TargetTenantIDs)
			assert.Equal(t, tt.wantRoles, a.TargetRoles)
		})
	}
}
