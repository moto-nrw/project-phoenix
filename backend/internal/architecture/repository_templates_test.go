package architecture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// The real repository policy is immutable during a test binary. Validate it
// once; repositoryPolicy decodes these bytes into independent mutable copies.
var repositoryPolicyJSON = sync.OnceValues(func() ([]byte, error) {
	policy, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		return nil, err
	}
	return json.Marshal(policy)
})

type ratchetTemplateKey struct {
	baseline, policy string
}

type ratchetTemplate struct {
	directory, base string
}

var ratchetTemplates = struct {
	sync.Mutex
	directory string
	entries   map[ratchetTemplateKey]ratchetTemplate
}{entries: make(map[ratchetTemplateKey]ratchetTemplate)}

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "architecture-ratchet-templates-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ratchetTemplates.directory = directory
	code := m.Run()
	if err := os.RemoveAll(directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}

// Cache only the immutable initial commit, keyed by both fixture inputs.
// Each caller copies every file, including .git, into its own t.TempDir:
// working files, indexes, objects and refs are never shared between tests.
// TestMain keeps templates alive until all parallel tests finish, then removes
// only this binary's private template directory, including after test failures.
func ratchetRepositoryTemplate(t *testing.T, baseline, policy string) ratchetTemplate {
	t.Helper()
	ratchetTemplates.Lock()
	defer ratchetTemplates.Unlock()
	key := ratchetTemplateKey{baseline: baseline, policy: policy}
	if template, exists := ratchetTemplates.entries[key]; exists {
		return template
	}
	directory, err := os.MkdirTemp(ratchetTemplates.directory, "repo-")
	if err != nil {
		t.Fatal(err)
	}
	template := ratchetTemplate{directory: directory, base: initializeRatchetRepository(t, directory, baseline, policy)}
	// Pack only this immutable fixture. Copies keep independent indexes, refs
	// and objects, without copying one file and directory per loose object.
	runGit(t, directory, "repack", "-adq", "--threads=1")
	ratchetTemplates.entries[key] = template
	return template
}
