package securityruntime

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

// TestStudentDocumentPermissionsMatchTheRegistry pins the restated names to
// the permission registry: a renamed permission must not silently leave a
// document category guarded by a name no role holds.
func TestStudentDocumentPermissionsMatchTheRegistry(t *testing.T) {
	t.Parallel()

	for restated, registered := range map[string]string{
		PermissionStudentDocumentsHealth: permissions.StudentDocumentsHealth,
		PermissionStudentDocumentsLegal:  permissions.StudentDocumentsLegal,
		PermissionUsersUpdate:            permissions.UsersUpdate,
		PermissionUsersAbsence:           permissions.UsersAbsence,
	} {
		if restated != registered {
			t.Errorf("restated permission %q, registry has %q", restated, registered)
		}
	}
}
