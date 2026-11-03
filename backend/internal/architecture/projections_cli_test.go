package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectionCommandsPreserveInputFailures(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"diagram", "dependencies"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			assertProjectionCommandFailures(t, command)
		})
	}
}

func TestProjectionsPreservesInputFailures(t *testing.T) {
	t.Parallel()
	assertProjectionCommandFailures(t, "projections")
}

func assertProjectionCommandFailures(t *testing.T, command string) {
	t.Helper()
	base := []string{command, "--project", fixturePath(t, "projection"), "--policy", fixturePath(t, "projection", "policy.json"), "--baseline", fixturePath(t, "projection", "legacy.jsonl")}
	if command != "diagram" {
		base = append(base, "--focus", "module:beta")
	}
	badBaseline := filepath.Join(t.TempDir(), "bad.jsonl")
	writeFile(t, badBaseline, "{not-json}\n")
	type failureCase struct {
		name string
		args []string
		want string
	}
	cases := []failureCase{
		{"policy", []string{"--policy", fixturePath(t, "unknown-field.json")}, `unknown field "surprise"`},
		{"baseline", []string{"--baseline", badBaseline}, "legacy baseline line 1: decode JSON"},
		{"graph", []string{"--project", t.TempDir()}, "load Go packages"},
		{"output", []string{"--output", fixturePath(t, "projection", "generated")}, "must be inside the system temporary directory"},
		{"argument", []string{"unexpected"}, command + ": unexpected arguments"},
		{"flag", []string{"--not-a-flag"}, "flag provided but not defined"},
	}
	if command != "diagram" {
		cases = append(cases,
			failureCase{"unknown-focus", []string{"--focus", "missing"}, "unknown focus"},
			failureCase{"ambiguous-focus", []string{"--focus", "beta"}, "ambiguous focus"},
			failureCase{"empty-focus", []string{"--focus", ""}, command + " requires --focus"},
		)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append(append([]string(nil), base...), tc.args...)
			err := RunCLI(args, CLIDependencies{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestProjectionsWritesExactlyExistingArtifactBytes(t *testing.T) {
	t.Parallel()
	for _, focus := range []string{"module:beta", "package:alpha"} {
		t.Run(focus, func(t *testing.T) {
			t.Parallel()
			base := []string{"--project", fixturePath(t, "projection"), "--policy", fixturePath(t, "projection", "policy.json"), "--baseline", fixturePath(t, "projection", "legacy.jsonl")}
			reference, combined := t.TempDir(), t.TempDir()
			for _, invocation := range []struct {
				command string
				output  string
				focus   bool
			}{{"diagram", reference, false}, {"dependencies", reference, true}, {"projections", combined, true}} {
				args := append([]string{invocation.command}, base...)
				args = append(args, "--output", invocation.output)
				if invocation.focus {
					args = append(args, "--focus", focus)
				}
				output, err := runArchitecture(t, args...)
				if err != nil {
					t.Fatalf("%s: %v\n%s", invocation.command, err, output)
				}
			}
			entries, err := os.ReadDir(combined)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 7 {
				t.Fatalf("got %d artifacts, want 7", len(entries))
			}
			for _, name := range []string{"architecture.json", "go-arch-lint.yml", "dependencies.json", "dependencies.goda", "target.svg", "migration.svg", "dependencies.svg"} {
				got, want := readFile(t, filepath.Join(combined, name)), readFile(t, filepath.Join(reference, name))
				if got == "" || got != want {
					t.Errorf("%s bytes differ from existing commands", name)
				}
				if strings.HasSuffix(name, ".svg") {
					assertValidSVGFile(t, filepath.Join(combined, name))
					if !strings.HasPrefix(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
						t.Errorf("%s lacks SVG root namespace", name)
					}
				}
			}
		})
	}
}

func TestProjectionsRequiresFocus(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"projections"}, {"projections", "--focus"}, {"projections", "--policy"}} {
		if err := RunCLI(args, CLIDependencies{}); err == nil {
			t.Fatalf("missing flag accepted: %v", args)
		}
	}
}

func TestProjectionsRejectsArtifactSymlinksAndOutputEscape(t *testing.T) {
	t.Parallel()
	base := []string{"projections", "--project", fixturePath(t, "projection"), "--policy", fixturePath(t, "projection", "policy.json"), "--baseline", fixturePath(t, "projection", "legacy.jsonl"), "--focus", "module:beta"}
	t.Run("artifact", func(t *testing.T) {
		t.Parallel()
		output, target := t.TempDir(), filepath.Join(t.TempDir(), "protected")
		writeFile(t, target, "unchanged")
		if err := os.Symlink(target, filepath.Join(output, "architecture.json")); err != nil {
			t.Fatal(err)
		}
		err := RunCLI(append(append([]string(nil), base...), "--output", output), CLIDependencies{})
		if err == nil || !strings.Contains(err.Error(), "write generated architecture artifact architecture.json") {
			t.Fatalf("error = %v", err)
		}
		if readFile(t, target) != "unchanged" {
			t.Fatal("artifact symlink target overwritten")
		}
	})
	t.Run("escape", func(t *testing.T) {
		t.Parallel()
		link := filepath.Join(t.TempDir(), "outside")
		if err := os.Symlink(fixturePath(t, "projection"), link); err != nil {
			t.Fatal(err)
		}
		err := RunCLI(append(append([]string(nil), base...), "--output", link), CLIDependencies{})
		if err == nil || !strings.Contains(err.Error(), "must resolve inside the system temporary directory") {
			t.Fatalf("error = %v", err)
		}
	})
}
