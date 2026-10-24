package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projectionRelocationRepository(t *testing.T) (string, string, *Policy) {
	t.Helper()
	repo := t.TempDir()
	p := studentProjectionPolicy(t)
	writeFile(t, filepath.Join(repo, "go.mod"), "module github.com/moto-nrw/project-phoenix\n\ngo 1.25\n")
	for _, pkg := range p.Packages {
		writeFile(t, filepath.Join(repo, pkg.Path, "projection.go"), "package projection\n")
	}
	writeStudentProjectionPolicy(t, repo, p)
	writeFile(t, filepath.Join(repo, "architecture/legacy.jsonl"), "")
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "architecture-test@example.test")
	runGit(t, repo, "config", "user.name", "Architecture Test")
	runGit(t, repo, "config", "commit.gpgsign", "false")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "projection base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	from, to := p.Packages[0].Path, "modules/parentmessageinbox/internal/postgres"
	writeFile(t, filepath.Join(repo, to, "projection.go"), readFile(t, filepath.Join(repo, from, "projection.go")))
	if err := os.Remove(filepath.Join(repo, from, "projection.go")); err != nil {
		t.Fatal(err)
	}
	p.PolicyEpoch++
	p.Packages[0].Path = to
	p.ReadProjections[0].Package = to
	p.Relocations = []Relocation{{From: from, To: to, Issue: "https://github.com/moto-nrw/project-phoenix/issues/4086"}}
	return repo, base, p
}

func TestProjectionGrantAcceptsOnlyVerifiedPathRelocation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*Policy)
		want   string
	}{
		{"path only", func(*Policy) {}, ""},
		{"extra table", func(p *Policy) {
			p.ReadProjections[0].DataObjects = append(p.ReadProjections[0].DataObjects, "users.privacy_consents")
		}, "new tenant-safe read projection grant"},
		{"unsafe privilege", func(p *Policy) { p.ReadProjections[0].TenantSafe = false }, "must be explicitly tenant-safe"},
		{"changed identity", func(p *Policy) { p.ReadProjections[0].ID = "different-grant" }, "new tenant-safe read projection grant"},
		{"unverified path", func(p *Policy) { p.Relocations = nil }, "new tenant-safe read projection grant"},
		{"changed owner", func(p *Policy) { p.Packages[0].Owner = "parent-announcement-audience" }, "changed owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, base, p := projectionRelocationRepository(t)
			tc.change(p)
			raw, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(repo, "architecture/policy.json"), string(raw))
			runGit(t, repo, "add", "-A")
			output, err := runRepositoryCheck(t, repo, base)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("pure relocation rejected: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(output, tc.want) {
				t.Fatalf("expected %q, got %v\n%s", tc.want, err, output)
			}
		})
	}
}
