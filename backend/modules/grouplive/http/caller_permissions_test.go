package grouplivehttp_test

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	groupLiveHTTP "github.com/moto-ogs/project-phoenix/modules/grouplive/http"
)

func TestGroupReadPermissionNamesGroupsRead(t *testing.T) {
	t.Parallel()
	if groupLiveHTTP.GroupReadPermission != permissions.GroupsRead {
		t.Errorf("GroupReadPermission = %q", groupLiveHTTP.GroupReadPermission)
	}
}
