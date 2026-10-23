package passthrough

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Registry is a finite inventory, not a budget or an ADR 0075 blanket floor.
// Scope pins scan semantics against the PR base. A future semantic/scope change
// needs a new-numbered decision, just like an individual new/changed site.
type Registry struct {
	Scope    string `json:"scope"`
	Decision string `json:"decision"`
	Sites    []Site `json:"sites"`
}

func scope() string {
	return "all-backend/non-hidden/non-_test.go/exported-method/14-lines/one-receiver-field-call:v1"
}

// CompareRegisters prevents spending a deleted method's allowance elsewhere,
// or updating its hash/shape/owner to make a regression green. New decisions
// are supplied from actual ADR numbers absent at the immutable PR base.
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
		if (!exists || old != site) && !addedADRs[site.Decision] {
			problems = append(problems, "new/changed registration needs a new ADR: "+site.ID)
		}
	}
	sort.Strings(problems)
	return problems
}

// WriteSnapshot prints measured evidence for review. It never writes the
// registry itself and cannot authorize an addition. Check validates the result
// against the base and checks every cited new decision exists.
func WriteSnapshot(out io.Writer, root, decision string) error {
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
// An absent base inventory is a bootstrap, not permission to add debt: each
// entry must cite a newly added ADR, and removing that ADR turns the gate red.
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
		base, adrs, err := baseEvidence(root, baseRef)
		if err != nil {
			return err
		}
		problems = append(problems, CompareRegisters(base, candidate, adrs)...)
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
	data, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, err
	}
	var registry Registry
	decoder := json.NewDecoder(strings.NewReader(string(data)))
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
	listing, err := git(root, "ls-tree", "-r", "--name-only", sha, "--", path)
	if err != nil {
		return Registry{}, nil, err
	}
	var base Registry
	if strings.TrimSpace(string(listing)) != "" {
		data, err := git(root, "show", sha+":"+path)
		if err != nil {
			return Registry{}, nil, err
		}
		if err := json.Unmarshal(data, &base); err != nil {
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
	hostPath, err := filepath.Abs(host)
	if err != nil {
		return err
	}
	projectPath, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	if hostPath != projectPath {
		return nil
	}
	return Check(projectPath, baseRef)
}
