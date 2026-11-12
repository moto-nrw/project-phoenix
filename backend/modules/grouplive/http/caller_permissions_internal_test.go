package grouplivehttp

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
)

func TestGroupReadPermissionNamesGroupsRead(t *testing.T) {
	t.Parallel()
	if GroupReadPermission != permissions.GroupsRead {
		t.Errorf("GroupReadPermission = %q", GroupReadPermission)
	}
}
