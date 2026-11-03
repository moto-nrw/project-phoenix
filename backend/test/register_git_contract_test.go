package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitRegistersWithEntry(key, reason, fingerprint string) registerGitFixture {
	return registerGitFixture{
		notices: map[string]losableSideEffectNotice{key: {Reason: reason, Fingerprint: fingerprint}},
		transactions: transactionBoundaryRegisterData{
			Admin: map[string]string{key: reason}, Commands: map[string]string{key: reason},
		},
	}
}

func requireGitRegisterRejection(t *testing.T, repo string, current registerGitFixture, want string) {
	t.Helper()
	requireGitRegisterErrors(t, repo, current, want, want)
}

func requireGitRegisterErrors(t *testing.T, repo string, current registerGitFixture, noticeWant, transactionWant string) {
	t.Helper()
	root := filepath.Join(repo, "backend")
	if err := sideEffectRegisterOnlyShrinks(root, current.notices); err == nil || !strings.Contains(err.Error(), noticeWant) {
		t.Errorf("notice error = %v, want %q", err, noticeWant)
	}
	// Exercise both scope maps independently so one cannot conceal the other.
	for name, scope := range map[string]transactionBoundaryRegisterData{
		"admin":    {Admin: current.transactions.Admin},
		"commands": {Commands: current.transactions.Commands},
	} {
		if err := transactionRegisterOnlyShrinks(root, scope); err == nil || !strings.Contains(err.Error(), transactionWant) {
			t.Errorf("%s error = %v, want %q", name, err, transactionWant)
		}
	}
}

func advanceRegisterGitTarget(t *testing.T, repo string, registers registerGitFixture) string {
	t.Helper()
	registerGit(t, repo, "checkout", "-B", "target", "origin/development")
	writeGitRegisters(t, repo, registers)
	registerGit(t, repo, "commit", "--allow-empty", "-am", "advance target")
	target := registerGit(t, repo, "rev-parse", "HEAD")
	registerGit(t, repo, "update-ref", "refs/remotes/origin/development", target)
	registerGit(t, repo, "checkout", "branch")
	return target
}

func TestRegisterRatchetsRejectBranchGrowthAfterTargetAdvance(t *testing.T) {
	t.Parallel()
	const moved = "new/worker.go:run:runtime.WithinAdmin#1"
	for _, tc := range []struct {
		name    string
		current registerGitFixture
	}{
		{"new entry", gitRegistersWithEntry("new/worker.go:other:runtime.WithinAdmin#1", "new work", "new body")},
		{"changed reason", gitRegistersWithEntry(registerGitKey, "wider work", "original body")},
		{"changed operation", gitRegistersWithEntry(registerGitKey+"changed", "existing administrative work", "original body")},
		{"copied relocation", registerGitFixture{
			notices: map[string]losableSideEffectNotice{
				registerGitKey: {Reason: "process-local hint", Fingerprint: "original body"},
				moved:          {Reason: "process-local hint", Fingerprint: "original body"},
			},
			transactions: transactionBoundaryRegisterData{
				Admin:    map[string]string{registerGitKey: "existing administrative work", moved: "existing administrative work"},
				Commands: map[string]string{registerGitKey: "existing command", moved: "existing command"},
			},
		}},
		{"relocation reused twice", registerGitFixture{
			notices: map[string]losableSideEffectNotice{
				moved: {Reason: "process-local hint", Fingerprint: "original body"},
				"other/worker.go:run:runtime.WithinAdmin#1": {Reason: "process-local hint", Fingerprint: "original body"},
			},
			transactions: transactionBoundaryRegisterData{
				Admin:    map[string]string{moved: "existing administrative work", "other/worker.go:run:runtime.WithinAdmin#1": "existing administrative work"},
				Commands: map[string]string{moved: "existing command", "other/worker.go:run:runtime.WithinAdmin#1": "existing command"},
			},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := newRegisterGitRepoWithMoves(t, map[string]string{"old": "new"})
			advanceRegisterGitTarget(t, repo, registerGitFixture{})
			advanceRegisterGitTarget(t, repo, registerGitFixture{})
			writeRegisterGitSource(t, repo, "new/worker.go", "package fixture\nfunc run() { work() }\n")
			requireGitRegisterRejection(t, repo, tc.current, "register")
		})
	}
}

func TestRegisterRatchetsMergePreservesTargetRetirements(t *testing.T) {
	t.Parallel()
	repo, _ := newRegisterGitRepo(t)
	registerGit(t, repo, "commit", "--allow-empty", "-m", "independent branch work")
	target := advanceRegisterGitTarget(t, repo, registerGitFixture{})
	registerGit(t, repo, "merge", "--no-ff", "--no-edit", "origin/development")
	if base := registerGit(t, repo, "merge-base", "HEAD", "origin/development"); base != target {
		t.Fatalf("merge baseline = %s, want target %s", base, target)
	}
	// Inspect Git's actual merge result, not a reconstructed expected tree.
	var merged registerGitFixture
	for path, value := range map[string]any{
		sideEffectNoticeRegister: &merged.notices, transactionBoundaryRegister: &merged.transactions,
	} {
		raw, err := os.ReadFile(filepath.Join(repo, "backend", path))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, value); err != nil {
			t.Fatal(err)
		}
	}
	if len(merged.notices)+len(merged.transactions.Admin)+len(merged.transactions.Commands) != 0 {
		t.Fatal("Git reintroduced retired entries")
	}
	checkGitRegisters(t, repo, merged)
	writeGitRegisters(t, repo, originalGitRegisters())
	requireGitRegisterRejection(t, repo, originalGitRegisters(), "register may only shrink")
}

