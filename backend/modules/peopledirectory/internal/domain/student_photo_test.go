package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The cases below moved here with the photo lifecycle (#3349); they previously
// drove services/users.applyPhotoConsent and photoFilenameOf.

func photoBoolPtr(b bool) *bool           { return &b }
func photoStringPtr(s string) *string     { return &s }
func photoTimePtr(t time.Time) *time.Time { return &t }
func photoInt64Ptr(v int64) *int64        { return &v }

func TestApplyPhotoConsentNoChangeWhenFieldOmitted(t *testing.T) {
	t.Parallel()

	granted := time.Now().Add(-time.Hour)
	current := StudentPhoto{
		PhotoPath:           photoStringPtr("/uploads/student-photos/foo.jpg"),
		PhotoConsentGivenAt: photoTimePtr(granted),
		PhotoConsentGivenBy: photoInt64Ptr(42),
	}

	updated, transition := ApplyPhotoConsent(nil, current, time.Now(), 7)

	assert.False(t, transition.GrantedNow)
	assert.False(t, transition.WithdrawnNow)
	assert.Equal(t, "", transition.RemovedPhotoPath)
	assert.NotNil(t, updated.PhotoPath)
	assert.NotNil(t, updated.PhotoConsentGivenAt)
}

func TestApplyPhotoConsentGrantTransition(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.UTC)
	updated, transition := ApplyPhotoConsent(photoBoolPtr(true), StudentPhoto{}, now, 42)

	assert.True(t, transition.GrantedNow)
	assert.False(t, transition.WithdrawnNow)
	// The grant is stamped with the acting account, so the trail names who
	// confirmed it.
	assert.Equal(t, now, *updated.PhotoConsentGivenAt)
	assert.Equal(t, int64(42), *updated.PhotoConsentGivenBy)
}

func TestApplyPhotoConsentWithdrawalClearsAndSurfacesPath(t *testing.T) {
	t.Parallel()

	current := StudentPhoto{
		PhotoPath:           photoStringPtr("/uploads/student-photos/abc.jpg"),
		PhotoConsentGivenAt: photoTimePtr(time.Now()),
		PhotoConsentGivenBy: photoInt64Ptr(7),
	}

	updated, transition := ApplyPhotoConsent(photoBoolPtr(false), current, time.Now(), 9)

	assert.True(t, transition.WithdrawnNow)
	assert.Equal(t, "/uploads/student-photos/abc.jpg", transition.RemovedPhotoPath)
	assert.Nil(t, updated.PhotoPath)
	assert.Nil(t, updated.PhotoConsentGivenAt)
	assert.Nil(t, updated.PhotoConsentGivenBy)
}

func TestApplyPhotoConsentAlreadyGrantedNoOp(t *testing.T) {
	t.Parallel()

	granted := time.Now().Add(-time.Hour)
	current := StudentPhoto{
		PhotoConsentGivenAt: photoTimePtr(granted),
		PhotoConsentGivenBy: photoInt64Ptr(99),
	}

	updated, transition := ApplyPhotoConsent(photoBoolPtr(true), current, time.Now(), 1)

	assert.False(t, transition.GrantedNow,
		"sending consent=true when already granted must NOT re-stamp")
	assert.True(t, updated.PhotoConsentGivenAt.Equal(granted))
	assert.Equal(t, int64(99), *updated.PhotoConsentGivenBy)
}

func TestPhotoFilenameOf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "abc.jpg", PhotoFilenameOf("/uploads/student-photos/abc.jpg"))
	assert.Equal(t, "abc.jpg", PhotoFilenameOf("abc.jpg"))
	assert.Equal(t, "", PhotoFilenameOf("/uploads/student-photos/"))
}
