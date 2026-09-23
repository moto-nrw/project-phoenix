package test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestModuleFileSizeRatchet caps the size of production files under modules/
// and workflows/ at 800 lines. The existing quality gates in this package all
// stop at the api/ and models/ directory boundaries, so the 228k production
// lines that migration #2580 moved into modules/ grew unwatched: 55 files are
// over the cap today and the largest, instance_service.go, holds 3194 lines.
// A file that big has no discoverable seam — nobody can review it, split it
// safely, or find the one function that matters in it.
//
// This test does not ask anyone to fix those files today. It freezes them at
// their current size so the number can only fall.
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
// Scope: every non-test .go file under modules/ and workflows/, including the
// legacy/ subtrees — those hold 19 of the biggest files in the backend and are
// the part of the problem most in need of a floor. Keys are repo-relative
// slash paths starting at "modules/" or "workflows/".
//
// A line is a newline, exactly as `wc -l` counts one.
const moduleFileSizeThreshold = 800

// moduleFileSizeTrees are the two subtrees this ratchet watches, relative to
// the backend module root.
var moduleFileSizeTrees = []string{"modules", "workflows"}

// moduleFileSizeAllowlist is the frozen 2026-09-18 baseline at merge commit
// ecf0003369: the 55 non-test files over the threshold, out of 1655 scanned
// files holding 332256 lines. The first seed was taken at 19feca2822 and
// re-measured here after the merge of origin/development (a47f77b2c3), which
// brought in PR #3408 (issue #3350) and with it three over-cap files that
// moved into modules/ from services/users and database/repositories/users.
// Shrink-only. Reproduce the measurement with:
//
//	cd backend && find modules workflows -name '*.go' ! -name '*_test.go' \
//	  -exec wc -l {} + | awk '$1 > 800 {print $2, $1}' | sort
var moduleFileSizeAllowlist = map[string]int{
	"modules/appointments/appointments.go":                      846,
	"modules/appointments/internal/adapters/postgres/store.go":  1011,
	"modules/careplan/internal/adapters/postgres/requests.go":   847,
	"modules/careplan/internal/application/excused_requests.go": 1567,
	// Moved in by PR #3408 (#3350) from services/users and
	// database/repositories/users; the size crossed the boundary with them.
	"modules/classday/internal/application/slotlists.go":                             2638,
	"modules/communication/internal/adapters/parentaudience/projection.go":           950,
	"modules/communication/internal/adapters/parentpostgres/parent_announcements.go": 950,
	"modules/communication/internal/staffannouncements/service.go":                   1256,
	"modules/enrollment/form_schema.go":                                              1015,
	"modules/grouplive/grouplive.go":                                                 815,
	"modules/identityaccess/compose/account_lifecycle.go":                            890,
	"modules/organizationtenancy/inbound/operator/provisioning.go":                   899,
	"modules/peopledirectory/http/guardian_handlers.go":                              1025,
	"modules/schoolcalendar/portal/internal/application/service.go":                  2000,
	"modules/timetable/compose/httpadapter/schedules.go":                             1090,
	"modules/timetable/compose/new.go":                                               1192,
	"modules/timetable/legacy/timetableplanning/deviation_apply.go":                  1534,
	"modules/timetable/legacy/timetableplanning/instance_service.go":                 3145,
	"modules/timetable/legacy/timetableplanning/materialization_service.go":          1118,
	"modules/timetable/legacy/timetableplanning/template_split_service.go":           1880,
	"modules/timetable/legacy/timetableplanning/template_update_service.go":          1135,
	"modules/timetable/legacy/timetableplanning/timetable_data_service.go":           844,
	"modules/timetable/legacy/timetableplanning/timetable_operations_service.go":     2047,
	"modules/timetable/timetable.go":                                                 1775,
	"modules/workforce/inbound/timetracking/api.go":                                  919,
	"modules/workforce/internal/adapters/postgres/shift_store.go":                    883,
	"modules/workforce/internal/adapters/postgres/worksession_store.go":              1148,
	"modules/workforce/legacy/timetracking/staff_absence_service.go":                 2132,
	"modules/workforce/legacy/timetracking/work_session_service.go":                  3093,
	"modules/workforce/legacy/timetracking/work_time_month_service.go":               1846,
	"modules/workforce/legacy/worksession_repositories.go":                           1158,
	"modules/workforce/timetracking.go":                                              824,
	"workflows/reminderdelivery/internal/application/service.go":                     864,
	"workflows/studentdeletion/deletion.go":                                          811,
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
// modules/ and workflows/ that exceeds the threshold, keyed by repo-relative
// slash path. Files at or below the threshold are omitted, so a file dropping
// under it reads as a stale allowlist entry.
func moduleFileSizeScan(backendRoot string) (map[string]int, error) {
	sizes := make(map[string]int)
	for _, tree := range moduleFileSizeTrees {
		root := filepath.Join(backendRoot, tree)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)

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
