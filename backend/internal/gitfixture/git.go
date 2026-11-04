//go:build testhooks

// Package gitfixture isolates temporary test repositories from caller Git state.
package gitfixture

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Environment returns a child-only environment, without inherited repository,
// identity, template or command-line config overrides. Git documents these at
// https://git-scm.com/docs/git#Documentation/git.txt-GITCONFIGGLOBAL and
// https://git-scm.com/docs/git-config#Documentation/git-config.txt-GITCONFIGCOUNT.
func Environment(environ []string) []string {
	environment := make([]string, 0, len(environ)+2)
	for _, variable := range environ {
		if !strings.HasPrefix(variable, "GIT_") {
			environment = append(environment, variable)
		}
	}
	return append(environment, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
}

// Run executes Git in a fixture repository without changing the test process.
func Run(t testing.TB, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = Environment(os.Environ())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

// Init initializes a fixture with local identity and no hooks or signing. Local
// settings also protect commands spawned by the code under test. Empty templates
// prevent user-installed template hooks/config from being copied into the repo.
func Init(t testing.TB, dir string) {
	t.Helper()
	Run(t, dir, "init", "--quiet", "--template=")
	for _, setting := range [][2]string{
		{"core.hooksPath", os.DevNull},
		{"user.name", "Git fixture"},
		{"user.email", "fixture@example.invalid"},
		{"commit.gpgsign", "false"},
		{"gc.auto", "0"},
		{"maintenance.auto", "false"},
	} {
		Run(t, dir, "config", "--local", setting[0], setting[1])
	}
}
