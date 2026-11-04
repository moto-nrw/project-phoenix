// Package querybudget reads authored query ceilings independently of test execution.
package querybudget

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

// Budget is a scenario's reviewed SQL statement contract.
type Budget struct {
	Scenario    string `json:"scenario"`
	Max         int    `json:"max"`
	Exact       bool   `json:"exact"`
	Description string `json:"description"`
}

// ReadQueryBudgets discovers all fragments. Empty registers, duplicates and malformed files
// fail closed; filenames are organizational, never an authority for identity.
func ReadQueryBudgets(source fs.FS, directory string) (map[string]Budget, error) {
	if err := validateBudgetDirectory(source, directory); err != nil {
		return nil, err
	}
	budgets := map[string]Budget{}
	err := fs.WalkDir(source, directory, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("query budget symlink %s", name)
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(name, ".json") {
			return nil
		}
		return addFragment(source, budgets, name)
	})
	if err != nil {
		return nil, err
	}
	if len(budgets) == 0 {
		return nil, fmt.Errorf("query budget register %s is empty", directory)
	}
	return budgets, nil
}

func addFragment(source fs.FS, budgets map[string]Budget, name string) error {
	data, err := fs.ReadFile(source, name)
	if err != nil {
		return err
	}
	budget, err := Parse(data)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if _, exists := budgets[budget.Scenario]; exists {
		return fmt.Errorf("duplicate query budget %q", budget.Scenario)
	}
	budgets[budget.Scenario] = budget
	return nil
}

// Parse rejects duplicate JSON members as well as unknown/missing fields.
func Parse(data []byte) (Budget, error) {
	fields, err := parseFields(data)
	if err != nil {
		return Budget{}, err
	}

	if len(fields) != 4 {
		return Budget{}, fmt.Errorf("query budget requires scenario, max, exact, description")
	}
	var budget Budget
	for name, target := range map[string]any{"scenario": &budget.Scenario, "max": &budget.Max, "exact": &budget.Exact, "description": &budget.Description} {
		value, exists := fields[name]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return Budget{}, fmt.Errorf("missing query budget field %q", name)
		}
		if err := json.Unmarshal(value, target); err != nil {
			return Budget{}, fmt.Errorf("query budget field %q: %w", name, err)
		}
	}
	if strings.TrimSpace(budget.Scenario) == "" || strings.TrimSpace(budget.Description) == "" || budget.Max < 0 {
		return Budget{}, fmt.Errorf("query budget requires scenario, description and nonnegative max")
	}
	return budget, nil
}

func parseFields(data []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("query budget must be an object")
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("query budget field must be a string")
		}
		if _, exists := fields[name]; exists {
			return nil, fmt.Errorf("duplicate query budget field %q", name)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		fields[name] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing query budget content")
	}
	return fields, nil
}

// WalkDir's initial Stat follows a root symlink. Reject every directory
// boundary so an accepted candidate remains readable as an immutable Git tree.
func validateBudgetDirectory(source fs.FS, directory string) error {
	if !fs.ValidPath(directory) {
		return fs.ErrInvalid
	}
	if directory == "." {
		return nil
	}
	if err := validateBudgetDirectory(source, path.Dir(directory)); err != nil {
		return err
	}
	entries, err := fs.ReadDir(source, path.Dir(directory))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != path.Base(directory) {
			continue
		}
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("query budget directory %s is not a regular directory", directory)
		}
		return nil
	}
	return &fs.PathError{Op: "readdir", Path: directory, Err: fs.ErrNotExist}
}
