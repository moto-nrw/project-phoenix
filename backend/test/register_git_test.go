package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const registerGitKey = "old/worker.go:run:runtime.WithinAdmin#1"

type registerGitFixture struct {
	notices      map[string]losableSideEffectNotice
	transactions transactionBoundaryRegisterData
}

func originalGitRegisters() registerGitFixture {
	return registerGitFixture{
		notices: map[string]losableSideEffectNotice{
			registerGitKey: {Reason: "process-local hint", Fingerprint: "original body"},
		},
		transactions: transactionBoundaryRegisterData{
			Admin:    map[string]string{registerGitKey: "existing administrative work"},
			Commands: map[string]string{registerGitKey: "existing command"},
		},
	}
}

func writeGitRegisters(t *testing.T, repo string, registers registerGitFixture) {
	t.Helper()
	for path, value := range map[string]any{
		sideEffectNoticeRegister:    registers.notices,
		transactionBoundaryRegister: registers.transactions,
	} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(repo, "backend", path)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func registerGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	output, err := registerGitOutput(repo, append([]string{
		"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false",
		"-c", "user.name=Register fixture", "-c", "user.email=register@example.invalid",
	}, args...)...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func newRegisterGitRepo(t *testing.T) (string, string) {
	t.Helper()
	return newRegisterGitRepoWithMoves(t, map[string]string{})
}

func newRegisterGitRepoWithMoves(t *testing.T, moves map[string]string) (string, string) {
	t.Helper()
	repo := t.TempDir()
	registerGit(t, repo, "init", "--initial-branch=branch")
	writeGitRegisters(t, repo, originalGitRegisters())
	// The architecture subprocess is a separate boundary. This fixture grants
	// only the supplied package relocations; Git and register checks remain real.
	if err := os.MkdirAll(filepath.Join(repo, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(moves)
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nset -eu\ntest \"$1\" = relocations\ntest \"$2\" = --base-ref\ncat <<'REGISTER_MOVES'\n" + string(raw) + "\nREGISTER_MOVES\n"
	if err := os.WriteFile(filepath.Join(repo, "scripts/backend-architecture.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	writeRegisterGitSource(t, repo, "old/worker.go", "package fixture\nfunc run() { work() }\n")
	registerGit(t, repo, "add", ".")
	registerGit(t, repo, "commit", "-m", "original registers")
	base := registerGit(t, repo, "rev-parse", "HEAD")
	registerGit(t, repo, "update-ref", "refs/remotes/origin/development", base)
	return repo, base
}

func writeRegisterGitSource(t *testing.T, repo, path, source string) {
	t.Helper()
	file := filepath.Join(repo, "backend", path)
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func checkGitRegisters(t *testing.T, repo string, registers registerGitFixture) {
	t.Helper()
	root := filepath.Join(repo, "backend")
	t.Run("notices", func(t *testing.T) {
		if err := sideEffectRegisterOnlyShrinks(root, registers.notices); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("transactions", func(t *testing.T) {
		if err := transactionRegisterOnlyShrinks(root, registers.transactions); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRegisterRatchetsUseBranchBase(t *testing.T) {
	t.Parallel()
	for _, diverged := range []bool{false, true} {
		name := "head is ancestor of target"
		if diverged {
			name = "diverged branch"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo, _ := newRegisterGitRepo(t)
			registerGit(t, repo, "checkout", "-b", "target")
			writeGitRegisters(t, repo, registerGitFixture{})
			registerGit(t, repo, "commit", "-am", "target retires entries")
			registerGit(t, repo, "update-ref", "refs/remotes/origin/development", "HEAD")
			registerGit(t, repo, "checkout", "branch")
			if diverged {
				registerGit(t, repo, "commit", "--allow-empty", "-m", "independent branch work")
			}
			checkGitRegisters(t, repo, originalGitRegisters())
		})
	}
}
