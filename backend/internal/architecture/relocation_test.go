package architecture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const movedTargetPath = "moved/target"

// relocatedRepository builds a candidate that moves the fixture's target
// package to moved/target: the Go files move, the importing package follows,
// the policy classifies the new path with the old owner and roles, the epoch
// increases and the declaration names the move. Its baseline records the one
// pre-existing debt entry at the new path. Callers break exactly one of those
// conditions per test.
func relocatedRepository(t *testing.T, mutate func(document map[string]any)) (string, string) {
	t.Helper()
	repo, baseRef := ratchetRepository(t, legacyRecord(2743))
	moveFixtureTarget(t, repo)
	writeFile(t, filepath.Join(repo, "source", "source.go"), "package source\n\nimport \"example.test/architecture-fixture/"+movedTargetPath+"\"\n\nfunc Use() string {\n\treturn target.Value\n}\n")
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), relocatedPolicy(t, repo, mutate))
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecordWithTarget(2743, "example.test/architecture-fixture/"+movedTargetPath))
	// A relocation reaches CI as tracked files, so the package-addition scan
	// sees the new path the way it sees any other added package.
	runGit(t, repo, "add", "-A")
	return repo, baseRef
}

func moveFixtureTarget(t *testing.T, repo string) {
	t.Helper()
	moved := filepath.Join(repo, filepath.FromSlash(movedTargetPath))
	if err := os.MkdirAll(moved, 0o755); err != nil {
		t.Fatalf("create relocated package: %v", err)
	}
	if err := os.Rename(filepath.Join(repo, "target", "target.go"), filepath.Join(moved, "target.go")); err != nil {
		t.Fatalf("move fixture package: %v", err)
	}
	if err := os.Remove(filepath.Join(repo, "target")); err != nil {
		t.Fatalf("remove empty package directory: %v", err)
	}
}

func relocatedPolicy(t *testing.T, repo string, mutate func(document map[string]any)) string {
	t.Helper()
	return mutatePolicy(t, readFile(t, filepath.Join(repo, "architecture", "policy.json")), func(document map[string]any) {
		document["policy_epoch"] = float64(2)
		for _, value := range document["packages"].([]any) {
			if pkg := value.(map[string]any); pkg["path"] == "target" {
				pkg["path"] = movedTargetPath
			}
		}
		document["relocations"] = []any{map[string]any{
			"from": "target", "to": movedTargetPath,
			"issue": "https://github.com/moto-nrw/project-phoenix/issues/3226",
		}}
		if mutate != nil {
			mutate(document)
		}
	})
}

func relocatedPackage(document map[string]any) map[string]any {
	for _, value := range document["packages"].([]any) {
		if pkg := value.(map[string]any); pkg["path"] == movedTargetPath {
			return pkg
		}
	}
	return nil
}

// A pure relocation keeps its debt. The consumer's entry is the one the base
// recorded, read at the moved package's path, with its migration issue intact.
func TestCheckKeepsDebtAcrossDeclaredRelocation(t *testing.T) {
	t.Parallel()
	repo, baseRef := relocatedRepository(t, nil)
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err != nil || !strings.Contains(output, "1 legacy violation(s) remain") {
		t.Fatalf("declared relocation lost its debt: %v\n%s", err, output)
	}
}

