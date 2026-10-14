package architecture

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompositionSnapshotsReuseSyntaxWithoutSharingResolution(t *testing.T) {
	t.Parallel()
	base := newCompositionSnapshot()
	setter := []byte("package services\ntype Service struct { port Port }\nfunc (s *Service) Configure(p Port) { s.port = p }\n")
	if err := base.add("services/service.go", setter); err != nil {
		t.Fatal(err)
	}
	if err := base.add("services/port.go", []byte("package services\ntype Port struct { count int }\n")); err != nil {
		t.Fatal(err)
	}
	candidate := newCompositionSnapshotSharingSyntax(base)
	if err := candidate.add("services/service.go", append([]byte(nil), setter...)); err != nil {
		t.Fatal(err)
	}
	if err := candidate.add("services/port.go", []byte("package services\ntype Port interface { Run() }\n")); err != nil {
		t.Fatal(err)
	}
	if base.files[0].syntax != candidate.files[0].syntax || base.fset != candidate.fset {
		t.Fatal("byte-identical source was parsed twice")
	}
	if got := base.inventory(); len(got) != 0 {
		t.Fatalf("candidate type change leaked into base resolution: %v", got)
	}
	current := candidate.inventory()
	key := "production|setter|services|Service.Configure->Service.port"
	if len(current) != 1 || current[key] != "services/service.go:3:39" {
		t.Fatalf("unchanged setter did not use candidate type resolution: %v", current)
	}
}

func TestCompositionSharedSyntaxRetainsFreshParsingAndLocations(t *testing.T) {
	t.Parallel()
	base := newCompositionSnapshot()
	if err := base.add("services/factory.go", []byte("package services\ntype Factory struct { Old int }\n")); err != nil {
		t.Fatal(err)
	}
	candidate := newCompositionSnapshotSharingSyntax(base)
	changed := []byte("\npackage services\ntype Factory struct { Old, New int }\n")
	for _, path := range []string{"services/factory.go", "services/factory_test.go"} {
		if err := candidate.add(path, changed); err != nil {
			t.Fatal(err)
		}
	}
	accepted, current := base.inventory(), candidate.inventory()
	if accepted["production|field|services|Factory.Old"] != "services/factory.go:2:6" {
		t.Fatalf("candidate parse replaced base locations: %v", accepted)
	}
	if _, found := accepted["production|field|services|Factory.New"]; found {
		t.Fatal("candidate field leaked into base inventory")
	}
	if current["production|field|services|Factory.New"] != "services/factory.go:3:6" ||
		current["internal_test|field|services|Factory.New"] != "services/factory_test.go:3:6" {
		t.Fatalf("changed or relocated source reused stale positions/scopes: %v", current)
	}
}

func TestCompositionReuseStillRejectsBuildExcludedSyntax(t *testing.T) {
	t.Parallel()
	for _, changed := range []bool{false, true} {
		name := "new"
		if changed {
			name = "changed"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepository(t, legacyRecord(2583))
			path := filepath.Join(repo, "services", "excluded.go")
			if changed {
				writeFile(t, path, "//go:build excluded_fixture\n\npackage services\nvar value int\n")
			}
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "--allow-empty", "-qm", "composition source base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			writeFile(t, path, "//go:build excluded_fixture\n\npackage services\nfunc Broken( {\n")
			err := CompareCompositionSurface(repo, base)
			if err == nil || !strings.Contains(err.Error(), "parse composition source:") || !strings.Contains(err.Error(), "services/excluded.go:") {
				t.Fatalf("build-excluded candidate syntax was not rejected: %v", err)
			}
		})
	}
}
