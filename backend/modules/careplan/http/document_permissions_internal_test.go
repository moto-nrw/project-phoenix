package careplanhttp

import "testing"

func TestStudentDocumentPermissionsPerCategory(t *testing.T) {
	t.Parallel()
	health, legal, fallback := StudentDocumentPermissions()
	if health != "student_documents:health" || legal != "student_documents:legal" || fallback != "users:update" {
		t.Errorf("StudentDocumentPermissions() = %q, %q, %q", health, legal, fallback)
	}
}
