package common

import (
	"fmt"
	"strings"
)

// StudentPhotoStoredURLPrefix is the URL prefix the backend persists in
// `users.students.photo_path` after a successful upload. The serve route
// (api/students.serveStudentPhoto) and the file unlink path
// (common.ResolveStoredPath) both key off this prefix, so changing it is a
// breaking migration — keep this and the on-disk dir (publicPhotoBaseDir)
// in `api/students/photo.go`) consistent.
const StudentPhotoStoredURLPrefix = "/uploads/student-photos/"

// BuildStudentPhotoServeURL converts the raw `/uploads/student-photos/...`
// URL stored in the DB into the authenticated serve URL the frontend
// consumes (`/api/students/{id}/photo/{filename}`). Used by both the
// students response shaper and the active-group visit display response so
// every endpoint returning a student-photo URL stays in sync.
//
// Returns:
//   - "" when storedURL is empty (no photo set / feature off).
//   - The rewritten /api/students/... URL when storedURL has the expected
//     prefix.
//   - storedURL unchanged otherwise — defensive: legacy data or manual
//     DB edits flow through so they fail loudly in the network tab
//     instead of silently 404-ing through the proxy.
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
