package test

import (
	"fmt"
	"os/exec"
	"strings"
)

// Git execution belongs to test-support tooling. Register checks and their
// isolated history fixtures use the same runner, not an E2E process boundary.
func registerGitOutput(repo string, args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir = repo
	return command.Output()
}

// registerMergeBase checks only debt introduced since the branch's shared
// history. A newer target tip can retire entries still unchanged on the branch;
// Git does not restore those entries when merging. On a merge checkout the
// target is an ancestor, so its retirements remain the comparison baseline.
func registerMergeBase(git func(...string) ([]byte, error)) (string, error) {
	// Pin the target once: other worktrees may fetch while this check runs.
	revision, err := git("rev-parse", "--verify", "origin/development^{commit}")
	if err != nil {
		return "", fmt.Errorf("register ratchet needs fetched origin/development: %w", err)
	}
	base, err := git("merge-base", "HEAD", strings.TrimSpace(string(revision)))
	if err != nil {
		return "", fmt.Errorf("register ratchet needs a merge base with origin/development: %w", err)
	}
	return strings.TrimSpace(string(base)), nil
}