func TestRegisterRatchetsMissingHistoryFailsClosed(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"missing ref", "non-commit ref", "unrelated history"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			repo, _ := newRegisterGitRepo(t)
			want := "needs fetched origin/development"
			switch scenario {
			case "missing ref":
				registerGit(t, repo, "update-ref", "-d", "refs/remotes/origin/development")
			case "non-commit ref":
				blob := registerGit(t, repo, "rev-parse", "HEAD:backend/"+sideEffectNoticeRegister)
				registerGit(t, repo, "update-ref", "refs/remotes/origin/development", blob)
			case "unrelated history":
				registerGit(t, repo, "checkout", "--orphan", "unrelated")
				registerGit(t, repo, "commit", "-m", "unrelated root")
				registerGit(t, repo, "update-ref", "refs/remotes/origin/development", "HEAD")
				registerGit(t, repo, "checkout", "branch")
				want = "needs a merge base"
			}
			// Empty current registers must not turn missing input into success.
			requireGitRegisterRejection(t, repo, registerGitFixture{}, want)
		})
	}
}

func TestRegisterMergeBasePinsTargetBeforeRefMoves(t *testing.T) {
	t.Parallel()
	repo, base := newRegisterGitRepo(t)
	registerGit(t, repo, "checkout", "--orphan", "unrelated")
	registerGit(t, repo, "commit", "-m", "unrelated root")
	replacement := registerGit(t, repo, "rev-parse", "HEAD")
	registerGit(t, repo, "checkout", "branch")
	resolved, err := registerMergeBase(func(args ...string) ([]byte, error) {
		raw, err := registerGitOutput(repo, args...)
		if len(args) > 0 && args[0] == "rev-parse" && err == nil {
			registerGit(t, repo, "update-ref", "refs/remotes/origin/development", replacement)
		}
		return raw, err
	})
	if err != nil || resolved != base {
		t.Fatalf("baseline = %s, error = %v, want original %s", resolved, err, base)
	}
}

func TestRegisterRatchetsRelocationsStillRequireOriginalIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, reason, fingerprint, body, operation string
		wantErr                                    bool
	}{
		{name: "unchanged relocation"},
		{name: "changed reason", reason: "wider work", wantErr: true},
		{name: "changed fingerprint and body", fingerprint: "changed body", body: "widerWork()", wantErr: true},
		{name: "changed operation", operation: "#2", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := newRegisterGitRepoWithMoves(t, map[string]string{"old": "new"})
			// Target changes both the register and source. Relocation fingerprints
			// must still come from the branch's common ancestor, not that tip.
			registerGit(t, repo, "checkout", "-b", "target")
			writeGitRegisters(t, repo, registerGitFixture{})
			writeRegisterGitSource(t, repo, "old/worker.go", "package fixture\nfunc run() { targetWork() }\n")
			registerGit(t, repo, "commit", "-am", "target changes the old source")
			registerGit(t, repo, "update-ref", "refs/remotes/origin/development", "HEAD")
			registerGit(t, repo, "checkout", "branch")
			key := "new/worker.go:run:runtime.WithinAdmin#1"
			if tc.operation != "" {
				key = "new/worker.go:run:runtime.WithinAdmin" + tc.operation
			}
			noticeReason, fingerprint := "process-local hint", "original body"
			adminReason, commandReason, body := "existing administrative work", "existing command", "work()"
			if tc.reason != "" {
				noticeReason, adminReason, commandReason = tc.reason, tc.reason, tc.reason
			}
			if tc.fingerprint != "" {
				fingerprint = tc.fingerprint
			}
			if tc.body != "" {
				body = tc.body
			}
			current := registerGitFixture{
				notices: map[string]losableSideEffectNotice{key: {Reason: noticeReason, Fingerprint: fingerprint}},
				transactions: transactionBoundaryRegisterData{
					Admin: map[string]string{key: adminReason}, Commands: map[string]string{key: commandReason},
				},
			}
			writeRegisterGitSource(t, repo, "new/worker.go", "package fixture\nfunc run() { "+body+" }\n")
			if err := os.Remove(filepath.Join(repo, "backend/old/worker.go")); err != nil {
				t.Fatal(err)
			}
			if tc.wantErr {
				if tc.body != "" {
					requireGitRegisterErrors(t, repo, current, "losable-notice register may only shrink", "relocated transaction body changed")
					return
				}
				requireGitRegisterRejection(t, repo, current, "register")
				return
			}
			checkGitRegisters(t, repo, current)
		})
	}
}

func TestRegisterRatchetsAllowRetirement(t *testing.T) {
	t.Parallel()
	repo, _ := newRegisterGitRepo(t)
	checkGitRegisters(t, repo, registerGitFixture{})
}
