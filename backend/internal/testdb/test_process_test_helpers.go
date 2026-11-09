//go:build testhooks

package testdb

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
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
	command := exec.CommandContext(ctx, executable, "-test.run=^"+regexp.QuoteMeta(name)+"$", "-test.v")
	command.Env = append(os.Environ(), environment...)
	output, err = command.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return output, exit.ExitCode(), nil
	}
	return output, 0, err
}
