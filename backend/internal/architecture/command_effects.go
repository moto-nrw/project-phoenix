package architecture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const commandEffectInventory = "architecture/command-effects.json"

type commandEffectEntry struct {
	commandEffectSite
	Effect         string `json:"effect"`
	Classification string `json:"classification"`
	ADR            string `json:"adr,omitempty"`
	ApprovalADR    string `json:"approval_adr,omitempty"`
	Reason         string `json:"reason"`
}

type commandEffectManifest struct {
	SchemaVersion int                  `json:"schema_version"`
	Entries       []commandEffectEntry `json:"entries"`
}

func loadCommandEffects(project string) (*commandEffectManifest, error) {
	blob, err := os.ReadFile(filepath.Join(project, commandEffectInventory))
	if err != nil {
		return nil, fmt.Errorf("load command effects: %w", err)
	}
	return decodeCommandEffects(blob)
}

func decodeCommandEffects(blob []byte) (*commandEffectManifest, error) {
	var manifest commandEffectManifest
	decoder := json.NewDecoder(bytes.NewReader(blob))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode command effects: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("command effects must contain exactly one JSON object")
	}
	if manifest.SchemaVersion != 1 {
		return nil, fmt.Errorf("command effects schema_version must be 1")
	}
	seen := map[string]bool{}
	for _, entry := range manifest.Entries {
		if entry.Port == "" || entry.Binding == "" || entry.Target == "" {
			return nil, fmt.Errorf("command effect identity is incomplete")
		}
		if seen[entry.key()] {
			return nil, fmt.Errorf("duplicate command effect %s", entry.key())
		}
		seen[entry.key()] = true
	}
	return &manifest, nil
}

func analyzeCommandEffects(project string, policy *Policy, loaded []*packages.Package) ([]Violation, error) {
	if !commandEffectsEnabled(policy) {
		return nil, nil
	}
	discovered := discoverCommandEffects(policy, loaded)
	manifest, err := loadCanonicalCommandEffects(project)
	if errors.Is(err, os.ErrNotExist) && len(discovered) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return manifest.siteViolations(discovered, policy), nil
}

func loadCanonicalCommandEffects(project string) (*commandEffectManifest, error) {
	blob, err := os.ReadFile(filepath.Join(project, commandEffectInventory))
	if err != nil {
		return nil, fmt.Errorf("load command effects: %w", err)
	}
	manifest, err := decodeCommandEffects(blob)
	if err != nil {
		return nil, err
	}
	canonical, err := marshalCommandEffects(manifest)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(blob, canonical) {
		return nil, fmt.Errorf("command effects must be canonical JSON; run scripts/backend-architecture.sh effects and review the result")
	}
	return manifest, nil
}

func (manifest *commandEffectManifest) siteViolations(discovered []commandEffectSite, policy *Policy) []Violation {
	classified := policy.packageMap()
	ownerKinds := map[string]string{}
	for _, owner := range policy.Owners {
		ownerKinds[owner.ID] = owner.Kind
	}
	entries := map[string]commandEffectEntry{}
	for _, entry := range manifest.Entries {
		entries[entry.key()] = entry
	}
	violations := manifest.targetEffectViolations()
	for _, site := range discovered {
		entry, present := entries[site.key()]
		if !present {
			rule := "uninventoried"
			if strings.HasPrefix(site.Target, "unresolved:") {
				rule = "unresolved"
			}
			violations = append(violations, commandEffectViolation(rule, site, "binding effect has no exact inventory entry; run scripts/backend-architecture.sh effects and review the result"))
			continue
		}
		if entry.BindingDigest != site.BindingDigest {
			violations = append(violations, commandEffectViolation("changed", site, "binding body or target signature changed; regenerate and review evidence"))
		}
		if err := validateEffectEntry(entry); err != nil {
			violations = append(violations, commandEffectViolation("classification", site, err.Error()))
		}
		if err := validateEffectDirection(entry, ownerKinds, classified); err != nil {
			violations = append(violations, commandEffectViolation("classification", site, err.Error()))
		}
		delete(entries, site.key())
	}
	for _, stale := range entries {
		violations = append(violations, commandEffectViolation("stale", stale.commandEffectSite, "inventory entry has no typed binding; remove it"))
	}
	return violations
}

func (manifest *commandEffectManifest) targetEffectViolations() []Violation {
	var violations []Violation
	targetEffects := map[string]string{}
	for _, entry := range manifest.Entries {
		if strings.HasPrefix(entry.Target, "unresolved:") {
			continue
		}
		target := strings.TrimPrefix(entry.Target, "*")
		if effect, found := targetEffects[target]; found && effect != entry.Effect {
			violations = append(violations, commandEffectViolation("inconsistent", entry.commandEffectSite, "target method has conflicting effects: "+effect+" and "+entry.Effect))
		} else {
			targetEffects[target] = entry.Effect
		}
	}
	return violations
}

// GenerateCommandEffects preserves exact reviewed entries but never invents effect labels.
func GenerateCommandEffects(project string, policy *Policy) error {
	loaded, err := loadTypedPackages(project, policy.Build)
	if err != nil {
		return err
	}
	existing, err := loadCommandEffects(project)
	if err != nil {
		return err
	}
	byKey := map[string]commandEffectEntry{}
	for _, entry := range existing.Entries {
		byKey[entry.key()] = entry
	}
	manifest := commandEffectManifest{SchemaVersion: 1, Entries: []commandEffectEntry{}}
	for _, site := range discoverCommandEffects(policy, loaded) {
		entry, ok := byKey[site.key()]
		if !ok {
			entry = commandEffectEntry{commandEffectSite: site, Effect: "unknown"}
		}
		entry.commandEffectSite = site
		manifest.Entries = append(manifest.Entries, entry)
	}
	blob, err := marshalCommandEffects(&manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(project, commandEffectInventory), blob, 0644)
}

func marshalCommandEffects(manifest *commandEffectManifest) ([]byte, error) {
	entries := append([]commandEffectEntry{}, manifest.Entries...)
	for i := range entries {
		entries[i].SourceOwner = entries[i].sourceOwner()
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key() < entries[j].key() })
	blob, err := json.MarshalIndent(commandEffectManifest{SchemaVersion: manifest.SchemaVersion, Entries: entries}, "", "  ")
	return append(blob, '\n'), err
}

func finishSemanticEffects(project string, policy *Policy, loaded []*packages.Package, violations []Violation) ([]Violation, error) {
	effects, err := analyzeCommandEffects(project, policy, loaded)
	if err != nil {
		return nil, err
	}
	return uniqueSortedViolations(append(violations, effects...)), nil
}
