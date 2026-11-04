package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const transactionBoundaryRegister = "test/transaction_boundaries.json"

type transactionBoundaryRegisterData struct {
	Admin    map[string]string `json:"admin"`
	Commands map[string]string `json:"commands"`
}

func transactionRegisterOnlyShrinks(root string, current transactionBoundaryRegisterData) error {
	run := func(args ...string) ([]byte, error) {
		return registerGitOutput(filepath.Dir(root), args...)
	}
	base, err := registerMergeBase(run)
	if err != nil {
		return err
	}
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
	if transactionRegisterUnchangedSubset(current, previous) {
		return nil
	}
	moves, err := verifiedTransactionRelocations(root, base)
	if err != nil {
		return err
	}
	fingerprints := transactionFingerprints(root, base, run)
	receiverRemoved := removedTransactionReceiverMethods(root)
	for _, pair := range []struct{ current, previous map[string]string }{{current.Admin, previous.Admin}, {current.Commands, previous.Commands}} {
		if err := transactionScopeSubset(pair.current, pair.previous, moves, fingerprints, receiverRemoved); err != nil {
			return err
		}
	}
	return nil
}

func transactionRegisterUnchangedSubset(current, previous transactionBoundaryRegisterData) bool {
	for _, pair := range []struct{ current, previous map[string]string }{{current.Admin, previous.Admin}, {current.Commands, previous.Commands}} {
		for key, reason := range pair.current {
			if old, ok := pair.previous[key]; !ok || old != reason {
				return false
			}
		}
	}
	return true
}

func verifiedTransactionRelocations(root, base string) (map[string]string, error) {
	command := exec.Command(filepath.Join(filepath.Dir(root), "scripts/backend-architecture.sh"), "relocations", "--base-ref", base)
	command.Dir = root
	command.Env = os.Environ()
	raw, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("verify transaction package relocations: %w", err)
	}
	var moves map[string]string
	if err := json.Unmarshal(raw, &moves); err != nil {
		return nil, fmt.Errorf("decode verified transaction package relocations: %w", err)
	}
	return moves, nil
}
