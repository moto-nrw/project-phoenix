package passthrough_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/passthrough"
)

func put(t *testing.T, root, path, source string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func scan(t *testing.T, root string) []passthrough.Site {
	t.Helper()
	sites, err := passthrough.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	return sites
}

func TestPassthroughRejectsSameOwnerSwap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := "modules/example/internal/application/service.go"
	put(t, root, path, `package application
func (s *Service) Find(ctx Context) (Public, error) {
 row, err := s.store.Find(ctx)
 return Public{ID: row.ID}, err
}`)
	before := scan(t, root)
	if len(before) != 1 {
		t.Fatalf("before: %v", before)
	}
	put(t, root, path, `package application
func (s *Service) Find(ctx Context) (Public, error) { return s.store.Find(ctx) }`)
	after := scan(t, root)
	if len(after) != len(before) {
		t.Fatal("fixture must keep the owner's sum unchanged")
	}
	if problems := passthrough.CompareSites(before, after); len(problems) == 0 {
		t.Fatal("translation replaced by unchanged forward must fail despite equal owner sum")
	}
}

func TestPassthroughRejectsRelocationToRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := "modules/example/internal/application/service.go"
	source := `package application
func (s *Service) Find(ctx Context) (Public, error) { return s.store.Find(ctx) }`
	put(t, root, path, source)
	registered := scan(t, root)
	if err := os.Remove(filepath.Join(root, path)); err != nil {
		t.Fatal(err)
	}
	put(t, root, "roots/owners/service.go", source)
	measured := scan(t, root)
	if len(measured) != 1 || measured[0].Owner != "roots" {
		t.Fatalf("relocated site hidden: %v", measured)
	}
	if problems := passthrough.CompareSites(registered, measured); len(problems) == 0 {
		t.Fatal("relocation must not spend the previous owner's evidence")
	}
}

func TestPassthroughSeparatesHooksAndFixtures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := `package application
func (s *Service) Find(ctx Context) (Public, error) { return s.store.Find(ctx) }`
	put(t, root, "modules/example/internal/application/service.go", source)
	put(t, root, "modules/example/internal/application/hooks.go", "//go:build testhooks && !windows\n\n"+source)
	put(t, root, "test/helpers.go", source)
	sites := scan(t, root)
	counts := map[string]int{}
	for _, site := range sites {
		counts[site.Build]++
	}
	if counts["production"] != 1 || counts["testhooks"] != 1 || counts["test-support"] != 1 {
		t.Fatalf("production must not absorb hook/fixture costs: %v", counts)
	}
}

func TestPassthroughGuardsAndRenamesAreNotTranslations(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"guard":  `if s.store == nil { return nil }; return s.store.Find(ctx)`,
		"rename": `return s.store.Renamed(ctx)`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			put(t, root, "roots/owners/port.go", "package owners\nfunc (s *Port) Find(ctx Context) error {"+body+"}")
			sites := scan(t, root)
			want := "forward-debt"
			if name == "guard" {
				want = "guard-debt"
			}
			if len(sites) != 1 || sites[0].Form != want {
				t.Fatalf("want %s, got %v", want, sites)
			}
		})
	}
}

func TestPassthroughRegisterCannotBeRebaselined(t *testing.T) {
	t.Parallel()
	old := passthrough.Registry{Scope: "all-backend-v1", Sites: []passthrough.Site{{ID: "modules/example/Service.Find", Evidence: "old", Decision: "0075"}}}
	for name, candidate := range map[string]passthrough.Registry{
		"same-size swap":     {Scope: old.Scope, Sites: []passthrough.Site{{ID: "modules/example/Service.Other", Evidence: "new", Decision: "0075"}}},
		"fingerprint update": {Scope: old.Scope, Sites: []passthrough.Site{{ID: "modules/example/Service.Find", Evidence: "new", Decision: "0075"}}},
		"narrow selectors":   {Scope: "modules-only", Sites: old.Sites},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if problems := passthrough.CompareRegisters(old, candidate, map[string]bool{}); len(problems) == 0 {
				t.Fatal("unreviewed register change accepted")
			}
		})
	}
}

func TestPassthroughEvidenceSurvivesFormattingAndFileRename(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := "roots/owners/one.go"
	put(t, root, path, `package owners
// Find reads a record.
func (s *Port) Find(ctx Context) error {
 return s.store.Find(ctx)
}`)
	before := scan(t, root)
	if err := os.Remove(filepath.Join(root, path)); err != nil {
		t.Fatal(err)
	}
	put(t, root, "roots/owners/two.go", `package owners
func (s *Port) Find(ctx Context) error {

 // A changed comment is not changed behavior.
 return s.store.Find(ctx)

}`)
	if problems := passthrough.CompareSites(before, scan(t, root)); len(problems) != 0 {
		t.Fatalf("format-only change: %v", problems)
	}
}

