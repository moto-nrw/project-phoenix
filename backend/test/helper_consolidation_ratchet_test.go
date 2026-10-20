package test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestHelperConsolidationRatchet guards the audit-B7 helper consolidations
// against regrowth (report §8.7). Three patterns, all shrink-only:
//
//  1. The PostgreSQL unique-violation code "23505" hard-coded outside
//     modules/transactionruntime/internal/adapters/postgres — use transactionruntime.IsUniqueViolation / transactionruntime.IsUniqueViolationOn.
//  2. Hand-rolled strconv.ParseInt(chi.URLParam(...)) in api/ — use
//     common.ParseInt64IDWithError / ParsePositiveInt64IDWithError.
//  3. Multi-line nil-check getLogger bodies — use cmp.Or(logger, slog.Default()).
//
// Allowlists are shrink-only: never add an entry, never raise a count.
var (
	// Non-comment "23505" references outside modules/transactionruntime/internal/adapters/postgres and migrations.
	uniqueViolationLiteralAllowlist = map[string]int{}

	// strconv.ParseInt(chi.URLParam(...)) sites in api/ outside
	// api/common/request.go (which implements the shared helper).
	handRolledParseIntAllowlist = map[string]int{}
)

var (
	unique23505Pattern    = regexp.MustCompile(`23505`)
	parseIntURLPattern    = regexp.MustCompile(`strconv\.ParseInt\(chi\.URLParam`)
	getLoggerHeadPattern  = regexp.MustCompile(`func \([^)]+\) getLogger\(\) \*slog\.Logger \{`)
	getLoggerNilCheckLine = regexp.MustCompile(`!= nil`)
)

func TestHelperConsolidationRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	var violations []string

	check := func(counts map[string]int, allowlist map[string]int, what, fix string) {
		for file, got := range counts {
			allowed, ok := allowlist[file]
			switch {
			case !ok:
				violations = append(violations, fmt.Sprintf(
					"%s: %d %s site(s), but the file is not in the allowlist.\n  %s", file, got, what, fix))
			case got > allowed:
				violations = append(violations, fmt.Sprintf(
					"%s: %d %s site(s), allowed %d.\n  %s Never raise the allowlist.", file, got, what, allowed, fix))
			case got < allowed:
				violations = append(violations, fmt.Sprintf(
					"%s: %d %s site(s), allowlist says %d.\n  Ratchet down: lower/delete the allowlist entry.", file, got, what, allowed))
			}
		}
		for file, allowed := range allowlist {
			if _, ok := counts[file]; !ok {
				violations = append(violations, fmt.Sprintf(
					"%s: allowlisted with %d %s site(s) but has none. Remove the entry.", file, allowed, what))
			}
		}
	}

	counts23505, err := scanRatchetPattern(backendRoot, ".", unique23505Pattern, func(rel string) bool {
		return strings.HasPrefix(rel, "modules/transactionruntime/internal/adapters/postgres/unique_violation.go") || strings.HasPrefix(rel, "database/migrations/")
	})
	if err != nil {
		t.Fatalf("scanning for 23505 literals failed: %v", err)
	}
	check(counts23505, uniqueViolationLiteralAllowlist, `"23505" literal`,
		"Use transactionruntime.IsUniqueViolation / transactionruntime.IsUniqueViolationOn (modules/transactionruntime/internal/adapters/postgres/unique_violation.go).")

	countsParseInt, err := scanRatchetPattern(backendRoot, "api", parseIntURLPattern, func(rel string) bool {
		return rel == "api/common/request.go"
	})
	if err != nil {
		t.Fatalf("scanning for hand-rolled ParseInt sites failed: %v", err)
	}
	check(countsParseInt, handRolledParseIntAllowlist, "hand-rolled URL-param ParseInt",
		"Use common.ParseInt64IDWithError / common.ParsePositiveInt64IDWithError.")

	multiLineGetLoggers, err := scanMultiLineGetLoggers(backendRoot)
	if err != nil {
		t.Fatalf("scanning for getLogger bodies failed: %v", err)
	}
	for _, file := range multiLineGetLoggers {
		violations = append(violations, fmt.Sprintf(
			"%s: getLogger() carries a nil-check body.\n  Use `return cmp.Or(x.logger, slog.Default())` instead.", file))
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Helper-consolidation ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// scanRatchetPattern counts non-comment pattern hits per non-test Go file
// under root/subdir, skipping files where skip(relPath) is true.
func scanRatchetPattern(backendRoot, subdir string, pattern *regexp.Regexp, skip func(string) bool) (map[string]int, error) {
	files, err := goSourceIndex(backendRoot)
	if err != nil {
		return nil, err
	}
	prefix := ""
	if subdir != "." {
		prefix = filepath.ToSlash(subdir) + "/"
	}
	counts := make(map[string]int)
	for _, f := range files {
		if !strings.HasPrefix(f.rel, prefix) || strings.HasSuffix(f.rel, "_test.go") || skip(f.rel) {
			continue
		}
		hits := countContentHits(f.content, func(line string) int {
			return len(pattern.FindAllString(line, -1))
		})
		if hits > 0 {
			counts[f.rel] = hits
		}
	}
	return counts, nil
}

// scanMultiLineGetLoggers returns files whose getLogger() method still
// carries a nil-check body instead of the one-line cmp.Or form.
func scanMultiLineGetLoggers(backendRoot string) ([]string, error) {
	sources, err := goSourceIndex(backendRoot)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range sources {
		if !strings.HasSuffix(f.rel, "_test.go") && !inSkippedDir(f.rel) && hasMultiLineGetLogger(string(f.content)) {
			files = append(files, f.rel)
		}
	}
	sort.Strings(files)
	return files, nil
}

func inSkippedDir(rel string) bool {
	parts := strings.Split(rel, "/")
	for _, part := range parts[:len(parts)-1] {
		if part == "node_modules" || part == ".git" {
			return true
		}
	}
	return false
}

func hasMultiLineGetLogger(content string) bool {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if getLoggerHeadPattern.MatchString(line) && i+1 < len(lines) && getLoggerNilCheckLine.MatchString(lines[i+1]) {
			return true
		}
	}
	return false
}
