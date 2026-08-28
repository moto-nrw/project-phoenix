package architecture

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type LegacyEntry struct {
	Violation
	Issue string `json:"issue"`
}

type LegacyManifest struct {
	Entries []LegacyEntry
	byKey   map[string]LegacyEntry
}

type GitHubIssue struct {
	Owner  string
	Repo   string
	Number int
	URL    string
}

func LoadLegacyManifest(path string) (*LegacyManifest, error) {
	file, err := os.Open(path) // #nosec G304 -- path is an explicit CLI input
	if err != nil {
		return nil, fmt.Errorf("open legacy baseline: %w", err)
	}
	defer func() { _ = file.Close() }()
	return DecodeLegacyManifest(file)
}

func DecodeLegacyManifest(reader io.Reader) (*LegacyManifest, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	manifest := &LegacyManifest{byKey: make(map[string]LegacyEntry)}
	for line := 1; scanner.Scan(); line++ {
		if err := manifest.addLine(line, scanner.Bytes()); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read legacy baseline: %w", err)
	}
	return manifest, nil
}

func (m *LegacyManifest) addLine(line int, raw []byte) error {
	if len(raw) == 0 {
		return fmt.Errorf("legacy baseline line %d is blank", line)
	}
	entry, err := decodeLegacyEntry(raw)
	if err != nil {
		return fmt.Errorf("legacy baseline line %d: %w", line, err)
	}
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("legacy baseline line %d: %w", line, err)
	}
	if err := requireCanonicalJSON(raw, entry); err != nil {
		return fmt.Errorf("legacy baseline line %d: %w", line, err)
	}
	return m.appendSorted(line, entry)
}

func decodeLegacyEntry(raw []byte) (LegacyEntry, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var entry LegacyEntry
	if err := decoder.Decode(&entry); err != nil {
		return LegacyEntry{}, fmt.Errorf("decode JSON: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return LegacyEntry{}, err
	}
	return entry, nil
}

func requireCanonicalJSON(raw []byte, entry LegacyEntry) error {
	canonical, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode canonical JSON: %w", err)
	}
	if !bytes.Equal(raw, canonical) {
		return fmt.Errorf("record must use canonical compact JSON field order: %s", canonical)
	}
	return nil
}

func (m *LegacyManifest) appendSorted(line int, entry LegacyEntry) error {
	key := entry.Key()
	if _, exists := m.byKey[key]; exists {
		return fmt.Errorf("legacy baseline line %d has duplicate canonical key %q", line, key)
	}
	if len(m.Entries) > 0 && m.Entries[len(m.Entries)-1].Key() > key {
		return fmt.Errorf("legacy baseline line %d is not sorted by canonical key", line)
	}
	m.Entries = append(m.Entries, entry)
	m.byKey[key] = entry
	return nil
}

func (e LegacyEntry) Validate() error {
	if !allowedScopes[string(e.Scope)] {
		return fmt.Errorf("scope %q is invalid", e.Scope)
	}
	if !identifierPattern.MatchString(e.Rule) {
		return fmt.Errorf("rule %q is invalid", e.Rule)
	}
	if err := validateExactLegacyValue("source", e.Source); err != nil {
		return err
	}
	if !isCanonicalSourcePackage(e.Source) {
		return fmt.Errorf("source %q must be an exact Go package path, not a package family or layer", e.Source)
	}
	if err := validateExactLegacyValue("target", e.Target); err != nil {
		return err
	}
	_, err := ParseGitHubIssue(e.Issue)
	return err
}

func isCanonicalSourcePackage(source string) bool {
	firstSlash := strings.IndexByte(source, '/')
	return firstSlash > 0 && strings.Contains(source[:firstSlash], ".")
}

func validateExactLegacyValue(label, value string) error {
	if value == "" || strings.TrimSpace(value) != value || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return fmt.Errorf("%s must be a non-empty exact value without whitespace", label)
	}
	if strings.ContainsAny(value, "*?[]{}") || strings.Contains(value, "...") {
		return fmt.Errorf("%s %q contains a wildcard or package-family pattern", label, value)
	}
	if strings.Contains(value, "|") {
		return fmt.Errorf("%s %q contains the canonical-key separator", label, value)
	}
	return nil
}

