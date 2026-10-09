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

// ghostRecordsDynamicAccessor replaces the fixture source package with one
// whose table expression the analyser cannot resolve, so no finding can say
// which table it touches.
const ghostRecordsDynamicAccessor = `package source

import (
	"context"
	"database/sql"

	"example.test/architecture-fixture/target"
)

func Use() string {
	return target.Value
}

func Purge(ctx context.Context, db *sql.DB, table string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM "+table)
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
		{
			// An accessor whose SQL the analyser cannot resolve proves
			// nothing: it may still be reaching the adopted table.
			name:     "unresolved accessor",
			baseline: legacyRecord(2583) + unclassifiedTableRecord(2707, "ghost.records"),
			source:   ghostRecordsDynamicAccessor,
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

// ghostRecordsRetiredAccessor is a generic store whose table arrives at run
// time, so the base can only record it as tables.unresolved debt (ADR 0045).
const ghostRecordsRetiredAccessor = `package retired

import (
	"context"
	"database/sql"
)

func Purge(ctx context.Context, db *sql.DB, table string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM "+table)
	return err
}
`

// ghostRecordsStaticStore names the table statically, the way the adopting
// owner's own adapter replaces the retired generic store.
const ghostRecordsStaticStore = `package store

import (
	"context"
	"database/sql"
)

func Purge(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "DELETE FROM ghost.records")
	return err
}
`

// unresolvedTableRecord is a base-baseline entry recording that a package
// reaches some table through an expression the analysis cannot resolve.
func unresolvedTableRecord(issue int, pkg string) string {
	return fmt.Sprintf("{\"scope\":\"production\",\"rule\":\"tables.unresolved\",\"source\":\"example.test/architecture-fixture/%s\",\"target\":\"%s.ExecContext\",\"issue\":\"https://github.com/moto-nrw/project-phoenix/issues/%d\"}\n", pkg, pkg, issue)
}

// classifyPostgres adds a postgres package of the given owner, adding the
// owner and the role when the fixture does not have them yet.
func classifyPostgres(document map[string]any, path, owner string) {
	if owner != "module" && !fixtureHasOwner(document, owner) {
		document["owners"] = append(document["owners"].([]any), map[string]any{"id": owner, "kind": "domain"})
	}
	if !containsJSONValue(document["roles"].([]any), "postgres") {
		document["roles"] = append(document["roles"].([]any), "postgres")
	}
	document["packages"] = append(document["packages"].([]any), map[string]any{
		"path": path, "owner": owner, "role": "postgres",
		"internal_test_role": "module-internal-test", "external_test_role": "module-behavior-test",
	})
}

func fixtureHasOwner(document map[string]any, owner string) bool {
	for _, item := range document["owners"].([]any) {
		if item.(map[string]any)["id"] == owner {
			return true
		}
	}
	return false
}

func withoutPackage(document map[string]any, path string) {
	var kept []any
	for _, item := range document["packages"].([]any) {
		if item.(map[string]any)["path"] != path {
			kept = append(kept, item)
		}
	}
	document["packages"] = kept
}

// retiredExpressionScenario describes one candidate against a base whose
// retired package reached an unowned table only through an unresolved
// expression.
type retiredExpressionScenario struct {
	reviewed         bool
	recorded         bool
	keepRetired      bool
	staticStores     map[string]string
	projectionReader bool
}

func runRetiredExpressionScenario(t *testing.T, scenario retiredExpressionScenario) (string, error) {
	t.Helper()
	base := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
		classifyPostgres(document, "retired", "other")
		document["external_classes"] = append(document["external_classes"].([]any), "standard")
		for _, path := range []string{"context", "database/sql"} {
			document["external_packages"] = append(document["external_packages"].([]any), map[string]any{"path": path, "class": "standard"})
		}
		document["rules"] = append(document["rules"].([]any), map[string]any{
			"id": "postgres.to.standard", "description": "Fixture stores use the standard library.", "scopes": []string{"production"},
			"source_role": "postgres", "target_class": "standard",
		})
	})
	baseline := legacyRecord(2583)
	if scenario.recorded {
		baseline += unresolvedTableRecord(2706, "retired")
	}
	repo, _ := ratchetRepositoryWithPolicy(t, baseline, base)
	writeFile(t, filepath.Join(repo, "retired", "retired.go"), ghostRecordsRetiredAccessor)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "retired accessor")
	baseRef := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	candidate := mutatePolicy(t, base, func(document map[string]any) {
		if scenario.reviewed {
			document["policy_epoch"] = document["policy_epoch"].(float64) + 1
		}
		document["data_objects"] = []any{map[string]any{"name": "ghost.records", "write_owner": "module"}}
		if !scenario.keepRetired {
			withoutPackage(document, "retired")
		}
		for path, owner := range scenario.staticStores {
			classifyPostgres(document, path, owner)
		}
		if scenario.projectionReader {
			classifyPostgres(document, "view", "foreign-view")
			for _, item := range document["owners"].([]any) {
				owner := item.(map[string]any)
				if owner["id"] == "foreign-view" {
					owner["kind"] = "projection"
				}
			}
			document["read_projections"] = []any{map[string]any{
				"id": "foreign-view", "package": "view", "data_objects": []string{"ghost.records"}, "tenant_safe": true,
			}}
		}
	})
	candidateBaseline := legacyRecord(2583)
	if scenario.keepRetired {
		if scenario.recorded {
			candidateBaseline += unresolvedTableRecord(2706, "retired")
		}
	} else {
		runGit(t, repo, "rm", "-q", "-r", "retired")
	}
	for path := range scenario.staticStores {
		writeFile(t, filepath.Join(repo, path, path+".go"), strings.Replace(ghostRecordsStaticStore, "package store", "package "+path, 1))
	}
	if scenario.projectionReader {
		writeFile(t, filepath.Join(repo, "view", "view.go"), `package view

