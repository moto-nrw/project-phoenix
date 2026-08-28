package source_test

import (
	"testing"

	external "example.test/architecture-scopes/external-target"
	"example.test/architecture-scopes/source"
)

func TestOtherExternalScope(t *testing.T) {
	t.Parallel()

	if source.OtherProduction == external.Value {
		t.Fatal("fixture values should differ")
	}
}
