package passthrough

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestImportSnapshotKeepsReviewedEvidenceAndRejectsStaleSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "architecture"), 0750); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "owner", "port.go")
	if err := os.MkdirAll(filepath.Dir(source), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("package owner\nfunc (p *Port) Read() error { return p.store.Read() }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var measured bytes.Buffer
	if err := WriteSnapshot(&measured, root, "0090"); err != nil {
		t.Fatal(err)
	}
	reviewed := filepath.Join(root, "reviewed.json")
	if err := os.WriteFile(reviewed, measured.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportSnapshot(root, reviewed); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "architecture", "passthrough.json")
	before, err := readRegistry(filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Sites) != 1 || before.Sites[0].Decision != "0090" || before.Sites[0].Reason == "" {
		t.Fatalf("review lost: %+v", before)
	}
	if err := os.WriteFile(source, []byte("package owner\nfunc (p *Port) Read() error { return p.store.Changed() }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportSnapshot(root, reviewed); err == nil || !strings.Contains(err.Error(), "snapshot does not match") {
		t.Fatalf("stale source accepted: %v", err)
	}
	after, err := readRegistry(filename)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("rejected import changed reviewed inventory: %v", err)
	}
}

func TestImportSnapshotRejectsForgedScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	snapshot := Registry{Scope: "forged", Decision: "0090", Sites: []Site{}}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "reviewed.json")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportSnapshot(root, filename); err == nil || !strings.Contains(err.Error(), "snapshot scope") {
		t.Fatalf("forged scope accepted: %v", err)
	}
}
