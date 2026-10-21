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

// An owned table cannot change hands under a reviewed epoch even when the
// baseline still carries a stale unclassified record for it.
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
