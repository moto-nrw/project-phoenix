package routetest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RegisteredErrorCode is an authored wire identity, not a generated expectation.
type RegisteredErrorCode struct {
	Code       string   `json:"code"`
	Class      string   `json:"class"`
	Parameters []string `json:"parameters"`
	Legacy     string   `json:"legacy"`
}

// ReadErrorRegistry discovers all domain fragments and refuses missing or
// conflicting input. Tests still check production envelopes independently of
// the generated constants and class maps they exercise.
func ReadErrorRegistry(directory string) ([]RegisteredErrorCode, error) {
	files, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s: no error registry fragments", directory)
	}
	var result []RegisteredErrorCode
	seen := make(map[string]bool)
	legacy := make(map[string]bool)
	for _, file := range files {
		entries, err := readErrorDomain(file)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if seen[entry.Code] || legacy[entry.Legacy] {
				return nil, fmt.Errorf("%s: duplicate code or legacy identity %q", file, entry.Code)
			}
			seen[entry.Code], legacy[entry.Legacy] = true, true
			result = append(result, entry)
		}
	}
	return result, nil
}

func readErrorDomain(file string) ([]RegisteredErrorCode, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var fragment struct {
		Codes []RegisteredErrorCode `json:"codes"`
	}
	if err := json.Unmarshal(data, &fragment); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if len(fragment.Codes) == 0 {
		return nil, fmt.Errorf("%s: empty error registry fragment", file)
	}
	domain := strings.TrimSuffix(filepath.Base(file), ".json")
	for _, entry := range fragment.Codes {
		area, _, ok := strings.Cut(entry.Code, ".")
		if !ok || area != domain || entry.Legacy == "" {
			return nil, fmt.Errorf("%s: invalid or misplaced code %q", file, entry.Code)
		}
	}
	return fragment.Codes, nil
}
