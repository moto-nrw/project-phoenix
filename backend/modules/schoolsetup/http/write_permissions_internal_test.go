package schoolsetuphttp

import (
	"slices"
	"testing"
)

func TestWritePermissionsAreConfigUpdateOrManage(t *testing.T) {
	t.Parallel()
	if got := WritePermissions(); !slices.Equal(got, []string{"config:update", "config:manage"}) {
		t.Errorf("WritePermissions() = %v", got)
	}
}
