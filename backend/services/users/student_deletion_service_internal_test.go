package users

import (
	"testing"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func TestStudentDeletionValidationHelpers(t *testing.T) {
	for _, reason := range []string{
		StudentDeletionReasonTestData,
		StudentDeletionReasonIncorrectEntry,
		StudentDeletionReasonDuplicate,
		StudentDeletionReasonPrivacyRequest,
	} {
		assert.True(t, isValidStudentDeletionReason(reason))
	}
	assert.False(t, isValidStudentDeletionReason("other"))

	assert.True(t, equalFingerprint("aabb", " aabb "))
	assert.False(t, equalFingerprint("aabb", "ccdd"))
	assert.False(t, equalFingerprint("invalid", "aabb"))
	assert.False(t, equalFingerprint("aabb", "abc"))
}

func TestBuildStudentDeletionPreviewRejectsIncompleteData(t *testing.T) {
	_, err := buildStudentDeletionPreview(nil, nil, &userModels.StudentDeletionCounts{})
	assert.ErrorContains(t, err, "preview is incomplete")
}
