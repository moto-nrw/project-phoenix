package architecture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moto-ogs/project-phoenix/architecture/artifacts"
)

type effectBase struct {
	root, sha    string
	addedADRs    map[string]bool
	domainCensus bool
}

// CompareCommandEffects checks reviewed effects against the immutable PR base.
// New leaves need exact evidence; expanded exceptions need a newly added ADR.
//
// It reads the base register through rewrite. A non-nil
// rewrite is an approved module rename whose Go diff is proven to be the pure
// rename (ADR 0217): base digests of the same identity then hash the same
// sources under the old path, so the candidate digest replaces them.
func CompareCommandEffects(project, ref string, rewrite *baseRewrite) error {
	candidate, err := loadCommandEffects(project)
	if err != nil {
		return err
	}
	context, err := resolveEffectBase(project, ref)
	if err != nil {
		return err
	}
	base, bootstrap, err := loadBaseCommandEffects(project, context, rewrite)
	if err != nil {
		return err
	}
	if rewrite != nil {
		adoptRenamedDigests(base, candidate)
	}
	if bootstrap {
		context.addedADRs["0083"] = true
	} else {
		expansion, err := newDomainEffectAddendum(context)
		if err != nil {
			return err
		}
		context.domainCensus = expansion
	}
	failures, added, changed := compareEffectEntries(base, candidate, context.addedADRs, context.domainCensus)
	if len(failures) > 0 {
		return fmt.Errorf("command effects against base %s:\n  %s", context.sha, strings.Join(failures, "\n  "))
	}
	fmt.Printf("command effects: %d entries (%d reviewed unresolved); %d added, %d changed against base %s\n", len(candidate.Entries), candidate.unresolvedCount(), added, changed, context.sha)
	return nil
}

func resolveEffectBase(project, ref string) (effectBase, error) {
	rootOutput, err := gitOutput(project, "rev-parse", "--show-toplevel")
	if err != nil {
		return effectBase{}, err
	}
	root := strings.TrimSpace(rootOutput)
	sha, err := resolveBaseCommit(root, ref)
	if err != nil {
		return effectBase{}, err
	}
	adrs, err := addedADRs(root, sha)
	if err != nil {
		return effectBase{}, err
	}
	accepted, err := acceptedADRs(root, sha)
	if err != nil {
		return effectBase{}, err
	}
	for number := range accepted {
		adrs[number] = true
	}
	return effectBase{root: root, sha: sha, addedADRs: adrs}, nil
}

// acceptedADRs lists the ADRs this change accepts: the base holds the file
// with front-matter status proposed, the candidate the same file with status
// accepted. A decision proposed in an earlier documentation change is
// accepted in the change that implements it (ADR 0206), so it approves that
// change's command effects like a new ADR would. It approves no ratchet
// budget raise.
func acceptedADRs(root, sha string) (map[string]bool, error) {
	changed, err := gitOutput(root, "diff", "--name-only", "-z", sha, "--", "docs/adr/")
	if err != nil {
		return nil, err
	}
	accepted := map[string]bool{}
	for _, name := range strings.Split(changed, "\x00") {
		number, ok := adrNumber(filepath.Base(name))
		if !ok || filepath.Dir(name) != "docs/adr" {
			continue
		}
		accepts, err := acceptsADR(root, sha, name)
		if err != nil {
			return nil, err
		}
		if accepts {
			accepted[number] = true
		}
	}
	return accepted, nil
}

// acceptsADR reports whether the candidate's file name is accepted while the
// base holds the same file as proposed.
func acceptsADR(root, sha, name string) (bool, error) {
	source, err := os.ReadFile(filepath.Join(root, name))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if adrStatus(string(source)) != "accepted" {
		return false, nil
	}
	base, err := gitOutput(root, "show", sha+":"+name)
	if err != nil {
		return false, nil // Not in the base under this name: addedADRs decides.
	}
	return adrStatus(base) == "proposed", nil
}

