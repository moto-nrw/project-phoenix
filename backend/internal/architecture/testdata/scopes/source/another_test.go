package source

import (
	"testing"

	internal "example.test/architecture-scopes/internal-target"
)

func TestOtherInternalScope(t *testing.T) {
	t.Parallel()

	if internal.Value == "" {
		t.Fatal("empty fixture value")
	}
}
