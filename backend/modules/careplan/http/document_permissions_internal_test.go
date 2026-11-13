package careplanhttp_test

import (
	"testing"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

func TestStudentDocumentPermissionsPerCategory(t *testing.T) {
	t.Parallel()
	health, legal, fallback := careplanhttp.StudentDocumentPermissions()
	if health != "student_documents:health" || legal != "student_documents:legal" || fallback != "users:update" {
		t.Errorf("StudentDocumentPermissions() = %q, %q, %q", health, legal, fallback)
	}
}
