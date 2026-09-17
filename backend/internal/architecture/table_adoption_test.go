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

// ghostRecordsAccessor replaces the fixture source package with one that
// still writes the recorded table through raw SQL.
const ghostRecordsAccessor = `package source

import (
	"context"
	"database/sql"

	"example.test/architecture-fixture/target"
)

func Use() string {
	return target.Value
}

func Purge(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "DELETE FROM ghost.records")
	return err
}
`

// sourceClassifiedAs moves the fixture source package to a second owner with
// the postgres role, so direct SQL is permitted and only ownership decides.
func sourceClassifiedAs(document map[string]any, owner string) {
	document["owners"] = append(document["owners"].([]any), map[string]any{"id": owner, "kind": "domain"})
	document["roles"] = append(document["roles"].([]any), "postgres")
	for _, item := range document["packages"].([]any) {
		pkg := item.(map[string]any)
		if pkg["path"] == "source" {
			pkg["owner"], pkg["role"] = owner, "postgres"
		}
	}
}

// A package of another owner that the base recorded as an accessor does not
// block the adoption once the candidate proves it no longer touches the
// table: the recorded access moved to the adopting owner (ADR 0015).
func TestCheckReviewedTableAdoptionAcceptsRetiredForeignAccessor(t *testing.T) {
	t.Parallel()
	base := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
		sourceClassifiedAs(document, "other")
	})
	repo, baseRef := ratchetRepositoryWithPolicy(t, legacyRecord(2583)+unclassifiedTableRecord(2707, "ghost.records"), base)
	candidate := mutatePolicy(t, base, func(document map[string]any) {
		document["policy_epoch"] = document["policy_epoch"].(float64) + 1
		document["data_objects"] = []any{map[string]any{"name": "ghost.records", "write_owner": "module"}}
	})
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), candidate)
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecord(2583))
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err != nil {
		t.Fatalf("adoption after the foreign accessor stopped touching the table was rejected: %v\n%s", err, output)
	}
}

// The adoption path stays narrow: no recorded debt, a foreign accessor that
// still touches the table, and a transfer of an owned table are still
// loosenings under a reviewed epoch.
func TestCheckReviewedTableAdoptionPreservesGuards(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name     string
		baseline string
		source   string
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
			source:   ghostRecordsAccessor,
			mutate: func(document map[string]any) {
				sourceClassifiedAs(document, "other")
			},
			want: "data object ghost.records was newly assigned to owner module",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef := ratchetRepository(t, scenario.baseline)
			if scenario.source != "" {
				writeFile(t, filepath.Join(repo, "source", "source.go"), scenario.source)
			}
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
