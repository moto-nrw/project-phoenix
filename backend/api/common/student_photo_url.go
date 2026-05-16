package common

import (
	"fmt"
	"strings"
)

// StudentPhotoStoredURLPrefix is the persisted prefix for `users.students.photo_path`.
// Changing it is a breaking migration — must stay in sync with publicPhotoBaseDir
// in api/students/photo.go.
const StudentPhotoStoredURLPrefix = "/uploads/student-photos/"

// BuildStudentPhotoServeURL rewrites the stored URL to the authenticated
// serve route. Returns "" for empty input; passes through unrecognised
// prefixes unchanged so legacy/manual data fails loudly instead of silently.
func BuildStudentPhotoServeURL(studentID int64, storedURL string) string {
	if storedURL == "" {
		return ""
	}
	if !strings.HasPrefix(storedURL, StudentPhotoStoredURLPrefix) {
		return storedURL
	}
	filename := strings.TrimPrefix(storedURL, StudentPhotoStoredURLPrefix)
	return fmt.Sprintf("/api/students/%d/photo/%s", studentID, filename)
}
