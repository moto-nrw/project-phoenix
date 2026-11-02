package architecture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

type effectBase struct {
	root, sha    string
	addedADRs    map[string]bool
	domainCensus bool
}

// CompareCommandEffects checks reviewed effects against the immutable PR base.
// New leaves need exact evidence; expanded exceptions need a newly added ADR.
func CompareCommandEffects(project, ref string) error {
	candidate, err := loadCommandEffects(project)
	if err != nil {
		return err
	}
	context, err := resolveEffectBase(project, ref)
	if err != nil {
		return err
	}
	base, bootstrap, err := loadBaseCommandEffects(project, context)
	if err != nil {
		return err
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
	return effectBase{root: root, sha: sha, addedADRs: adrs}, err
}

const commandEffectDecision = "docs/adr/0083-ratchets-protect-invariants-and-expose-command-effects.md"
const commandEffectAddendum = "\n## Implementation addendum 2026-10-23 (#4739)\n"

func loadBaseCommandEffects(project string, base effectBase) (*commandEffectManifest, bool, error) {
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
	manifest, err := decodeCommandEffects(stored.Document)
	return manifest, false, err
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

func compareCommandEffectsIfEnabled(project, ref string, policy *Policy) error {
	if !commandEffectsEnabled(policy) {
		return nil
	}
	// LoadGraph has already rejected an absent register if a foreign binding
	// exists. Policies with no binding surface need no empty register/bootstrap.
	if _, err := os.Stat(filepath.Join(project, commandEffectInventory)); os.IsNotExist(err) {
		return nil
	}
	return CompareCommandEffects(project, ref)
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
