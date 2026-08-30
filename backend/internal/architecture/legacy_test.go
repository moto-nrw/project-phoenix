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

func TestDecodeLegacyManifestAcceptsStandardLibraryTarget(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"net/http", "testing"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			record := `{"scope":"production","rule":"imports.forbidden","source":"example.com/source","target":"` + target + `","issue":"https://github.com/moto-nrw/project-phoenix/issues/2583"}`
			manifest, err := DecodeLegacyManifest(strings.NewReader(record))
			if err != nil {
				t.Fatalf("standard-library target was rejected: %v", err)
			}
			if len(manifest.Entries) != 1 {
				t.Fatalf("loaded %d legacy entries, want 1", len(manifest.Entries))
			}
		})
	}
}

func TestDecodeLegacyManifestAcceptsSemanticTarget(t *testing.T) {
	t.Parallel()

	manifest, err := DecodeLegacyManifest(strings.NewReader(`{"scope":"production","rule":"tables.unresolved","source":"example.com/source","target":"account_permissions_direct","issue":"https://github.com/moto-nrw/project-phoenix/issues/2583"}`))
	if err != nil {
		t.Fatalf("semantic target was rejected: %v", err)
	}
	if len(manifest.Entries) != 1 {
		t.Fatalf("loaded %d legacy entries, want 1", len(manifest.Entries))
	}
}

func TestDecodeLegacyManifestRejectsPackageFamilyTargets(t *testing.T) {
	t.Parallel()

	for _, rule := range []string{"imports.forbidden", "policy.rules-overlap"} {
		t.Run(rule, func(t *testing.T) {
			t.Parallel()

			record := `{"scope":"production","rule":"` + rule + `","source":"example.com/source","target":"services","issue":"https://github.com/moto-nrw/project-phoenix/issues/2583"}`
			_, err := DecodeLegacyManifest(strings.NewReader(record))
			if err == nil || !strings.Contains(err.Error(), "target \"services\" must be an exact Go package path") {
				t.Fatalf("package-family target was accepted: %v", err)
			}
		})
	}
}
