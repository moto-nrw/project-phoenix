package source_test

import (
	"testing"

	external "example.test/architecture-scopes/external-target"
	"example.test/architecture-scopes/source"
)

func TestExternalScope(t *testing.T) {
	t.Parallel()

	if source.Production == external.Value {
		t.Fatal("fixture values should differ")
	}
}
