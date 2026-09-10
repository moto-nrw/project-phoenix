package architecture

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitectureCLIIsolatesRunnerCache(t *testing.T) {
	t.Parallel()

	output, err := runArchitectureWithEnv(t,
		map[string]string{"GOCACHEPROG": filepath.Join(t.TempDir(), "missing-runner-cache")},
		"check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "allowed-edge.json"),
	)
	if err != nil {
		t.Fatalf("CLI fixture inherited runner cache configuration: %v\n%s", err, output)
	}
}

func TestProcessOutputSeparatesSuccessfulDiagnostics(t *testing.T) {
	t.Parallel()

	output, err := processOutput(t.TempDir(), nil, "sh", "-c", `printf '{"Name":"source"}'; printf 'blacksmith-gocacheprog pid=123\n' >&2`)
	if err != nil {
		t.Fatalf("successful command failed: %v", err)
	}
	if string(output) != `{"Name":"source"}` {
		t.Fatalf("machine-readable output contains diagnostics: %q", output)
	}
}

func TestProcessOutputPreservesFailureDiagnostics(t *testing.T) {
	t.Parallel()

	output, err := processOutput(t.TempDir(), nil, "sh", "-c", `printf 'partial output\n'; printf 'command failed\n' >&2; exit 7`)
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 7 {
		t.Fatalf("command failure was lost: %v", err)
	}
	for _, want := range []string{"partial output", "command failed"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("failure output lost %q: %s", want, output)
		}
	}
}
