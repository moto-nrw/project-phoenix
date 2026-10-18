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
	var added []string
	for key, notice := range current {
		old, ok := previous[key]
		if !ok || old.Fingerprint != notice.Fingerprint {
			added = append(added, key)
		}
	}
	sort.Strings(added)
	if len(added) > 0 {
		return fmt.Errorf("losable-notice register may only shrink, not add or replace entries: %s", strings.Join(added, ", "))
	}
	return nil
}
