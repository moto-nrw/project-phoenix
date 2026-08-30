package simulate

import (
	"encoding/json"
	"fmt"
	"os"
)

func WriteSeedState(state *SeedState, path string) error {
	state.normalize()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal seed state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write seed state: %w", err)
	}
	return nil
}