import (
	"context"
	"database/sql"
)

func Read(ctx context.Context, db *sql.DB, tenantID int64) error {
	rows, err := db.QueryContext(ctx, "SELECT id FROM ghost.records WHERE tenant_id = $1", tenantID)
	if err != nil {
		return err
	}
	return rows.Close()
}
`)
	}
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), candidate)
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), candidateBaseline)
	runGit(t, repo, "add", ".")
	return runRepositoryCheck(t, repo, baseRef)
}

// A reviewed epoch may adopt an unowned table that the base reached only
// through a retired package's unresolved expression, once the adopting
// owner's own adapter names it statically and nobody else does (ADR 0045).
func TestCheckReviewedTableAdoptionFromRetiredExpression(t *testing.T) {
	t.Parallel()
	output, err := runRetiredExpressionScenario(t, retiredExpressionScenario{
		reviewed: true, recorded: true, staticStores: map[string]string{"store": "module"},
	})
	if err != nil {
		t.Fatalf("adoption of a table the retired generic store reached was rejected: %v\n%s", err, output)
	}
}

// The retired-expression path stays as narrow as the debt path: it needs a
// reviewed epoch, recorded unresolved debt on a package the candidate
// deletes, and static access by the adopting owner alone.
func TestCheckReviewedTableAdoptionFromRetiredExpressionPreservesGuards(t *testing.T) {
	t.Parallel()
	const rejected = "data object ghost.records was newly assigned to owner module"
	for name, scenario := range map[string]retiredExpressionScenario{
		"unchanged epoch": {
			recorded: true, staticStores: map[string]string{"store": "module"},
		},
		"no recorded unresolved debt": {
			reviewed: true, staticStores: map[string]string{"store": "module"},
		},
		"unresolved accessor kept": {
			reviewed: true, recorded: true, keepRetired: true, staticStores: map[string]string{"store": "module"},
		},
		"no static access": {
			reviewed: true, recorded: true,
		},
		"foreign static access": {
			reviewed: true, recorded: true, staticStores: map[string]string{"store": "module", "rival": "other"},
		},
		"foreign projection read": {
			reviewed: true, recorded: true, staticStores: map[string]string{"store": "module"}, projectionReader: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			output, err := runRetiredExpressionScenario(t, scenario)
			if err == nil || !strings.Contains(output, rejected) {
				t.Fatalf("%s was accepted: %v\n%s", name, err, output)
			}
		})
	}
}
