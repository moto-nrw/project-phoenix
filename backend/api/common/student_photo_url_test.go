package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BuildStudentPhotoServeURL is the boundary helper that rewrites stored
// URLs (under /uploads/student-photos/) to the authenticated proxy URL
// (/api/students/{id}/photo/{filename}). A typo here would silently 404
// every avatar render.

func TestBuildStudentPhotoServeURL_Empty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, BuildStudentPhotoServeURL(7, ""))
}

func TestBuildStudentPhotoServeURL_HappyPath(t *testing.T) {
	t.Parallel()

	stored := StudentPhotoStoredURLPrefix + "abc.jpg"
	got := BuildStudentPhotoServeURL(123, stored)
	assert.Equal(t, "/api/students/123/photo/abc.jpg", got)
	assert.True(t, strings.HasPrefix(got, "/api/students/"),
		"serve URL must use the authenticated proxy prefix, not /uploads/...")
}

func TestBuildStudentPhotoServeURL_LegacyPath(t *testing.T) {
	t.Parallel()

	legacy := "/legacy/avatars/foo.jpg"
	assert.Equal(t, legacy, BuildStudentPhotoServeURL(1, legacy))
}
