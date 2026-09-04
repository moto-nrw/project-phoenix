package communication

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnouncementValidateNormalizesPublicValue(t *testing.T) {
	t.Parallel()
	value := &Announcement{
		Title: "  Wartung  ", Content: "  Heute Abend  ", Type: TypeMaintenance,
		Severity: SeverityWarning, CreatedBy: 17,
	}

	require.NoError(t, value.Validate())
	assert.Equal(t, "Wartung", value.Title)
	assert.Equal(t, "Heute Abend", value.Content)
	assert.Empty(t, value.TargetRoles)
	assert.NotNil(t, value.TargetRoles)
	assert.NotNil(t, value.TargetOrgIDs)
	assert.NotNil(t, value.TargetTenantIDs)
}

func TestAnnouncementValidateRejectsInvalidBoundaryValues(t *testing.T) {
	t.Parallel()
	tests := map[string]*Announcement{
		"nil":           nil,
		"title":         {Content: "body", Type: TypeAnnouncement, Severity: SeverityInfo, CreatedBy: 1},
		"content":       {Title: "title", Type: TypeAnnouncement, Severity: SeverityInfo, CreatedBy: 1},
		"type":          {Title: "title", Content: "body", Type: "unknown", Severity: SeverityInfo, CreatedBy: 1},
		"severity":      {Title: "title", Content: "body", Type: TypeAnnouncement, Severity: "unknown", CreatedBy: 1},
		"creator":       {Title: "title", Content: "body", Type: TypeAnnouncement, Severity: SeverityInfo},
		"version":       {Title: "title", Content: "body", Type: TypeAnnouncement, Severity: SeverityInfo, CreatedBy: 1, Version: stringPointer(strings.Repeat("v", 51))},
		"organizations": {Title: "title", Content: "body", Type: TypeAnnouncement, Severity: SeverityInfo, CreatedBy: 1, TargetOrgIDs: make([]int64, MaxTargetIDs+1)},
		"schools":       {Title: "title", Content: "body", Type: TypeAnnouncement, Severity: SeverityInfo, CreatedBy: 1, TargetTenantIDs: make([]int64, MaxTargetIDs+1)},
	}
	for name, value := range tests {
		value := value
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, value.Validate())
		})
	}
}

func TestAnnouncementPublicErrorCodesAreStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "announcement_not_found", ErrorCode(&AnnouncementNotFoundError{AnnouncementID: 9}))
	inner := errors.New("invalid target")
	invalid := &InvalidDataError{Err: inner}
	assert.Equal(t, "invalid_announcement", ErrorCode(invalid))
	assert.ErrorIs(t, invalid, ErrInvalidAnnouncement)
	assert.ErrorIs(t, invalid, inner)
	assert.Equal(t, "internal_error", ErrorCode(errors.New("database unavailable")))
}

func TestIsAnnouncementExpiredUsesStrictInstant(t *testing.T) {
	t.Parallel()
	now := time.Now()
	assert.False(t, IsAnnouncementExpired(nil, now))
	assert.False(t, IsAnnouncementExpired(&Announcement{ExpiresAt: &now}, now))
	past := now.Add(-time.Nanosecond)
	assert.True(t, IsAnnouncementExpired(&Announcement{ExpiresAt: &past}, now))
}

func stringPointer(value string) *string { return &value }
