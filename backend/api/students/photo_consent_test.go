package students

// Unit tests for applyPhotoConsent — pure helper, no DB. Verifying the four
// transition cases keeps the consent-withdrawal cleanup invariant explicit:
// withdrawal must clear path + audit columns AND surface the old path so
// the handler can unlink the file post-commit.

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func ptrBool(b bool) *bool           { return &b }
func ptrString(s string) *string     { return &s }
func ptrTime(t time.Time) *time.Time { return &t }
func ptrInt64(v int64) *int64        { return &v }

func TestApplyPhotoConsent_NoChangeWhenFieldOmitted(t *testing.T) {
	now := time.Now().Add(-time.Hour)
	student := &users.Student{
		PhotoPath:           ptrString("/uploads/student-photos/foo.jpg"),
		PhotoConsentGivenAt: ptrTime(now),
		PhotoConsentGivenBy: ptrInt64(42),
	}
	req := &UpdateStudentRequest{} // PhotoConsentGiven nil → no transition

	got := applyPhotoConsent(req, student)

	assert.False(t, got.GrantedNow)
	assert.False(t, got.WithdrawnNow)
	assert.Equal(t, "", got.RemovedPhotoPath)
	// Existing state must remain untouched
	assert.NotNil(t, student.PhotoPath)
	assert.NotNil(t, student.PhotoConsentGivenAt)
	assert.NotNil(t, student.PhotoConsentGivenBy)
}

func TestApplyPhotoConsent_GrantTransition(t *testing.T) {
	student := &users.Student{} // no consent yet
	req := &UpdateStudentRequest{PhotoConsentGiven: ptrBool(true)}

	got := applyPhotoConsent(req, student)

	assert.True(t, got.GrantedNow)
	assert.False(t, got.WithdrawnNow)
	assert.Equal(t, "", got.RemovedPhotoPath)
	// applyPhotoConsent itself does NOT stamp _at/_by — handler does that
	// because only it knows the acting account from JWT context.
	assert.Nil(t, student.PhotoConsentGivenAt)
	assert.Nil(t, student.PhotoConsentGivenBy)
}

func TestApplyPhotoConsent_WithdrawalClearsAndSurfacesPath(t *testing.T) {
	now := time.Now().Add(-time.Hour)
	student := &users.Student{
		PhotoPath:           ptrString("/uploads/student-photos/abc.jpg"),
		PhotoConsentGivenAt: ptrTime(now),
		PhotoConsentGivenBy: ptrInt64(7),
	}
	req := &UpdateStudentRequest{PhotoConsentGiven: ptrBool(false)}

	got := applyPhotoConsent(req, student)

	assert.False(t, got.GrantedNow)
	assert.True(t, got.WithdrawnNow)
	assert.Equal(t, "/uploads/student-photos/abc.jpg", got.RemovedPhotoPath,
		"path must be surfaced so the handler can unlink the file post-commit")
	// All three columns must be nilled in a single update
	assert.Nil(t, student.PhotoPath)
	assert.Nil(t, student.PhotoConsentGivenAt)
	assert.Nil(t, student.PhotoConsentGivenBy)
}

func TestApplyPhotoConsent_WithdrawalWithoutPhoto(t *testing.T) {
	// Edge case: consent revoked before any photo was uploaded. Path stays
	// empty (nothing to delete) but consent metadata still clears.
	student := &users.Student{
		PhotoConsentGivenAt: ptrTime(time.Now()),
		PhotoConsentGivenBy: ptrInt64(1),
	}
	req := &UpdateStudentRequest{PhotoConsentGiven: ptrBool(false)}

	got := applyPhotoConsent(req, student)

	assert.True(t, got.WithdrawnNow)
	assert.Equal(t, "", got.RemovedPhotoPath)
	assert.Nil(t, student.PhotoConsentGivenAt)
	assert.Nil(t, student.PhotoConsentGivenBy)
}

func TestApplyPhotoConsent_AlreadyGrantedNoOp(t *testing.T) {
	now := time.Now().Add(-time.Hour)
	student := &users.Student{
		PhotoConsentGivenAt: ptrTime(now),
		PhotoConsentGivenBy: ptrInt64(99),
	}
	req := &UpdateStudentRequest{PhotoConsentGiven: ptrBool(true)}

	got := applyPhotoConsent(req, student)

	assert.False(t, got.GrantedNow,
		"sending consent=true when already granted must NOT re-stamp")
	assert.False(t, got.WithdrawnNow)
	// Original timestamp must remain — re-stamping would falsely reset who/when
	assert.True(t, student.PhotoConsentGivenAt.Equal(now))
	assert.Equal(t, int64(99), *student.PhotoConsentGivenBy)
}
