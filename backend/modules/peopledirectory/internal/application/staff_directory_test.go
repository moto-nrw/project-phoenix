package application

import (
	"testing"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

func TestNewStaffDirectoryRequiresRuntime(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered != "staff directory: the tenant runtime is required" {
			t.Fatalf("expected missing runtime panic, got %v", recovered)
		}
	}()

	NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{})
}