// The moved package's own debt re-keys the same way, so a relocation carries
// both directions of every entry it appears in.
func TestRelocationRekeysTheMovedPackagesOwnDebt(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepositoryWithPolicy(t, "", readFile(t, fixturePath(t, "vertical-forbidden.json")))
	writeFile(t, filepath.Join(repo, "target", "target.go"), "package target\n\nimport _ \"example.test/architecture-fixture/linuxonly\"\n\nconst Value = \"target\"\n")
	outgoing := fmt.Sprintf("{\"scope\":\"production\",\"rule\":\"imports.forbidden\",\"source\":%q,\"target\":\"example.test/architecture-fixture/linuxonly\",\"issue\":\"https://github.com/moto-nrw/project-phoenix/issues/2743\"}\n", "example.test/architecture-fixture/target")
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecord(2743)+outgoing)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "target imports linuxonly")
	baseRef := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	moveFixtureTarget(t, repo)
	writeFile(t, filepath.Join(repo, "source", "source.go"), "package source\n\nimport \"example.test/architecture-fixture/"+movedTargetPath+"\"\n\nfunc Use() string {\n\treturn target.Value\n}\n")
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), relocatedPolicy(t, repo, nil))
	moved := "example.test/architecture-fixture/" + movedTargetPath
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"),
		fmt.Sprintf("{\"scope\":\"production\",\"rule\":\"imports.forbidden\",\"source\":%q,\"target\":\"example.test/architecture-fixture/linuxonly\",\"issue\":\"https://github.com/moto-nrw/project-phoenix/issues/2743\"}\n", moved)+
			legacyRecordWithTarget(2743, moved))
	runGit(t, repo, "add", "-A")
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err != nil || !strings.Contains(output, "2 legacy violation(s) remain") {
		t.Fatalf("relocation dropped the moved package's own debt: %v\n%s", err, output)
	}
}

// A relocation renames keys. It may not smuggle a dependency in with them.
func TestRelocationRejectsAnAddedImport(t *testing.T) {
	t.Parallel()
	repo, baseRef := relocatedRepository(t, nil)
	writeFile(t, filepath.Join(repo, filepath.FromSlash(movedTargetPath), "target.go"), "package target\n\nimport _ \"example.test/architecture-fixture/linuxonly\"\n\nconst Value = \"target\"\n")
	moved := "example.test/architecture-fixture/" + movedTargetPath
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"),
		fmt.Sprintf("{\"scope\":\"production\",\"rule\":\"imports.forbidden\",\"source\":%q,\"target\":\"example.test/architecture-fixture/linuxonly\",\"issue\":\"https://github.com/moto-nrw/project-phoenix/issues/2743\"}\n", moved)+
			legacyRecordWithTarget(2743, moved))
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "absent from the base baseline") {
		t.Fatalf("relocation accepted a new import as old debt: %v\n%s", err, output)
	}
}

// Owner and role changes are ownership decisions, not renames.
func TestRelocationRejectsClassificationChanges(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, want string
		mutate     func(document map[string]any)
	}{
		{
			name: "owner", want: "changed owner from module to other",
			mutate: func(document map[string]any) {
				document["owners"] = append(document["owners"].([]any), map[string]any{"id": "other", "kind": "domain"})
				relocatedPackage(document)["owner"] = "other"
			},
		},
		{
			name: "production role", want: "changed production role from domain to application",
			mutate: func(document map[string]any) { relocatedPackage(document)["role"] = "application" },
		},
		{
			name: "internal_test role", want: "changed internal_test role from module-internal-test to domain",
			mutate: func(document map[string]any) { relocatedPackage(document)["internal_test_role"] = "domain" },
		},
		{
			name: "external_test role", want: "changed external_test role from module-behavior-test to domain",
			mutate: func(document map[string]any) { relocatedPackage(document)["external_test_role"] = "domain" },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef := relocatedRepository(t, tt.mutate)
			output, err := runRepositoryCheck(t, repo, baseRef)
			if err == nil || !strings.Contains(output, tt.want) {
				t.Fatalf("relocation accepted a reclassification: %v\n%s", err, output)
			}
		})
	}
}

// The migration issue identifies who owns the debt. A rename may not move it.
func TestRelocationRejectsIssueReassignment(t *testing.T) {
	t.Parallel()
	repo, baseRef := relocatedRepository(t, nil)
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecordWithTarget(2750, "example.test/architecture-fixture/"+movedTargetPath))
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "changed migration issue") {
		t.Fatalf("relocation accepted a reassigned issue: %v\n%s", err, output)
	}
}

