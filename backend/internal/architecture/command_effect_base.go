package architecture

import (
	"fmt"
	"path/filepath"
	"strings"
)

type effectBase struct {
	root, sha string
	addedADRs map[string]bool
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
	base, err := loadBaseCommandEffects(project, context)
	if err != nil {
		return err
	}
	failures, added, changed := compareEffectEntries(base, candidate, context.addedADRs)
	if len(failures) > 0 {
		return fmt.Errorf("command effects against base %s:\n  %s", context.sha, strings.Join(failures, "\n  "))
	}
	fmt.Printf("command effects: %d entries; %d added, %d changed against base %s\n", len(candidate.Entries), added, changed, context.sha)
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
	return effectBase{root, sha, adrs}, err
}

func loadBaseCommandEffects(project string, base effectBase) (*commandEffectManifest, error) {
	relative, err := repositoryRelativePath(base.root, project, commandEffectInventory)
	if err != nil {
		return nil, err
	}
	listing, err := gitOutput(base.root, "ls-tree", "--name-only", base.sha, "--", filepath.ToSlash(relative))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(listing) == "" {
		if !base.addedADRs["0089"] {
			return nil, fmt.Errorf("command effect bootstrap requires new ADR 0089")
		}
		return &commandEffectManifest{SchemaVersion: 1}, nil
	}
	blob, err := readGitBlob(base.root, project, base.sha, commandEffectInventory)
	if err != nil {
		return nil, err
	}
	return decodeCommandEffects(blob)
}

func compareEffectEntries(base, candidate *commandEffectManifest, adrs map[string]bool) ([]string, int, int) {
	previous := map[string]commandEffectEntry{}
	for _, entry := range base.Entries {
		previous[entry.key()] = entry
	}
	var failures []string
	added, changed := 0, 0
	for _, entry := range candidate.Entries {
		was, existed := previous[entry.key()]
		if !existed {
			added++
		}
		if existed && was.BindingDigest != entry.BindingDigest {
			changed++
		}
		failures = append(failures, entry.effectBaseFailures(was, existed, adrs[entry.ApprovalADR])...)
	}
	return failures, added, changed
}

func (entry commandEffectEntry) effectBaseFailures(was commandEffectEntry, existed, newADR bool) []string {
	isException := strings.HasPrefix(entry.Classification, "E") || entry.Classification == "unresolved"
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
	return CompareCommandEffects(project, ref)
}