// adrStatus is the status of the ADR's leading front matter, empty without.
func adrStatus(source string) string {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			return ""
		}
		if value, ok := strings.CutPrefix(line, "status:"); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

const commandEffectDecision = "docs/adr/0083-ratchets-protect-invariants-and-expose-command-effects.md"
const commandEffectAddendum = "\n## Implementation addendum 2026-10-23 (#4739)\n"

func loadBaseCommandEffects(project string, base effectBase, rewrite *baseRewrite) (*commandEffectManifest, bool, error) {
	relative, err := repositoryRelativePath(base.root, project, commandEffectInventory)
	if err != nil {
		return nil, false, err
	}
	listing, err := gitOutput(base.root, "ls-tree", "--name-only", base.sha, "--", filepath.ToSlash(relative), filepath.ToSlash(strings.TrimSuffix(relative, ".json")))
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(listing) == "" {
		if err := validateEffectBootstrap(base); err != nil {
			return nil, false, err
		}
		return &commandEffectManifest{SchemaVersion: 1}, true, nil
	}

	stored, err := artifacts.ReadInventoryAtRevision(base.root, base.sha, filepath.ToSlash(filepath.Dir(relative)), filepath.Base(relative), artifacts.CommandEffects, false)
	if err != nil {
		return nil, false, err
	}
	manifest, err := decodeCommandEffects(rewrite.apply(stored.Document))
	return manifest, false, err
}

func adoptRenamedDigests(base, candidate *commandEffectManifest) {
	digests := make(map[string]string, len(candidate.Entries))
	for _, entry := range candidate.Entries {
		digests[entry.key()] = entry.BindingDigest
	}
	for index, entry := range base.Entries {
		if digest, ok := digests[entry.key()]; ok {
			base.Entries[index].BindingDigest = digest
		}
	}
}

// Only the first register may cite the new implementation addendum. Once a
// base holds the register, future exception growth still needs a new ADR number.
func validateEffectBootstrap(base effectBase) error {
	candidate, err := os.ReadFile(filepath.Join(base.root, commandEffectDecision))
	if err != nil {
		return fmt.Errorf("read command effect bootstrap decision: %w", err)
	}
	var previous []byte
	if !base.addedADRs["0083"] {
		previous, err = readGitBlob(base.root, base.root, base.sha, commandEffectDecision)
		if err != nil {
			return err
		}
	}
	if !strings.Contains(string(candidate), commandEffectAddendum) || strings.Contains(string(previous), commandEffectAddendum) {
		return fmt.Errorf("command effect bootstrap requires the new ADR 0083 #4739 implementation addendum")
	}
	return nil
}

const commandEffectDomainAddendum = "\n## Domain-source expansion 2026-10-23 (#4739)\n"

// The new census decision covers previously unscanned owners and newly exposed
// providers/diagnostics of already reviewed bindings, never existing changes or
// growth once the decision is present at the immutable base.
func newDomainEffectAddendum(base effectBase) (bool, error) {
	candidate, err := os.ReadFile(filepath.Join(base.root, commandEffectDecision))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !strings.Contains(string(candidate), commandEffectDomainAddendum) {
		return false, nil
	}
	previous, err := readGitBlob(base.root, base.root, base.sha, commandEffectDecision)
	if err != nil {
		return false, err
	}
	return !strings.Contains(string(previous), commandEffectDomainAddendum), nil
}

type effectBaseReview struct {
	previous        map[string]commandEffectEntry
	coveredOwners   map[string]bool
	ports, bindings map[string]bool
	addedADRs       map[string]bool
	domainCensus    bool
}

