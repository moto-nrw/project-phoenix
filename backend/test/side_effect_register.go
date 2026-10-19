package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func sideEffectRegisterOnlyShrinks(root string, current map[string]losableSideEffectNotice) error {
	repo := filepath.Dir(root)
	run := func(args ...string) ([]byte, error) {
		command := exec.Command("git", args...)
		command.Dir = repo
		return command.Output()
	}
	// Never turn a missing base ref into a successful shrink check.
	revision, err := run("rev-parse", "--verify", "origin/development^{commit}")
	if err != nil {
		return fmt.Errorf("notice ratchet needs fetched origin/development: %w", err)
	}
	base := strings.TrimSpace(string(revision))
	path := "backend/" + sideEffectNoticeRegister
	entries, err := run("ls-tree", "--name-only", base, "--", path)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(entries)) == 0 {
		return nil
	} // Initial #4087 introduction.
	raw, err := run("show", base+":"+path)
	if err != nil {
		return err
	}
	previous := map[string]losableSideEffectNotice{}
	if err := json.Unmarshal(raw, &previous); err != nil {
		return err
	}
	return sideEffectRegisterSubset(current, previous)
}

const sideEffectNoticeRegister = "test/durable_side_effect_notices.json"

type losableSideEffectNotice struct {
	Reason      string `json:"reason"`
	Fingerprint string `json:"fingerprint"`
}

func sideEffectRegisterSubset(current, previous map[string]losableSideEffectNotice) error {
	// Only removed entries can supply a relocation. Consume each once so a
	// copied notice cannot become an additional allowance at another path.
	relocations := map[sideEffectNoticeIdentity]int{}
	for key, notice := range previous {
		if _, retained := current[key]; retained {
			continue
		}
		if identity, ok := sideEffectNoticeIdentityAt(key, notice); ok {
			relocations[identity]++
		}
	}
	keys := make([]string, 0, len(current))
	for key := range current {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var added []string
	for _, key := range keys {
		notice := current[key]
		old, ok := previous[key]
		if ok {
			if old != notice {
				added = append(added, key)
			}
			continue
		}
		identity, validPath := sideEffectNoticeIdentityAt(key, notice)
		if !validPath || relocations[identity] == 0 {
			added = append(added, key)
			continue
		}
		relocations[identity]--
	}
	sort.Strings(added)
	if len(added) > 0 {
		return fmt.Errorf("losable-notice register may only shrink or relocate unchanged entries: %s", strings.Join(added, ", "))
	}
	return nil
}

type sideEffectNoticeIdentity struct {
	operation, reason, fingerprint string
}

func sideEffectNoticeIdentityAt(key string, notice losableSideEffectNotice) (sideEffectNoticeIdentity, bool) {
	file, operation, ok := strings.Cut(key, ":")
	if !ok || filepath.Ext(file) != ".go" || operation == "" {
		return sideEffectNoticeIdentity{}, false
	}
	return sideEffectNoticeIdentity{operation, notice.Reason, notice.Fingerprint}, true
}
