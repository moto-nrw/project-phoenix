package passthrough

import (
	"embed"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

// Registry is a finite inventory, not a budget or an ADR 0075 blanket floor.
// Scope pins scan semantics against the PR base. A future semantic/scope change
// needs a new-numbered decision, just like a new or worsened debt form.
type Registry struct {
	Scope    string `json:"scope"`
	Decision string `json:"decision"`
	Sites    []Site `json:"sites"`
}

// Embed the code actually compiled into the scanner, including all roots,
// filters, classification and comparison logic. Even a refreshed snapshot
// cannot conceal a source change from the immutable base scope.
//
//go:embed predicate.go scanner.go forms.go register.go
var scanSources embed.FS

func scope() string {
	digest := fnv.New128a() // Change detector, not a cryptographic trust boundary.
	_, _ = digest.Write([]byte(artifacts.SourceFingerprint()))
	files, _ := scanSources.ReadDir(".") // Embedded files always exist, sorted by name.
	for _, file := range files {
		source, _ := scanSources.ReadFile(file.Name())
		_, _ = fmt.Fprintf(digest, "%s:%d:", file.Name(), len(source))
		_, _ = digest.Write(source)
	}
	return fmt.Sprintf("all-backend/non-hidden/non-_test.go/exported-method/%d-lines/one-receiver-field-call:fnv128a:%x", MaxBodyLines, digest.Sum(nil))
}

// CompareRegisters requires a newly numbered ADR only for a changed scan scope
// or new/worsened debt forms. Body/reason/owner changes remain visible in the
// source-checked register diff but do not spend an ADR number (ADR 0057).
func CompareRegisters(base, candidate Registry, addedADRs map[string]bool) []string {
	var problems []string
	if base.Scope != candidate.Scope && !addedADRs[candidate.Decision] {
		problems = append(problems, "scan selectors changed without a new ADR")
	}
	previous := map[string]Site{}
	for _, site := range base.Sites {
		previous[site.ID] = site
	}
	for _, site := range candidate.Sites {
		old, exists := previous[site.ID]
		debt := site.Form == "forward-debt" || site.Form == "guard-debt"
		if debt && (!exists || old.Form != site.Form) && !addedADRs[site.Decision] {
			problems = append(problems, "new/worsened debt needs a new ADR: "+site.ID)
		}
	}
	sort.Strings(problems)
	return problems
}

// Check removals directly in each base package, independently of Scan's tree
// traversal. A selector excluding api/ must not masquerade as deleted methods,
// even when a new scope ADR is present. The head predicate must prove that the
// method was deleted or no longer meets the pass-through conditions.
func checkRemovedSites(root string, base, candidate []Site) ([]string, error) {
	present := map[string]bool{}
	for _, site := range candidate {
		present[site.ID] = true
	}
	packages := map[string]bool{}
	for _, site := range base {
		if !present[site.ID] {
			packages[filepath.Dir(site.ID)] = true
		}
	}
	remaining := map[string]bool{}
	for dir := range packages {
		sites, err := packageSites(root, dir)
		if err != nil {
			return nil, err
		}
		for _, site := range sites {
			remaining[site.ID] = true
		}
	}

	var problems []string
	for _, site := range base {
		if !present[site.ID] && remaining[site.ID] {
			problems = append(problems, "disappeared method still matches: "+site.ID)
		}
	}
	sort.Strings(problems)
	return problems, nil
}

// Read a base package directly so Scan's tree filters cannot hide its methods.
func packageSites(root, dir string) ([]Site, error) {
	path := filepath.Join(root, dir)
	files, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sites []Site
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		found, err := scanFile(root, filepath.Join(path, file.Name()), nil)
		if err != nil {
			return nil, err
		}
		sites = append(sites, found...)
	}
	return sites, nil
}

func compareBase(root, baseRef string, candidate Registry) ([]string, error) {
	base, adrs, err := baseEvidence(root, baseRef)
	if err != nil {
		return nil, err
	}
	removed, err := checkRemovedSites(root, base.Sites, candidate.Sites)
	if err != nil {
		return nil, err
	}
	return append(CompareRegisters(base, candidate, adrs), removed...), nil
}

// WriteSnapshot prints measured evidence for review. It never writes the
// registry itself and cannot authorize new debt. Check validates the result
// against the base and verifies each required newly numbered decision.
func WriteSnapshot(out io.Writer, root, decision string) error {
	if decision == "" {
		current, err := readRegistry(filepath.Join(root, "architecture/passthrough.json"))
		if err != nil {
			return fmt.Errorf("load default snapshot decision: %w", err)
		}
		decision = current.Decision
	}
	sites, err := Scan(root)
	if err != nil {
		return err
	}
	prepareSnapshot(sites, decision)
	registry := Registry{Scope: scope(), Decision: decision, Sites: sites}
	// One exact method per line keeps generated register diffs reviewable.
	header, err := json.Marshal(registry.Scope)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "{\n  \"scope\": %s,\n  \"decision\": %q,\n  \"sites\": [\n", header, decision); err != nil {
		return err
	}
	for i, site := range sites {
		data, err := json.Marshal(site)
		if err != nil {
			return err
		}
		suffix := ","
		if i == len(sites)-1 {
			suffix = ""
		}
		if _, err := fmt.Fprintf(out, "    %s%s\n", data, suffix); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out, "  ]\n}")
	return err
}