func newEffectBaseReview(base *commandEffectManifest, adrs map[string]bool, domainCensus bool) effectBaseReview {
	review := effectBaseReview{
		previous: map[string]commandEffectEntry{}, coveredOwners: map[string]bool{},
		ports: map[string]bool{}, bindings: map[string]bool{}, addedADRs: adrs, domainCensus: domainCensus,
	}
	for _, entry := range base.Entries {
		review.previous[entry.key()] = entry
		review.coveredOwners[entry.sourceOwner()] = true
		review.bindings[entry.sourceOwner()+"|"+entry.Port+"|"+entry.Binding] = true
		review.ports[entry.reviewedPortKey()] = true
	}
	return review
}

func (review effectBaseReview) approves(entry commandEffectEntry, existed bool) bool {
	if review.addedADRs[entry.ApprovalADR] {
		return true
	}
	if existed || entry.ApprovalADR != "0083" || !review.domainCensus {
		return false
	}
	if !review.coveredOwners[entry.sourceOwner()] && entry.sourceOwner() != "enrollment" {
		return true
	}
	if review.ports[entry.reviewedPortKey()] {
		return true
	}
	return entry.Effect == "unresolved" && entry.Classification == "unresolved" && entry.ADR == "0083" && strings.HasPrefix(entry.Target, "unresolved:") && review.bindings[entry.sourceOwner()+"|"+entry.Port+"|"+entry.Binding]
}

func compareEffectEntries(base, candidate *commandEffectManifest, adrs map[string]bool, domainCensus bool) ([]string, int, int) {
	review := newEffectBaseReview(base, adrs, domainCensus)
	var failures []string
	added, changed := 0, 0
	for _, entry := range candidate.Entries {
		was, existed := review.previous[entry.key()]
		if !existed {
			added++
		}
		if existed && was.BindingDigest != entry.BindingDigest {
			changed++
		}
		failures = append(failures, entry.effectBaseFailures(was, existed, review.approves(entry, existed))...)
	}
	return failures, added, changed
}

func (entry commandEffectEntry) reviewedPortKey() string {
	return strings.Join([]string{entry.sourceOwner(), entry.Port, entry.Binding, entry.TargetOwner, entry.Effect, entry.Classification, entry.ADR}, "|")
}

func (entry commandEffectEntry) effectBaseFailures(was commandEffectEntry, existed, newADR bool) []string {
	isException := strings.HasPrefix(entry.Classification, "E") || entry.Classification == "unresolved" || entry.Classification == "debt" || entry.Classification == "workflow"
	if !existed {
		if isException && !newADR {
			return []string{"new exception needs approval_adr naming an ADR added in this change: " + entry.key()}
		}
		return nil
	}
	var failures []string
	semanticChange := was.Effect != entry.Effect || was.Classification != entry.Classification || was.ADR != entry.ADR
	if semanticChange && !newADR {
		failures = append(failures, "effect/classification changed without a new approval_adr: "+entry.key())
	}
	if was.BindingDigest != entry.BindingDigest {
		failures = append(failures, entry.changedBindingFailures(was, isException, newADR)...)
	}
	return failures
}

func (entry commandEffectEntry) changedBindingFailures(was commandEffectEntry, isException, newADR bool) []string {
	var failures []string
	if entry.Reason == was.Reason {
		failures = append(failures, "binding changed; update exact review evidence: "+entry.key())
	}
	if isException && !newADR {
		failures = append(failures, "exception binding changed without a new approval_adr: "+entry.key())
	}
	return failures
}

func compareCommandEffectsIfEnabled(project, ref string, policy *Policy, rewrite *baseRewrite) error {
	if !commandEffectsEnabled(policy) {
		return nil
	}
	// LoadGraph has already rejected an absent register if a foreign binding
	// exists. Policies with no binding surface need no empty register/bootstrap.
	if _, err := os.Stat(filepath.Join(project, commandEffectInventory)); os.IsNotExist(err) {
		return nil
	}
	return CompareCommandEffects(project, ref, rewrite)
}

func (manifest *commandEffectManifest) unresolvedCount() int {
	count := 0
	for _, entry := range manifest.Entries {
		if entry.Effect == "unresolved" {
			count++
		}
	}
	return count
}
