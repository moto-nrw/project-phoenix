package architecture

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelocationsCommandUsesCanonicalGitVerifier(t *testing.T) {
	t.Parallel()
	repo, base := relocatedRepository(t, nil)
	output, err := runArchitecture(t, "relocations", "--project", repo, "--base-ref", base, "--baseline", "architecture/legacy.jsonl")
	if err != nil {
		t.Fatalf("verified relocation metadata failed: %v\n%s", err, output)
	}
	var moves map[string]string
	if err := json.Unmarshal([]byte(output), &moves); err != nil {
		t.Fatal(err)
	}
	if len(moves) != 1 || moves["target"] != movedTargetPath {
		t.Fatalf("unexpected verified map: %v", moves)
	}
	writeFile(t, filepath.Join(repo, movedTargetPath, "new.go"), "package target\n")
	output, err = runArchitecture(t, "relocations", "--project", repo, "--base-ref", base, "--baseline", "architecture/legacy.jsonl")
	if err == nil || !strings.Contains(output, "is not a rename") {
		t.Fatalf("unverified file set was published: %v\n%s", err, output)
	}
}
