package test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestServiceRepositoryRatchet enforces .claude/rules/backend-conventions.md
// Rule 11: services MUST NOT construct database queries directly on a
// bun.DB / bun.IDB / bun.Tx handle — all data access goes through
// repositories. The violation count regrew from 12 (issue #585, Jan 2026) to
// 71 because detection was docs-only; this test makes the count shrink-only.
//
// serviceQueryRatchetAllowlist holds the per-file count of currently tolerated
// query-construction sites in backend/services/. The rules:
//
//   - A file NOT in the allowlist must have ZERO hits. Put the query in a
//     repository under database/repositories/ instead.
//   - A file may never exceed its allowed count.
//   - When refactoring removes hits, the test fails until the allowlist entry
//     is lowered/removed — the ratchet only turns one way. Never raise a number.
//
// Deliberately-kept transaction-control statements (SAVEPOINT / ROLLBACK TO /
// RELEASE via ExecContext, pg_advisory_xact_lock, LOCK TABLE) count toward a
// file's allowance: transaction orchestration is legitimately service-layer,
// but new ones still deserve the review friction of touching this list.
var serviceQueryRatchetAllowlist = map[string]int{
	// Deliberately-kept transaction control (SAVEPOINT / advisory locks /
	// LOCK TABLE) — transaction orchestration is service-layer per Rule 11.
	"services/active/session_service.go":              4, // advisory lock + savepoints for best-effort DB side effects
	"services/enrollment/rejected_cleanup_service.go": 4, // savepoint isolates cleanup inside the scheduler's ambient tenant transaction
	"services/import/student_import_config.go":        3,
	"services/users/caregiver_capability.go":          1,
}

var (
	// Query builder construction on a db/tx handle.
	queryBuilderPattern = regexp.MustCompile(`\.New(Select|Update|Insert|Delete|Raw)\(`)
	// Driver-level statement execution on a db/tx handle — both the
	// context-taking and context-free variants (Exec/Query/QueryRow compile
	// fine on bun.DB too). The trailing ([^)]|$) requires an argument (or a
	// multi-line call continuing past the line end) so the no-arg
	// `r.URL.Query()` accessor all over api/ does not match.
	driverExecPattern = regexp.MustCompile(`\.(Exec|Query|QueryRow)(Context)?\(([^)]|$)`)
)

func TestServiceRepositoryRatchet(t *testing.T) {
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	counts, err := scanServiceQueryConstruction(backendRoot)
	if err != nil {
		t.Fatalf("scanning services/ failed: %v", err)
	}

	var violations []string

	for file, got := range counts {
		allowed, ok := serviceQueryRatchetAllowlist[file]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"%s: %d query-construction site(s), but the file is not in the allowlist.\n"+
					"  New direct queries in services/ are forbidden (Rule 11) — move the query\n"+
					"  into a repository under database/repositories/.", file, got))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"%s: %d query-construction site(s), allowed %d.\n"+
					"  New direct queries in services/ are forbidden (Rule 11) — move the query\n"+
					"  into a repository under database/repositories/. Never raise the allowlist.",
				file, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"%s: %d query-construction site(s), allowlist says %d.\n"+
					"  Nice — sites were removed. Ratchet down: lower (or delete) the entry in\n"+
					"  serviceQueryRatchetAllowlist so the improvement cannot regress.",
				file, got, allowed))
		}
	}

	for file, allowed := range serviceQueryRatchetAllowlist {
		if _, ok := counts[file]; !ok {
			violations = append(violations, fmt.Sprintf(
				"%s: allowlisted with %d site(s) but has none (deleted or cleaned up?).\n"+
					"  Remove the entry from serviceQueryRatchetAllowlist.", file, allowed))
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Service→Repository ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// scanServiceQueryConstruction counts query-construction sites per file under
// backend/services/, skipping test files and line comments.
func scanServiceQueryConstruction(backendRoot string) (map[string]int, error) {
	counts := make(map[string]int)
	servicesDir := filepath.Join(backendRoot, "services")

	err := filepath.WalkDir(servicesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		hits, err := countQueryConstructionSites(path)
		if err != nil {
			return err
		}
		if hits > 0 {
			rel, err := filepath.Rel(backendRoot, path)
			if err != nil {
				return err
			}
			counts[filepath.ToSlash(rel)] = hits
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return counts, nil
}

func countQueryConstructionSites(path string) (int, error) {
	f, err := os.Open(path) // #nosec G304 -- test scans repo-local source files
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	hits := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		hits += len(queryBuilderPattern.FindAllString(line, -1))
		hits += len(driverExecPattern.FindAllString(line, -1))
	}
	return hits, scanner.Err()
}
