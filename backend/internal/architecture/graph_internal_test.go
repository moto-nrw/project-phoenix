package architecture

import (
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceLocationRejectsUnavailableAndExternalPositions(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	tests := []struct {
		name     string
		position token.Position
		want     string
	}{
		{name: "unavailable", position: token.Position{}, want: "source position is unavailable"},
		{name: "outside project", position: token.Position{Filename: filepath.Join(filepath.Dir(project), "outside.go"), Line: 1}, want: "source path is outside project"},
	}
	for _, test := range tests {
		_, err := sourceLocation(project, test.position, "declaration")
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s error = %v, want %q", test.name, err, test.want)
		}
	}
}

// The base tree is extracted into a new temporary directory per check. Its
// export data comes from the build cache only if the directory stays out of
// the compile action, which needs -trimpath even when the caller sets no
// GOFLAGS. The first half of a build ID is the action ID, the cache key.
func TestFixedBuildEnvironmentReusesCompileActionsAcrossDirectories(t *testing.T) {
	t.Parallel()

	var inherited []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOFLAGS=") {
			inherited = append(inherited, entry)
		}
	}
	environment := append(buildEnvironment(inherited, Build{GOOS: "linux", GOARCH: "amd64", Tags: []string{"analysis"}}),
		"GOCACHE="+t.TempDir(), "GOWORK=off")
	actionID := func(directory string) string {
		t.Helper()
		module := map[string]string{
			"go.mod":      "module example.com/trimmed\n\ngo 1.21\n",
			"trimmed.go":  "package trimmed\n\nconst Answer = 42\n",
			"tagged.go":   "//go:build analysis\n\npackage trimmed\n\nconst Tagged = true\n",
			"untagged.go": "//go:build !analysis\n\npackage trimmed\n\nconst Tagged = false\n",
		}
		for name, source := range module {
			if err := os.WriteFile(filepath.Join(directory, name), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		output, err := processOutput(directory, environment, "go", "list", "-export", "-f", "{{.BuildID}} {{.GoFiles}}", ".")
		if err != nil {
			t.Fatalf("go list in %s: %v: %s", directory, err, output)
		}
		buildID, files, _ := strings.Cut(strings.TrimSpace(string(output)), " ")
		if files != "[tagged.go trimmed.go]" {
			t.Fatalf("analysis tags select %s, want [tagged.go trimmed.go]", files)
		}
		action, _, found := strings.Cut(buildID, "/")
		if !found || action == "" {
			t.Fatalf("build ID %q has no action ID", buildID)
		}
		return action
	}
	if first, second := actionID(t.TempDir()), actionID(t.TempDir()); first != second {
		t.Fatalf("the same package in two directories has action IDs %s and %s; the second compiles again", first, second)
	}
}
