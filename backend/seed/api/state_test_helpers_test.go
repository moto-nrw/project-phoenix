package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func LoadSeedState(path string) (*SeedState, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read seed state: %w", err)
	}
	var state SeedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal seed state: %w", err)
	}
	if state.Version != "" && state.Version != CurrentSeedStateVersion {
		return nil, fmt.Errorf("unsupported seed state version: %s", state.Version)
	}
	state.Normalize()
	return &state, nil
}