// Like every reviewed mechanism, a relocation needs an epoch a reviewer raised.
func TestRelocationRequiresAReviewedEpoch(t *testing.T) {
	t.Parallel()
	repo, baseRef := relocatedRepository(t, func(document map[string]any) { document["policy_epoch"] = float64(1) })
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "requires a reviewed policy epoch above 1") {
		t.Fatalf("relocation ran without a reviewed epoch: %v\n%s", err, output)
	}
}

// The rename is proven from Git, not from the declaration: a target that does
// not hold exactly the source's files is a rewrite and keeps the ordinary
// guards.
func TestRelocationRequiresTheSameFiles(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, want string
		change     func(t *testing.T, repo string)
	}{
		{
			name: "added file", want: "added extra.go",
			change: func(t *testing.T, repo string) {
				writeFile(t, filepath.Join(repo, filepath.FromSlash(movedTargetPath), "extra.go"), "package target\n\nconst Extra = \"extra\"\n")
			},
		},
		{
			name: "renamed file", want: "missing target.go; added value.go",
			change: func(t *testing.T, repo string) {
				moved := filepath.Join(repo, filepath.FromSlash(movedTargetPath))
				if err := os.Rename(filepath.Join(moved, "target.go"), filepath.Join(moved, "value.go")); err != nil {
					t.Fatalf("rename relocated file: %v", err)
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef := relocatedRepository(t, nil)
			tt.change(t, repo)
			output, err := runRepositoryCheck(t, repo, baseRef)
			if err == nil || !strings.Contains(output, "is not a rename: "+tt.want) {
				t.Fatalf("relocation accepted a rewritten package: %v\n%s", err, output)
			}
		})
	}
}

// A declaration whose source path is already gone at the base is history. It
// must not re-apply, or a later epoch could replay an old rename over entries
// that have nothing to do with it.
func TestRecordedRelocationIsInertOnceMerged(t *testing.T) {
	t.Parallel()
	repo, _ := relocatedRepository(t, nil)
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-qm", "relocate target")
	baseRef := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), mutatePolicy(t, readFile(t, filepath.Join(repo, "architecture", "policy.json")), func(document map[string]any) {
		document["policy_epoch"] = float64(3)
	}))
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err != nil || !strings.Contains(output, "1 legacy violation(s) remain") {
		t.Fatalf("merged relocation did not settle: %v\n%s", err, output)
	}
}

// The declaration itself is validated before any comparison runs.
func TestPolicyRejectsInvalidRelocations(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, want  string
		relocations []any
	}{
		{
			name: "source still classified", want: `relocated package "source" is still classified`,
			relocations: []any{map[string]any{"from": "source", "to": movedTargetPath, "issue": relocationIssue}},
		},
		{
			name: "unclassified target", want: `relocation target "moved/elsewhere" has no owner or role`,
			relocations: []any{map[string]any{"from": "target", "to": "moved/elsewhere", "issue": relocationIssue}},
		},
		{
			name: "unsorted", want: `relocation "alpha" is not sorted by from path`,
			relocations: []any{
				map[string]any{"from": "target", "to": movedTargetPath, "issue": relocationIssue},
				map[string]any{"from": "alpha", "to": "moved/alpha", "issue": relocationIssue},
			},
		},
		{
			name: "no move", want: `relocation "target" does not move the package`,
			relocations: []any{map[string]any{"from": "target", "to": "target", "issue": relocationIssue}},
		},
		{
			name: "issue", want: "must be an exact https://github.com",
			relocations: []any{map[string]any{"from": "target", "to": movedTargetPath, "issue": "3226"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			policy := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
				for _, value := range document["packages"].([]any) {
					if pkg := value.(map[string]any); pkg["path"] == "target" {
						pkg["path"] = movedTargetPath
					}
				}
				document["relocations"] = tt.relocations
			})
			path := filepath.Join(t.TempDir(), "policy.json")
			writeFile(t, path, policy)
			if _, err := LoadPolicy(path); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("invalid relocation accepted: %v", err)
			}
		})
	}
}

const relocationIssue = "https://github.com/moto-nrw/project-phoenix/issues/3226"
