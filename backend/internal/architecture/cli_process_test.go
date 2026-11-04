package architecture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type architectureTestCommand struct {
	directory, module, binary, goCommand string
}

// The production CLI is immutable during a test process. Build it once, not
// once per assertion. Keep the real shell wrapper and every fresh CLI process;
// only its repeated compilation is replaced with this identical executable.
// The separate Git-hook wrapper test still exercises the ordinary go-run path.
var architectureTestCLI = sync.OnceValues(func() (architectureTestCommand, error) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		return architectureTestCommand{}, err
	}
	directory, err := os.MkdirTemp(ratchetTemplates.directory, "cli-")
	if err != nil {
		return architectureTestCommand{}, err
	}
	command := architectureTestCommand{
		directory: directory, module: filepath.Join(root, "scripts", "backend-architecture"),
		binary: filepath.Join(directory, "architecturecmd"),
	}
	environment := append(gitTestEnvironment(), "GOCACHEPROG=")
	output, err := processOutput(command.module, environment, "go", "env", "GOROOT")
	if err != nil {
		return architectureTestCommand{}, err
	}
	goRoot := strings.TrimSpace(string(output))
	if goRoot == "" {
		return architectureTestCommand{}, fmt.Errorf("go env GOROOT returned an empty path")
	}
	command.goCommand = filepath.Join(goRoot, "bin", "go")
	output, err = processOutput(command.module, environment, command.goCommand, "build", "-o", command.binary, ".")
	if err != nil {
		return architectureTestCommand{}, fmt.Errorf("build real architecture CLI: %w\n%s", err, output)
	}
	// Paths are passed as environment entries, never interpolated into shell.
	// Other Go commands, including package/type analysis, run unchanged.
	bridge := `#!/bin/sh
set -eu
if [ "$PWD" = "$PHX_ARCH_TEST_MODULE" ] && [ "${1:-}" = run ] && [ "${2:-}" = . ]; then
 shift 2
 exec "$PHX_ARCH_TEST_BINARY" "$@"
fi
exec "$PHX_ARCH_TEST_GO" "$@"
`
	if err := os.WriteFile(filepath.Join(directory, "go"), []byte(bridge), 0o700); err != nil {
		return architectureTestCommand{}, err
	}
	return command, nil
})

func runArchitecture(t *testing.T, args ...string) (string, error) {
	return runArchitectureWithEnv(t, nil, args...)
}

func runArchitectureWithEnv(t *testing.T, environment map[string]string, args ...string) (string, error) {
	t.Helper()
	if hasProjectWithoutBaseline(args) {
		args = append(args, "--baseline", "")
	}

	root := filepath.Clean(filepath.Join(packageDir(t), "..", "..", ".."))
	environmentVariables := gitTestEnvironment()
	for key, value := range environment {
		prefix := key + "="
		filtered := environmentVariables[:0]
		for _, variable := range environmentVariables {
			if !strings.HasPrefix(variable, prefix) {
				filtered = append(filtered, variable)
			}
		}
		environmentVariables = append(filtered, prefix+value)
	}
	// These tests compare the CLI's failure diagnostics byte-for-byte. Keep
	// runner-injected cache telemetry out of the shared CLI build;
	// the ordinary local Go build cache remains enabled.
	environmentVariables = append(environmentVariables, "GOCACHEPROG=")
	if len(environment) == 0 {
		command, err := architectureTestCLI()
		if err != nil {
			t.Fatal(err)
		}
		environmentVariables = append(environmentVariables,
			"PHX_ARCH_TEST_MODULE="+command.module,
			"PHX_ARCH_TEST_BINARY="+command.binary,
			"PHX_ARCH_TEST_GO="+command.goCommand,
			"PATH="+command.directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	output, err := processOutput(root, environmentVariables, filepath.Join(root, "scripts", "backend-architecture.sh"), args...)
	return string(output), err
}

func hasProjectWithoutBaseline(args []string) bool {
	hasProject := false
	for _, argument := range args {
		switch {
		case argument == "--project" || strings.HasPrefix(argument, "--project="):
			hasProject = true
		case argument == "--baseline" || strings.HasPrefix(argument, "--baseline="):
			return false
		}
	}
	return hasProject
}