func ParseGitHubIssue(raw string) (GitHubIssue, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return GitHubIssue{}, fmt.Errorf("migration issue URL %q is invalid: %w", raw, err)
	}
	parts := strings.Split(parsed.Path, "/")
	if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || len(parts) != 5 || parts[0] != "" || parts[3] != "issues" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return GitHubIssue{}, fmt.Errorf("migration issue %q must be an exact https://github.com/<owner>/<repo>/issues/<number> URL", raw)
	}
	number, err := strconv.Atoi(parts[4])
	if err != nil || number < 1 || parts[1] == "" || parts[2] == "" || raw != fmt.Sprintf("https://github.com/%s/%s/issues/%d", parts[1], parts[2], number) {
		return GitHubIssue{}, fmt.Errorf("migration issue %q has an invalid issue number", raw)
	}
	return GitHubIssue{Owner: parts[1], Repo: parts[2], Number: number, URL: raw}, nil
}

func EnforceLegacyBaseline(violations []Violation, manifest *LegacyManifest) (int, error) {
	current := uniqueSortedViolations(violations)
	currentByKey := make(map[string]Violation, len(current))
	for _, violation := range current {
		currentByKey[violation.Key()] = violation
	}
	newViolations := missingFromManifest(current, manifest)
	staleEntries := missingFromViolations(manifest, currentByKey)
	if len(newViolations) == 0 && len(staleEntries) == 0 {
		return len(current), nil
	}
	return len(current), formatLegacyMismatch(newViolations, staleEntries)
}

func missingFromManifest(violations []Violation, manifest *LegacyManifest) []Violation {
	var missing []Violation
	for _, violation := range violations {
		if _, exists := manifest.byKey[violation.Key()]; !exists {
			missing = append(missing, violation)
		}
	}
	return missing
}

func missingFromViolations(manifest *LegacyManifest, violations map[string]Violation) []LegacyEntry {
	var missing []LegacyEntry
	for _, entry := range manifest.Entries {
		if _, exists := violations[entry.Key()]; !exists {
			missing = append(missing, entry)
		}
	}
	return missing
}

func formatLegacyMismatch(newViolations []Violation, stale []LegacyEntry) error {
	lines := []string{"backend architecture legacy ratchet failed:"}
	if len(newViolations) > 0 {
		lines = append(lines, fmt.Sprintf("new violations (%d):", len(newViolations)))
		for _, violation := range newViolations {
			lines = append(lines, "  "+violation.Key()+" -- "+violation.Detail)
		}
	}
	if len(stale) > 0 {
		lines = append(lines, fmt.Sprintf("stale legacy entries (%d):", len(stale)))
		for _, entry := range stale {
			lines = append(lines, "  "+entry.Key()+" -- remove the resolved entry")
		}
	}
	return errors.New(strings.Join(lines, "\n"))
}

func CompareLegacyBaselines(candidate, base *LegacyManifest) error {
	var added, reassigned []string
	for _, entry := range candidate.Entries {
		baseEntry, exists := base.byKey[entry.Key()]
		switch {
		case !exists:
			added = append(added, fmt.Sprintf("%s is absent from the base baseline", entry.Key()))
		case baseEntry.Issue != entry.Issue:
			reassigned = append(reassigned, fmt.Sprintf("%s changed migration issue from %s to %s", entry.Key(), baseEntry.Issue, entry.Issue))
		}
	}
	if len(added) == 0 && len(reassigned) == 0 {
		return nil
	}
	sort.Strings(added)
	sort.Strings(reassigned)
	lines := []string{"base legacy baseline comparison failed:"}
	lines = appendBaseProblems(lines, "new violations compared with base", added)
	lines = appendBaseProblems(lines, "migration issue changes", reassigned)
	return errors.New(strings.Join(lines, "\n"))
}

func appendBaseProblems(lines []string, label string, problems []string) []string {
	if len(problems) == 0 {
		return lines
	}
	lines = append(lines, fmt.Sprintf("%s (%d):", label, len(problems)))
	for _, problem := range problems {
		lines = append(lines, "  "+problem)
	}
	return lines
}
