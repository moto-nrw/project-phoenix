package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModulePassthroughOwnerKey(t *testing.T) {
	t.Parallel()

	for path, want := range map[string]string{
		"modules/timetable/internal/application/x.go":                   "modules/timetable",
		"modules/timetable/compose/x.go":                                "modules/timetable",
		"modules/timetable/compose/httpadapter/x.go":                    "modules/timetable",
		"modules/careplan/requestfeed/internal/application/x.go":        "modules/careplan",
		"modules/careplan/requestfeed/compose/x.go":                     "modules/careplan",
		"modules/communication/composition/x.go":                        "modules/communication",
		"workflows/parentportal/compose/x.go":                           "workflows/parentportal",
		"workflows/sessionend/internal/application/x.go":                "workflows/sessionend",
		"modules/timetable/internal/adapters/postgres/x.go":             "",
		"modules/timetable/http/x.go":                                   "",
		"modules/timetable/x.go":                                        "",
		"modules/compose/x.go":                                          "",
		"modules/timetable/internal/domain/compose_rules.go":            "",
		"modules/timetable/internal/application/compose/nested_test.go": "modules/timetable",
	} {
		if got := modulePassthroughOwnerKey(path); got != want {
			t.Errorf("modulePassthroughOwnerKey(%q) = %q, want %q", path, got, want)
		}
	}
}

// passthroughSource is a file with the given number of counted pass-throughs:
// each exported method makes exactly one call through a receiver field.
func passthroughSource(pkg string, methods ...string) string {
	var b strings.Builder
	b.WriteString("package " + pkg + "\n\ntype store interface{ Find() error }\n\ntype S struct{ store store }\n")
	for _, method := range methods {
		b.WriteString("\nfunc (s S) " + method + "() error { return s.store.Find() }\n")
	}
	return b.String()
}

func writePassthroughTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	// The scan walks both trees; this one may hold nothing.
	if err := os.MkdirAll(filepath.Join(root, "workflows"), 0o750); err != nil {
		t.Fatal(err)
	}
	for rel, source := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestModulePassthroughOwnerRegister(t *testing.T) {
	t.Parallel()

	const (
		alphaApplication = "modules/alpha/internal/application/service.go"
		alphaCompose     = "modules/alpha/compose/engine.go"
		betaApplication  = "modules/beta/internal/application/service.go"
	)
	// The base tree: alpha forwards once in its application layer and once in
	// its composition, beta once in its application layer.
	budgets := map[string]int{"modules/alpha": 2, "modules/beta": 1}

	for _, scenario := range []struct {
		name  string
		files map[string]string
		want  []string // substrings of the expected violations, in any order
	}{
		{
			name: "unchanged tree",
			files: map[string]string{
				alphaApplication: passthroughSource("application", "Find"),
				alphaCompose:     passthroughSource("compose", "Bind"),
				betaApplication:  passthroughSource("application", "Find"),
			},
		},
		{
			name: "a move from the composition into the application layer of the same owner",
			files: map[string]string{
				alphaApplication: passthroughSource("application", "Find", "Bind"),
				alphaCompose:     passthroughSource("compose"),
				betaApplication:  passthroughSource("application", "Find"),
			},
		},
		{
			name: "growth in the application layer",
			files: map[string]string{
				alphaApplication: passthroughSource("application", "Find", "Extra"),
				alphaCompose:     passthroughSource("compose", "Bind"),
				betaApplication:  passthroughSource("application", "Find"),
			},
			want: []string{"modules/alpha: 3 single-call delegation(s), budget 2."},
		},
		{
			name: "growth in the composition",
			files: map[string]string{
				alphaApplication: passthroughSource("application", "Find"),
				alphaCompose:     passthroughSource("compose", "Bind", "Extra"),
				betaApplication:  passthroughSource("application", "Find"),
			},
			want: []string{"modules/alpha: 3 single-call delegation(s), budget 2."},
		},
		{
			name: "a move to another owner counts as growth there",
			files: map[string]string{
				alphaApplication: passthroughSource("application", "Find"),
				alphaCompose:     passthroughSource("compose"),
				betaApplication:  passthroughSource("application", "Find", "Bind"),
			},
			want: []string{
				"modules/alpha: 1 single-call delegation(s), budget says 2.",
				"modules/beta: 2 single-call delegation(s), budget 1.",
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			root := writePassthroughTree(t, scenario.files)
			counts, err := modulePassthroughScan(root, modulePassthroughOwnerKey)
			if err != nil {
				t.Fatal(err)
			}
			violations := modulePassthroughViolations(counts, budgets)
			if len(violations) != len(scenario.want) {
				t.Fatalf("violations = %q, want %d matching %q", violations, len(scenario.want), scenario.want)
			}
			for _, want := range scenario.want {
				found := false
				for _, violation := range violations {
					found = found || strings.Contains(violation, want)
				}
				if !found {
					t.Errorf("no violation contains %q: %q", want, violations)
				}
			}
		})
	}
}
