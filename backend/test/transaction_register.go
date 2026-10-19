package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const transactionBoundaryRegister = "test/transaction_boundaries.json"

type transactionBoundaryRegisterData struct {
	Admin    map[string]string `json:"admin"`
	Commands map[string]string `json:"commands"`
}

func transactionRegisterOnlyShrinks(root string, current transactionBoundaryRegisterData) error {
	run := func(args ...string) ([]byte, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = filepath.Dir(root)
		return cmd.Output()
	}
	revision, err := run("rev-parse", "--verify", "origin/development^{commit}")
	if err != nil {
		return err
	}
	base := strings.TrimSpace(string(revision))
	path := "backend/" + transactionBoundaryRegister
	files, err := run("ls-tree", "--name-only", base, "--", path)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(files)) == 0 {
		return nil
	} // Initial #4088 register.
	raw, err := run("show", base+":"+path)
	if err != nil {
		return err
	}
	var previous transactionBoundaryRegisterData
	if err = json.Unmarshal(raw, &previous); err != nil {
		return err
	}
	for _, pair := range []struct{ current, previous map[string]string }{{current.Admin, previous.Admin}, {current.Commands, previous.Commands}} {
		for key, reason := range pair.current {
			if old, ok := pair.previous[key]; !ok || old != reason {
				return fmt.Errorf("transaction register may only shrink; added or changed: %s", key)
			}
		}
	}
	return nil
}
