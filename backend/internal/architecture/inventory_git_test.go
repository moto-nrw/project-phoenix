package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
	"github.com/moto-nrw/project-phoenix/internal/gitfixture"
)

func TestInventoryBaseRemainsImmutableAfterCandidateMutation(t *testing.T) {
	t.Parallel()
	original := []byte(`{"schema_version":1,"entries":[{"source_owner":"alpha","port":"alpha.Read","binding":"alpha.Owner.Read","target":"alpha.Store.Read","binding_digest":"frozen","effect":"query","classification":"local","reason":"Reviewed query"}]}`)
	root := t.TempDir()
	filename := filepath.Join(root, "command-effects.json")
	if err := artifacts.WriteInventory(filename, artifacts.CommandEffects, original); err != nil {
		t.Fatal(err)
	}
	gitfixture.Init(t, root)
	runGit(t, root, "add", ".")
	runGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "Base")
	sha := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
	if err := os.RemoveAll(filepath.Join(root, "command-effects")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(`{"schema_version":1,"entries":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := artifacts.ReadInventoryAtRevision(root, sha, ".", "command-effects.json", artifacts.CommandEffects, true)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(original, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stored.Document, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("base changed with candidate: %s", stored.Document)
	}
}

func TestImmutableEmptyInventoryNeedsNoTrackedDirectories(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	filename := filepath.Join(root, "command-effects.json")
	if err := artifacts.WriteInventory(filename, artifacts.CommandEffects, []byte(`{"schema_version":1,"entries":[]}`)); err != nil {
		t.Fatal(err)
	}
	gitfixture.Init(t, root)
	runGit(t, root, "add", ".")
	runGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "Empty inventory")
	sha := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
	stored, err := artifacts.ReadInventoryAtRevision(root, sha, ".", "command-effects.json", artifacts.CommandEffects, true)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := decodeCommandEffects(stored.Document)
	if err != nil || len(manifest.Entries) != 0 {
		t.Fatalf("empty immutable base: %s, %v", stored.Document, err)
	}
	if _, err := artifacts.ReadInventoryAtRevision(root, strings.Repeat("0", 40), ".", "command-effects.json", artifacts.CommandEffects, true); err == nil {
		t.Fatal("unresolved revision became empty inventory")
	}
}

func TestInventoryOrphanBaseCannotBootstrap(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	reviewGapFixture(t, project, policy, "query", "query", "")
	if err := os.Remove(filepath.Join(project, commandEffectInventory)); err != nil {
		t.Fatal(err)
	}
	sha := commitGapFixture(t, project)
	_, _, err := loadBaseCommandEffects(project, effectBase{root: project, sha: sha, addedADRs: map[string]bool{}})
	if err == nil || !strings.Contains(err.Error(), "orphan") {
		t.Fatalf("orphan base bypassed immutable inventory load: %v", err)
	}
}