func TestPassthroughBuildConstraintsDoNotHideProduction(t *testing.T) {
	t.Parallel()
	for tag, want := range map[string]string{
		"testhooks && linux":                "testhooks",
		"testhooks || linux":                "production",
		"!testhooks":                        "production",
		"!other && testhooks":               "testhooks",
		"testhooks && (!windows || darwin)": "testhooks",
		"windows && !windows":               "production",
	} {
		t.Run(tag, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			put(t, root, "roots/owners/port.go", "//go:build "+tag+"\n\npackage owners\nfunc (s *Port) Find() error { return s.store.Find() }")
			sites := scan(t, root)
			if len(sites) != 1 || sites[0].Build != want {
				t.Fatalf("want %s, got %v", want, sites)
			}
		})
	}
}

func TestPassthroughKeepsOnlyExactADR0075Exceptions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, method := range []string{"MarkFileDeleted", "MarkQueuedCleanupCompleteByFilename", "ActivateQueuedCleanup"} {
		put(t, root, "modules/careplan/internal/application/"+method+".go", "package application\nfunc (s DocumentFileRecords) "+method+"() error { return s.store.Renamed() }")
	}
	put(t, root, "modules/settings/internal/application/schema.go", "package application\nfunc (s TenantOperations) Schema() (any,error) { return s.store.GetSchema() }")
	put(t, root, "roots/owners/impostor.go", "package owners\nfunc (s TenantOperations) Schema() (any,error) { return s.store.GetSchema() }")
	counts := map[string]int{}
	for _, site := range scan(t, root) {
		counts[site.Form]++
	}
	if counts["adr0075-named-exception"] != 4 || counts["forward-debt"] != 1 {
		t.Fatalf("exceptions broadened/lost: %v", counts)
	}
}

func TestPassthroughIncludesFutureTreesAndLegacy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := "package example\nfunc (s *Port) Find() error { return s.store.Find() }"
	for _, path := range []string{"future/tree/service.go", "modules/example/legacy/service.go", "roots/owners/generated.go", "modules/example/compose/service.go", "workflows/example/internal/application/service.go"} {
		put(t, root, path, source)
	}
	put(t, root, ".hidden/service.go", source)
	put(t, root, "roots/owners/service_test.go", source)
	sites := scan(t, root)
	if len(sites) != 5 {
		t.Fatalf("scope silently narrowed: %v", sites)
	}
}

func TestPassthroughCLIRequiresHostInventoryButNotForeignFixtures(t *testing.T) {
	t.Parallel()
	host := t.TempDir()
	if err := passthrough.CheckProject(host, filepath.Join(host, "."), ""); err == nil {
		t.Fatal("missing host inventory must fail, including explicit --project host")
	}
	// A generic CLI fixture can legitimately use moto's module identity too.
	foreign := t.TempDir()
	put(t, foreign, "go.mod", "module github.com/moto-nrw/project-phoenix\n")
	if err := passthrough.CheckProject(host, foreign, ""); err != nil {
		t.Fatalf("independent synthetic project requires host inventory: %v", err)
	}
}

func TestPassthroughSnapshotExportsReviewableFiniteEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	put(t, root, "roots/owners/port.go", "package owners\n// A comment is not evidence.\nfunc (s *Port) Find(ctx Context) error { return s.store.Renamed(ctx) }")
	var output bytes.Buffer
	if err := passthrough.WriteSnapshot(&output, root, "0090"); err != nil {
		t.Fatal(err)
	}
	var registry passthrough.Registry
	if err := json.Unmarshal(output.Bytes(), &registry); err != nil {
		t.Fatal(err)
	}
	if registry.Scope != "all-backend/non-hidden/non-_test.go/exported-method/14-lines/one-receiver-field-call:v1" {
		t.Fatalf("unexpected scanner scope: %s", registry.Scope)
	}
	if len(registry.Sites) != 1 {
		t.Fatalf("exported sites: %v", registry.Sites)
	}
	site := registry.Sites[0]
	if site.ID != "roots/owners/Port.Find" || site.Form != "forward-debt" || site.Decision != "0090" || site.Reason == "" {
		t.Fatalf("missing finite decision evidence: %+v", site)
	}
	if !strings.Contains(site.Evidence, "s.store.Renamed(ctx)") || strings.Contains(site.Evidence, "comment") {
		t.Fatalf("unreviewable normalized evidence: %q", site.Evidence)
	}
}
