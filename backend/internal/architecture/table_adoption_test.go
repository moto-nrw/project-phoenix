package architecture

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// unclassifiedTableRecord is a base-baseline entry recording that the fixture
// source package accesses a table nobody owns yet.
func unclassifiedTableRecord(issue int, table string) string {
	return fmt.Sprintf("{\"scope\":\"production\",\"rule\":\"tables.unclassified\",\"source\":\"example.test/architecture-fixture/source\",\"target\":%q,\"issue\":\"https://github.com/moto-nrw/project-phoenix/issues/%d\"}\n", table, issue)
}

// A reviewed epoch may adopt a table the base baseline tracks as unowned
// when every recorded accessor belongs to the adopting owner (ADR 0015).
func TestCheckReviewedTableAdoption(t *testing.T) {
	t.Parallel()
	for _, reviewed := range []bool{false, true} {
		name := "unchanged epoch"
		if reviewed {
			name = "reviewed epoch"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef := ratchetRepository(t, legacyRecord(2583)+unclassifiedTableRecord(2707, "ghost.records"))
			policy := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
				if reviewed {
					document["policy_epoch"] = document["policy_epoch"].(float64) + 1
				}
				document["data_objects"] = []any{map[string]any{"name": "ghost.records", "write_owner": "module"}}
			})
			writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)
			writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecord(2583))
			output, err := runRepositoryCheck(t, repo, baseRef)
			if reviewed {
				if err != nil {
					t.Fatalf("reviewed adoption of recorded unowned table rejected: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(output, "data object ghost.records was newly assigned to owner module") {
				t.Fatalf("unreviewed adoption was not rejected: %v\n%s", err, output)
			}
		})
	}
}

// The adoption path stays narrow: no recorded debt, a foreign accessor, and
// a transfer of an owned table are still loosenings under a reviewed epoch.
func TestCheckReviewedTableAdoptionPreservesGuards(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name     string
		baseline string
		mutate   func(map[string]any)
		want     string
	}{
		{
			name:     "no recorded debt",
			baseline: legacyRecord(2583),
			mutate:   func(map[string]any) {},
			want:     "data object ghost.records was newly assigned to owner module",
		},
		{
			name:     "foreign accessor",
			baseline: legacyRecord(2583) + unclassifiedTableRecord(2707, "ghost.records"),
			mutate: func(document map[string]any) {
				document["owners"] = append(document["owners"].([]any), map[string]any{"id": "other", "kind": "domain"})
				for _, item := range document["packages"].([]any) {
					pkg := item.(map[string]any)
					if pkg["path"] == "source" {
						pkg["owner"] = "other"
					}
				}
			},
			want: "data object ghost.records was newly assigned to owner module",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef := ratchetRepository(t, scenario.baseline)
			policy := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
				document["policy_epoch"] = document["policy_epoch"].(float64) + 1
				document["data_objects"] = []any{map[string]any{"name": "ghost.records", "write_owner": "module"}}
				scenario.mutate(document)
			})
			writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)
			writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecord(2583))
			output, err := runRepositoryCheck(t, repo, baseRef)
			if err == nil || !strings.Contains(output, scenario.want) {
				t.Fatalf("%s was accepted: %v\n%s", scenario.name, err, output)
			}
		})
	}
}

// An owned table cannot change hands through the adoption path even when
// the baseline still carries a stale unclassified record for it.
func TestCheckReviewedTableAdoptionRejectsTransfer(t *testing.T) {
	t.Parallel()
	base := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
		document["owners"] = append(document["owners"].([]any), map[string]any{"id": "other", "kind": "domain"})
		document["data_objects"] = []any{map[string]any{"name": "ghost.records", "write_owner": "other"}}
	})
	repo, baseRef := ratchetRepositoryWithPolicy(t, legacyRecord(2583)+unclassifiedTableRecord(2707, "ghost.records"), base)
	candidate := mutatePolicy(t, base, func(document map[string]any) {
		document["policy_epoch"] = document["policy_epoch"].(float64) + 1
		document["data_objects"] = []any{map[string]any{"name": "ghost.records", "write_owner": "module"}}
	})
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), candidate)
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecord(2583))
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "data object ghost.records changed write owner from other to module") {
		t.Fatalf("ownership transfer was accepted: %v\n%s", err, output)
	}
}
