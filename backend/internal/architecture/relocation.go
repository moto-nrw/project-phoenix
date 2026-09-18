package architecture

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Relocation declares that one classified package moved to a new path in this
// policy epoch without becoming a different package. It is the fourth reviewed
// mechanism (ADR 0020), next to the test-infrastructure rule (ADR 0014), table
// adoption (ADR 0015) and workflow additions (ADR 0019): a structurally new
// move that the base comparison had no way to express.
//
// The move renames every violation key the package appears in, as source and
// as target, so the ordinary base comparison reports the package's own debt as
// resolved and every consumer's debt as new. Neither is true: no import, owner
// or role changed. Recording the renamed keys as fresh debt is impossible
// (they were forbidden at the base, so the import-debt conversion rejects
// them), and permitting them with rules would grant every package that shares
// the owner and role point a dependency it does not have.
//
// A declaration therefore lets the base baseline be read at the new path. It
// carries no permission: the relocated entries keep their exact scope, rule,
// counterpart and migration issue, and every other guard still applies.
type Relocation struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Issue string `json:"issue"`
}

func (p *Policy) validateRelocations() error {
	packages := make(map[string]struct{}, len(p.Packages))
	for _, pkg := range p.Packages {
		packages[pkg.Path] = struct{}{}
	}
	seenFrom := make(map[string]struct{}, len(p.Relocations))
	seenTo := make(map[string]struct{}, len(p.Relocations))
	previous := ""
	for _, relocation := range p.Relocations {
		for _, side := range []struct{ label, path string }{
			{label: "from", path: relocation.From},
			{label: "to", path: relocation.To},
		} {
			if side.path == "" || strings.HasPrefix(side.path, "/") || strings.Contains(side.path, "..") {
				return fmt.Errorf("relocation %s path %q must be relative to module_path", side.label, side.path)
			}
		}
		if relocation.From == relocation.To {
			return fmt.Errorf("relocation %q does not move the package", relocation.From)
		}
		if relocation.From < previous {
			return fmt.Errorf("relocation %q is not sorted by from path", relocation.From)
		}
		previous = relocation.From
		if _, exists := seenFrom[relocation.From]; exists {
			return fmt.Errorf("relocation from path %q is declared more than once", relocation.From)
		}
		if _, exists := seenTo[relocation.To]; exists {
			return fmt.Errorf("relocation to path %q is declared more than once", relocation.To)
		}
		seenFrom[relocation.From] = struct{}{}
		seenTo[relocation.To] = struct{}{}
		if _, exists := packages[relocation.From]; exists {
			return fmt.Errorf("relocated package %q is still classified", relocation.From)
		}
		if _, exists := packages[relocation.To]; !exists {
			return fmt.Errorf("relocation target %q has no owner or role", relocation.To)
		}
		if _, err := ParseGitHubIssue(relocation.Issue); err != nil {
			return fmt.Errorf("relocation %q: %w", relocation.From, err)
		}
	}
	return nil
}

// relocatedPackages returns the active relocations of this candidate as a map
// from the base import path to the candidate import path.
//
// A declaration is active while its source path is still classified in the
// base policy; afterwards it is a historical record and is ignored, so a later
// epoch cannot replay it. An active declaration must prove all of the
// following, and the candidate fails closed when it does not:
//
//   - the policy epoch increased, as for every reviewed mechanism;
//   - the source path is classified in the base policy and gone from the
//     candidate, and the target path is classified in the candidate and absent
//     from the base;
//   - owner, production role, internal-test role and external-test role are
//     identical on both sides;
//   - the target holds exactly the Go files the source held at the immutable
//     base commit, so the move is a rename of the package, not a rewrite of it.
func relocatedPackages(project, ref string, base, candidate *Policy) (map[string]string, error) {
	var active []Relocation
	basePackages := packagesByPath(base)
	candidatePackages := packagesByPath(candidate)
	for _, relocation := range candidate.Relocations {
		if _, classified := basePackages[relocation.From]; !classified {
			continue
		}
		active = append(active, relocation)
	}
	if len(active) == 0 {
		return nil, nil
	}
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return nil, fmt.Errorf("relocation of %s requires a reviewed policy epoch above %d", active[0].From, base.PolicyEpoch)
	}
	moves := make(map[string]string, len(active))
	for _, relocation := range active {
		if _, classified := candidatePackages[relocation.To]; !classified {
			return nil, fmt.Errorf("relocation target %s has no owner or role", relocation.To)
		}
		if _, existed := basePackages[relocation.To]; existed {
			return nil, fmt.Errorf("relocation target %s already existed at the base commit", relocation.To)
		}
		if problem := relocationClassificationChange(basePackages[relocation.From], candidatePackages[relocation.To]); problem != "" {
			return nil, fmt.Errorf("relocation %s -> %s %s", relocation.From, relocation.To, problem)
		}
		if err := requireRelocatedFiles(project, ref, relocation); err != nil {
			return nil, err
		}
		moves[base.absolutePackage(relocation.From)] = candidate.absolutePackage(relocation.To)
	}
	return moves, nil
}

