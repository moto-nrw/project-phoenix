package test

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// TestModuleFileSizeRatchet caps the size of production files under modules/
// and workflows/, roots (ADR 0082), and every backend tree (ADR 0087)
// at 800 lines. No gate measured file
// size there when migration #2580 moved 228k production lines into modules/: on
// 2026-09-18, 55 files were over the cap and the largest,
// instance_service.go, held 3194 lines. A file that big has no discoverable
// seam — nobody can review it, split it safely, or find the one function that
// matters in it.
//
// The original register drained to zero. ADR 0082 adds the measured root
// baseline, still without headroom.
//
// Allowlist semantics (per file, like handler_complexity_ratchet_test.go, but
// counting lines rather than gocognit scores):
//
//   - A file NOT in the allowlist must stay at or below 800 lines.
//   - An allowlisted file may never exceed its recorded line count.
//   - When a split or deletion lowers a count (or drops the file to ≤ 800),
//     the test fails until the entry is lowered/removed — the ratchet only
//     turns one way. Never raise a number.
//
// Scope: walkModuleRatchetSources covers all backend trees recursively,
// including legacy, migrations, tools, generated code, hooks and test support.
// Only _test.go and hidden directories are excluded. ADR 0087 records the
// category audit and exact additions measured at 8aca1925f7. Keys are slash
// paths relative to backend/.
//
// A line is a newline, exactly as `wc -l` counts one.
const moduleFileSizeThreshold = 800

// moduleFileSizeAllowlist is the frozen 2026-09-18 baseline at merge commit
// ecf0003369: the 55 non-test files over the threshold, out of 1655 scanned
// files holding 332256 lines. The first seed was taken at 19feca2822 and
// re-measured here after the merge of origin/development (a47f77b2c3), which
// brought in PR #3408 (issue #3350) and with it three over-cap files that
// moved into modules/ from services/users and database/repositories/users.
// Shrink-only. Reproduce the measurement with:
//
//	cd backend && find modules workflows -name '*.go' ! -name '*_test.go' \
//	  -exec wc -l {} + | awk '$1 > 800 && $2 != "total" {print $2, $1}' | sort
//
// ADR 0082: newly covered sources, measured at 722c5aa147; no headroom.
var moduleFileSizeAllowlist = map[string]int{
	"cmd/cleanup.go": 1139,
	// ADR 0087.
	"database/migrations/request_child_storage_backfill.go": 1186,
	// ADR 0087.
	"database/migrations/staff_owner_backfill.go": 868,
	// ADR 0087.
	"database/migrations/student_owner_backfill.go": 850,
	// ADR 0087.
	"internal/architecture/semantic.go": 1480,
	// ADR 0087.
	"internal/devtools/seedapi/parent_enrollment.go": 1440,
	// ADR 0087.
	"internal/devtools/seedapi/seed_time_tracking.go": 849,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go": 1477,
	// ADR 0087.
	"internal/devtools/simulate/demo_day.go": 818,
	// ADR 0087.
	"internal/devtools/simulate/fullday.go": 921,
	// ADR 0087.
	"observability/prometheus.go": 1722,
	// ADR 0087.
	"test/fixtures.go": 3201,
}

func TestModuleFileSizeRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	sizes, err := moduleFileSizeScan(backendRoot)
	if err != nil {
		t.Fatalf("file-size scan failed: %v", err)
	}

	violations := moduleFileSizeViolations(sizes, moduleFileSizeAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module file-size ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleFileSizeViolations compares the measured line counts of files over the
// threshold against the allowlist. It deliberately does not reuse
// complexityViolations (its messages name gocognit) or ratchetViolations
// (its entries are pattern hit counts, where zero hits is the goal — here the
// goal is a count at or below the threshold, at which point the file drops out
// of the scan entirely).
func moduleFileSizeViolations(sizes, allowlist map[string]int) []string {
	var violations []string
	for file, got := range sizes {
		allowed, ok := allowlist[file]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[file-size] %s has %d lines (limit %d), and is not in the allowlist.\n  Split the file along a real seam — a new file over the limit does not get an entry.",
				file, got, moduleFileSizeThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[file-size] %s has %d lines, allowed %d.\n  The file grew. Move the addition into its own file — never raise the allowlist.",
				file, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[file-size] %s has %d lines, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.",
				file, got, allowed, got))
		}
	}
	for file, allowed := range allowlist {
		if _, ok := sizes[file]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[file-size] %s is allowlisted at %d lines but now has ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				file, allowed, moduleFileSizeThreshold))
		}
	}
	return violations
}

// moduleFileSizeScan returns the line count of every non-test .go file under
// the shared backend source scope that exceeds the threshold, keyed by
// backend-relative slash path. Files at or below the threshold are omitted, so a file dropping
// under it reads as a stale allowlist entry.
func moduleFileSizeScan(backendRoot string) (map[string]int, error) {
	sizes := make(map[string]int)
	err := walkModuleRatchetSources(backendRoot, func(path, rel string) error {
		lines, countErr := moduleFileSizeCountLines(path)
		if countErr != nil {
			return fmt.Errorf("count %s: %w", rel, countErr)
		}
		if lines > moduleFileSizeThreshold {
			sizes[rel] = lines
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sizes, nil
}

// moduleFileSizeCountLines counts newlines in a file, matching `wc -l`: a
// trailing fragment without a newline is not counted, and gofmt guarantees
// every Go source file ends with one.
func moduleFileSizeCountLines(path string) (int, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- test reads repo-local source files
	if err != nil {
		return 0, err
	}
	return bytes.Count(data, []byte{'\n'}), nil
}
