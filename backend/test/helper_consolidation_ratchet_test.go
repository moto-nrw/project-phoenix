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

// TestHelperConsolidationRatchet guards the audit-B7 helper consolidations
// against regrowth (report §8.7). Three patterns, all shrink-only:
//
//  1. The PostgreSQL unique-violation code "23505" hard-coded outside
//     models/base — use base.IsUniqueViolation / base.IsUniqueViolationOn.
//  2. Hand-rolled strconv.ParseInt(chi.URLParam(...)) in api/ — use
//     common.ParseInt64IDWithError / ParsePositiveInt64IDWithError.
//  3. Multi-line nil-check getLogger bodies — use cmp.Or(logger, slog.Default()).
//
// Allowlists are shrink-only: never add an entry, never raise a count.
var (
	// Non-comment "23505" references outside models/base and migrations.
	uniqueViolationLiteralAllowlist = map[string]int{
		// Deliberately kept: guardian-email unique check has an extra
		// SQLSTATE-substring fallback for errors that lose the pgdriver type.
		"services/parent/parent_guardian_service.go": 2,
	}

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
		return strings.HasPrefix(rel, "models/base/") || strings.HasPrefix(rel, "database/migrations/")
	})
	if err != nil {
		t.Fatalf("scanning for 23505 literals failed: %v", err)
	}
	check(counts23505, uniqueViolationLiteralAllowlist, `"23505" literal`,
		"Use base.IsUniqueViolation / base.IsUniqueViolationOn (models/base/db_errors.go).")

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
	counts := make(map[string]int)
	err := filepath.WalkDir(filepath.Join(backendRoot, subdir), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(backendRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skip(rel) {
			return nil
		}
		hits, err := countNonCommentHits(path, pattern)
		if err != nil {
			return err
		}
		if hits > 0 {
			counts[rel] = hits
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

func countNonCommentHits(path string, pattern *regexp.Regexp) (int, error) {
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
		hits += len(pattern.FindAllString(line, -1))
	}
	return hits, scanner.Err()
}

// scanMultiLineGetLoggers returns files whose getLogger() method still
// carries a nil-check body instead of the one-line cmp.Or form.
func scanMultiLineGetLoggers(backendRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(backendRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path) // #nosec G304 -- test scans repo-local source files
		if err != nil {
			return err
		}
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if !getLoggerHeadPattern.MatchString(line) {
				continue
			}
			if i+1 < len(lines) && getLoggerNilCheckLine.MatchString(lines[i+1]) {
				rel, relErr := filepath.Rel(backendRoot, path)
				if relErr != nil {
					return relErr
				}
				files = append(files, filepath.ToSlash(rel))
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
