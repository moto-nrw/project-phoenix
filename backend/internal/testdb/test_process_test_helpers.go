//go:build testhooks

package testdb

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// RunTestProcess runs exactly one named test in the current test executable.
// It lets fixture tests verify TestMain/cleanup failures without granting the
// fixture package a general process-execution capability. Extra environment is
// child-local; nonzero test exits are results, while start failures are errors.
func RunTestProcess(ctx context.Context, name string, environment ...string) (output []byte, exitCode int, err error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, -1, err
	}
	// Direct binary execution lacks go test's normal ten-minute deadline.
	// Keep the same finite default for deadline-aware fixtures.
	command := exec.CommandContext(ctx, executable, "-test.run=^"+regexp.QuoteMeta(name)+"$", "-test.v", "-test.timeout=10m")
	command.Env = append(os.Environ(), environment...)
	return runTestCommand(command)
}

// RunGoTest executes a tiny, caller-owned fixture module with the current Go
// toolchain. It cannot execute an arbitrary program and never uses a shell.
func RunGoTest(ctx context.Context, dir string, parallel int, timeout time.Duration) ([]byte, int, error) {
	command := exec.CommandContext(ctx, "go",
		"test", "-count=1", "-v", "-parallel="+strconv.Itoa(parallel), "-timeout="+timeout.String(), ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	return runTestCommand(command)
}

func runTestCommand(command *exec.Cmd) (output []byte, exitCode int, err error) {
	output, err = command.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return output, exit.ExitCode(), nil
	}
	return output, 0, err
}
