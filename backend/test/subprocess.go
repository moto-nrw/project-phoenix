package test

import (
	"os"
	"os/exec"
	"regexp"
)

// RunTestBinary runs exactly one named test in the caller's own test binary.
// Exit behavior can then be verified without importing process execution into
// an owner behavior suite. Environment additions contain synthetic inputs only.
func RunTestBinary(name string, environment []string) ([]byte, int, error) {
	command := exec.Command(os.Args[0], "-test.run=^"+regexp.QuoteMeta(name)+"$")
	command.Env = append(os.Environ(), environment...)
	output, err := command.CombinedOutput()
	if command.ProcessState == nil {
		return output, -1, err
	}
	return output, command.ProcessState.ExitCode(), err
}
