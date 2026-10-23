package test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/passthrough"
)

// TestMethodPassthroughRatchet supplements, not replaces, the ADR 0075 owner
// floors. Exact package/type/method and normalized body evidence prevent an
// unchanged owner sum from hiding a swap or a relocation. The architecture
// CLI also checks the registry against the immutable PR base (ADR 0090).
func TestMethodPassthroughRatchet(t *testing.T) {
	t.Parallel()
	if modulePassthroughMaxBodyLines != passthrough.MaxBodyLines {
		t.Fatal("legacy and shared predicate thresholds differ")
	}
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := passthrough.Check(root, ""); err != nil {
		t.Fatal(err)
	}
}