func packagesByPath(policy *Policy) map[string]Package {
	packages := make(map[string]Package, len(policy.Packages))
	for _, pkg := range policy.Packages {
		packages[pkg.Path] = pkg
	}
	return packages
}

func relocationClassificationChange(from, to Package) string {
	for _, field := range []struct{ label, base, candidate string }{
		{label: "owner", base: from.Owner, candidate: to.Owner},
		{label: "production role", base: from.Role, candidate: to.Role},
		{label: "internal_test role", base: from.InternalTestRole, candidate: to.InternalTestRole},
		{label: "external_test role", base: from.ExternalTestRole, candidate: to.ExternalTestRole},
	} {
		if field.base != field.candidate {
			return fmt.Sprintf("changed %s from %s to %s", field.label, field.base, field.candidate)
		}
	}
	return ""
}

// requireRelocatedFiles proves the rename from Git rather than from the
// declaration. The Go files the source directory held at the immutable base
// commit must be exactly the Go files the target directory holds now. A
// relocation that adds, drops or renames a file is not a relocation; make that
// change in its own commit, where the ordinary guards see it.
func requireRelocatedFiles(project, ref string, relocation Relocation) error {
	root, err := gitOutput(project, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("resolve repository root for relocation: %w", err)
	}
	root = strings.TrimSpace(root)
	sha, err := resolveBaseCommit(root, ref)
	if err != nil {
		return err
	}
	source, err := repositoryRelativePath(root, project, filepath.FromSlash(relocation.From))
	if err != nil {
		return err
	}
	names, err := gitOutput(root, "ls-tree", "-r", "--name-only", "-z", sha, "--", filepath.ToSlash(source))
	if err != nil {
		return fmt.Errorf("list base files of %s: %w", relocation.From, err)
	}
	before := make(map[string]struct{})
	for _, name := range strings.Split(names, "\x00") {
		if filepath.Ext(name) != ".go" || filepath.Clean(filepath.Dir(name)) != filepath.Clean(source) {
			continue
		}
		before[filepath.Base(name)] = struct{}{}
	}
	after, err := goFileNames(filepath.Join(project, filepath.FromSlash(relocation.To)))
	if err != nil {
		return fmt.Errorf("read relocated package %s: %w", relocation.To, err)
	}
	if problem := fileSetDifference(before, after); problem != "" {
		return fmt.Errorf("relocation %s -> %s is not a rename: %s", relocation.From, relocation.To, problem)
	}
	return nil
}

func goFileNames(dir string) (map[string]struct{}, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		names[entry.Name()] = struct{}{}
	}
	return names, nil
}

func fileSetDifference(before, after map[string]struct{}) string {
	var missing, added []string
	for name := range before {
		if _, exists := after[name]; !exists {
			missing = append(missing, name)
		}
	}
	for name := range after {
		if _, exists := before[name]; !exists {
			added = append(added, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(added)
	var problems []string
	if len(missing) > 0 {
		problems = append(problems, "missing "+strings.Join(missing, ", "))
	}
	if len(added) > 0 {
		problems = append(problems, "added "+strings.Join(added, ", "))
	}
	return strings.Join(problems, "; ")
}

// relocateManifest rewrites the base baseline into the candidate's package
// paths. Only the source and target paths change; scope, rule, counterpart and
// migration issue stay exactly as recorded, so a relocated entry still has to
// match the candidate entry the ordinary comparison expects.
func relocateManifest(base *LegacyManifest, moves map[string]string) (*LegacyManifest, error) {
	if len(moves) == 0 {
		return base, nil
	}
	relocated := &LegacyManifest{byKey: make(map[string]LegacyEntry, len(base.Entries))}
	for _, entry := range base.Entries {
		if moved, exists := moves[entry.Source]; exists {
			entry.Source = moved
		}
		if moved, exists := moves[entry.Target]; exists {
			entry.Target = moved
		}
		if _, exists := relocated.byKey[entry.Key()]; exists {
			return nil, fmt.Errorf("relocation collapses two base entries onto %s", entry.Key())
		}
		relocated.byKey[entry.Key()] = entry
		relocated.Entries = append(relocated.Entries, entry)
	}
	sort.Slice(relocated.Entries, func(i, j int) bool { return relocated.Entries[i].Key() < relocated.Entries[j].Key() })
	return relocated, nil
}
