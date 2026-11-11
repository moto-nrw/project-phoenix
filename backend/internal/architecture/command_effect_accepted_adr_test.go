package architecture

import (
	"path/filepath"
	"strings"
	"testing"
)

// ADR 0206: a decision proposed in a documentation change approves the
// command effects of the change that accepts it, and nothing else does.
func TestAcceptedADRsNeedTheAcceptanceInThisChange(t *testing.T) {
	t.Parallel()
	const proposed, accepted, historical = "---\nstatus: proposed\n---\n\n# ADR\n", "---\nstatus: accepted\n---\n\n# ADR\n", "---\nstatus: historical\n---\n\n# ADR\n"
	for _, tt := range []struct {
		name       string
		base, head string
		want       bool
	}{
		{"accepted in this change", proposed, accepted, true},
		{"accepted before", accepted, accepted + "\nAddendum.\n", false},
		{"still proposed", proposed, proposed + "\nMore.\n", false},
		{"historical revived", historical, accepted, false},
		{"status only in the body", "# ADR\n\nstatus: proposed\n", "# ADR\n\nstatus: accepted\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepository(t, legacyRecord(2583))
			path := filepath.Join(repo, "docs", "adr", "0009-decision.md")
			writeFile(t, path, tt.base)
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "ADR base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			writeFile(t, path, tt.head)
			got, err := acceptedADRs(repo, base)
			if err != nil {
				t.Fatal(err)
			}
			if got["0009"] != tt.want {
				t.Fatalf("accepted ADR 0009 = %v, want %v", got["0009"], tt.want)
			}
		})
	}
}

func TestAcceptedADRsIgnoreANewFile(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(repo, "docs", "adr", "0010-new.md"), "---\nstatus: accepted\n---\n\n# New\n")
	got, err := acceptedADRs(repo, base)
	if err != nil {
		t.Fatal(err)
	}
	if got["0010"] {
		t.Fatal("a new ADR is addedADRs' decision, not an acceptance")
	}
}
