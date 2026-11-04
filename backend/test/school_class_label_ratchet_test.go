package test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// #5101: class names are free text ("1b" or "Klasse 1b"), so a hand-written
// "Klasse " in front of one reads "Klasse Klasse 1b". collation.SchoolClassLabel
// (sharedkernel/collation) is the only backend place that adds the word. This
// ratchet has no allowlist: it rejects "Klasse " + x and "Klasse %s" in any
// quoted Go string, including raw strings, outside that helper. The demo
// seeder only parses the prefix and stays outside the scan.
var handBuiltSchoolClassLabel = regexp.MustCompile("\"[^\"]*\\bKlassen? „?\"\\s*\\+|`[^`]*\\bKlassen? „?`\\s*\\+|(?:\"[^\"]*|`[^`]*)\\bKlassen? „?%[sqv]")

var schoolClassLabelScanExempt = []string{
	"sharedkernel/collation/",
	"internal/devtools/",
}

func TestSchoolClassLabelRatchet(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	findings, err := scanHandBuiltSchoolClassLabels(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) > 0 {
		t.Errorf("Label class names with collation.SchoolClassLabel, not by writing \"Klasse \" in front (#5101):\n%s", strings.Join(findings, "\n"))
	}
}

func TestSchoolClassLabelScannerPattern(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		`return "Klasse " + class`,
		`title := name + ", Klasse " + class`,
		`fmt.Sprintf("Neue Krankmeldung in Klasse %s", class)`,
		`fmt.Sprintf("Klassen %v", classes)`,
		`fmt.Sprintf("Die Klasse „%s“ fehlt", class)`,
		`return "Klasse „" + class + "“"`,
		"return `Klasse ` + class",
		"fmt.Sprintf(`Klassen %v`, classes)",
		"fmt.Sprintf(`Die Klasse „%s“ fehlt`, class)",
	} {
		if !handBuiltSchoolClassLabel.MatchString(line) {
			t.Errorf("scanner misses %s", line)
		}
	}
	for _, line := range []string{
		`{ID: ColumnSchoolClass, Label: "Klasse"},`,
		`return strconv.Itoa(grade) + ". Klasse"`,
		`return "Ohne Klasse"`,
		`return "Unterklasse " + x`,
	} {
		if handBuiltSchoolClassLabel.MatchString(line) {
			t.Errorf("scanner flags %s", line)
		}
	}
}

func scanHandBuiltSchoolClassLabels(root string) ([]string, error) {
	var findings []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "tmp") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, prefix := range schoolClassLabelScanExempt {
			if strings.HasPrefix(rel, prefix) {
				return nil
			}
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if handBuiltSchoolClassLabel.MatchString(line) {
				findings = append(findings, rel+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	sort.Strings(findings)
	return findings, err
}
