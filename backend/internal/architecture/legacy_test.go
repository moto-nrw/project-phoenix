package architecture

import (
	"strings"
	"testing"
)

func TestDecodeLegacyManifestAcceptsModuleRootSourcePackage(t *testing.T) {
	t.Parallel()

	manifest, err := DecodeLegacyManifest(strings.NewReader(`{"scope":"production","rule":"imports.forbidden","source":"example.com","target":"example.com/target","issue":"https://github.com/moto-nrw/project-phoenix/issues/2583"}`))
	if err != nil {
		t.Fatalf("module-root source package was rejected: %v", err)
	}
	if len(manifest.Entries) != 1 {
		t.Fatalf("loaded %d legacy entries, want 1", len(manifest.Entries))
	}
}

func TestDecodeLegacyManifestRejectsNonPackageTarget(t *testing.T) {
	t.Parallel()

	_, err := DecodeLegacyManifest(strings.NewReader(`{"scope":"production","rule":"imports.forbidden","source":"example.com/source","target":"services","issue":"https://github.com/moto-nrw/project-phoenix/issues/2583"}`))
	if err == nil || !strings.Contains(err.Error(), "target \"services\" must be an exact Go package path") {
		t.Fatalf("non-package target was accepted: %v", err)
	}
}
