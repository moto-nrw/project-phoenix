package artifacts

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnerFragmentsComposeCompletePolicy(t *testing.T) {
	t.Parallel()
	source := map[string]string{
		"policy.json":       `{"schema_version":5,"storage":"owner-fragments-v1"}`,
		"policy/alpha.json": `{"packages":[{"path":"alpha","owner":"alpha"}],"data_objects":[{"name":"alpha.records","write_owner":"alpha"}],"rules":[{"id":"alpha.read","source_owner":"alpha"}]}`,
		"policy/beta.json":  `{"packages":[{"path":"beta","owner":"beta"}],"read_projections":[{"id":"beta-view","package":"beta"}]}`,
	}
	data, err := ReadPolicyFS(testArtifactFS(t, source), "policy.json")
	if err != nil {
		t.Fatal(err)
	}
	var policy map[string]json.RawMessage
	if err := json.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}
	if _, exists := policy["storage"]; exists {
		t.Fatal("storage marker leaked into semantic policy")
	}
	for section, expected := range map[string]int{"packages": 2, "data_objects": 1, "rules": 1, "read_projections": 1} {
		var records []json.RawMessage
		if err := json.Unmarshal(policy[section], &records); err != nil || len(records) != expected {
			t.Fatalf("%s: got %s, error %v", section, policy[section], err)
		}
	}
}

func TestMalformedFragmentsNeverBecomeEmptyPolicy(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, header, alpha, beta, want string }{
		{"missing", `{"storage":"owner-fragments-v1"}`, "", "", "policy fragments"},
		{"mixed", `{"storage":"owner-fragments-v1","packages":[]}`, "", "", "mixed policy storage"},
		{"unknown format", `{"storage":"anything"}`, "", "", "unsupported policy storage"},
		{"misplaced", `{"storage":"owner-fragments-v1"}`, `{"packages":[{"path":"alpha","owner":"beta"}]}`, "", "cannot own"},
		{"duplicate", `{"storage":"owner-fragments-v1"}`, `{"packages":[{"path":"same","owner":"alpha"}]}`, `{"packages":[{"path":"same","owner":"beta"}]}`, "duplicate packages identity"},
		{"unknown section", `{"storage":"owner-fragments-v1"}`, `{"permissions":[]}`, "", "unknown policy fragment section"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := map[string]string{"policy.json": test.header}
			if test.alpha != "" {
				source["policy/alpha.json"] = test.alpha
			}
			if test.beta != "" {
				source["policy/beta.json"] = test.beta
			}
			if _, err := ReadPolicyFS(testArtifactFS(t, source), "policy.json"); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want %q, got %v", test.want, err)
			}
		})
	}
}

func TestHistoricalMaterializedPolicyNeedsNoFragments(t *testing.T) {
	t.Parallel()
	want := `{"schema_version":4,"packages":[{"path":"historical"}]}`
	got, err := ReadPolicyFS(testArtifactFS(t, map[string]string{"policy.json": want}), "policy.json")
	if err != nil || string(got) != want {
		t.Fatalf("historical policy changed: %s, %v", got, err)
	}
}

func testArtifactFS(t *testing.T, files map[string]string) fs.FS {
	t.Helper()
	root := t.TempDir()
	for name, data := range files {
		filename := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return os.DirFS(root)
}

func TestPolicyRejectsDirectorySymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "policy.json"), []byte(`{"storage":"owner-fragments-v1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "policy-source"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "policy-source", "alpha.json"), []byte(`{"packages":[{"path":"alpha","owner":"alpha"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("policy-source", filepath.Join(root, "policy")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPolicy(filepath.Join(root, "policy.json")); err == nil {
		t.Fatal("candidate policy accepts directory symlink")
	}
}