// Check validates source evidence and the register's immutable-base delta.
// An absent base inventory is a bootstrap, not permission to add debt: its
// scope and initial debt need newly added ADRs; removing them turns the gate red.
func Check(root, baseRef string) error {
	candidate, err := readRegistry(filepath.Join(root, "architecture/passthrough.json"))
	if err != nil {
		return err
	}
	measured, err := Scan(root)
	if err != nil {
		return err
	}
	problems := CompareSites(candidate.Sites, measured)
	if candidate.Scope != scope() {
		problems = append(problems, "registry scope does not match scanner selectors")
	}
	for _, site := range candidate.Sites {
		if site.Reason == "" || site.Decision == "" {
			problems = append(problems, "missing reason/decision: "+site.ID)
		}
	}
	if baseRef != "" {
		baseProblems, err := compareBase(root, baseRef, candidate)
		if err != nil {
			return err
		}
		problems = append(problems, baseProblems...)
	}
	if len(problems) != 0 {
		return fmt.Errorf("method pass-through evidence failed:\n  %s", strings.Join(problems, "\n  "))
	}
	counts := map[string]int{}
	for _, site := range measured {
		counts[site.Build+"/"+site.Form]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Printf("method pass-through evidence passed: %d exact sites\n", len(measured))
	for _, key := range keys {
		fmt.Printf("  %s: %d\n", key, counts[key])
	}
	return nil
}

func readRegistry(path string) (Registry, error) {
	stored, err := artifacts.ReadInventory(path, artifacts.Passthrough, true)
	if err != nil {
		return Registry{}, err
	}
	var registry Registry
	decoder := json.NewDecoder(strings.NewReader(string(stored.Document)))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&registry)
	return registry, err
}

var adrName = regexp.MustCompile(`^(\d{4})-.*\.md$`)

func baseEvidence(root, baseRef string) (Registry, map[string]bool, error) {
	resolved, err := git(root, "rev-parse", "--verify", baseRef+"^{commit}")
	if err != nil {
		return Registry{}, nil, err
	}
	sha := strings.TrimSpace(string(resolved))
	prefix, err := git(root, "rev-parse", "--show-prefix")
	if err != nil {
		return Registry{}, nil, err
	}
	path := strings.TrimSpace(string(prefix)) + "architecture/passthrough.json"
	// ls-tree resolves its pathspec relative to root; show needs the full
	// repository-relative path computed above.
	listing, err := git(root, "ls-tree", "-r", "--name-only", sha, "--", "architecture/passthrough.json", "architecture/passthrough")
	if err != nil {
		return Registry{}, nil, err
	}
	var base Registry
	if strings.TrimSpace(string(listing)) != "" {
		repo, err := git(root, "rev-parse", "--show-toplevel")
		if err != nil {
			return Registry{}, nil, err
		}

		stored, err := artifacts.ReadInventoryAtRevision(strings.TrimSpace(string(repo)), sha, filepath.ToSlash(filepath.Dir(path)), filepath.Base(path), artifacts.Passthrough, false)
		if err != nil {
			return Registry{}, nil, err
		}
		if err := json.Unmarshal(stored.Document, &base); err != nil {
			return Registry{}, nil, err
		}
	}
	adrs, err := newADRs(root, sha)
	return base, adrs, err
}

func newADRs(root, sha string) (map[string]bool, error) {
	// Use the repository root so backend-relative paths and nested callers do
	// not alter the set of new ADR numbers.
	repo, err := git(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	repoRoot := strings.TrimSpace(string(repo))
	listing, err := git(repoRoot, "ls-tree", "-r", "--name-only", sha, "--", "docs/adr/")
	if err != nil {
		return nil, err
	}
	existing := map[string]bool{}
	for _, path := range strings.Split(string(listing), "\n") {
		if match := adrName.FindStringSubmatch(filepath.Base(path)); match != nil {
			existing[match[1]] = true
		}
	}
	files, err := filepath.Glob(filepath.Join(repoRoot, "docs/adr/*.md"))
	if err != nil {
		return nil, err
	}
	added := map[string]bool{}
	for _, path := range files {
		if match := adrName.FindStringSubmatch(filepath.Base(path)); match != nil && !existing[match[1]] {
			added[match[1]] = true
		}
	}
	return added, nil
}

func git(root string, args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %v: %w: %s", args, err, output)
	}
	return output, nil
}

func prepareSnapshot(sites []Site, decision string) {
	for i := range sites {
		sites[i].Decision = decision
		sites[i].Reason = "Exact evidence; ADR 0075 eligibility unchanged; semantic form review pending."
		if sites[i].Form == "forward-debt" || sites[i].Form == "guard-debt" {
			sites[i].Reason = "Finite named forwarding debt under ADR 0090, not a new floor; removal/deepening pending."
		}
		if sites[i].Form == "adr0075-named-exception" {
			sites[i].Reason = "ADR 0075 amendment: transaction-boundary names or Settings HTTP policy edge block removal."
		}
	}
}

// CheckProject applies the host repository's inventory only when that host is
// being checked. The generic CLI also checks independent synthetic projects,
// some of which deliberately use moto's module path. They have their own
// architecture contracts, not the host's finite source inventory.
func CheckProject(host, project, baseRef string) error {
	hostPath, err := filepath.EvalSymlinks(host)
	if err != nil {
		return err
	}
	projectPath, err := filepath.EvalSymlinks(project)
	if err != nil {
		return err
	}
	hostPath, err = filepath.Abs(hostPath)
	if err != nil {
		return err
	}
	projectPath, err = filepath.Abs(projectPath)
	if err != nil {
		return err
	}
	if hostPath != projectPath {
		return nil
	}
	return Check(projectPath, baseRef)
}
