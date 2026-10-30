package artifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WriteInventory partitions an explicit reviewed/generator document. It never
// infers labels or raises a budget. Callers retain their semantic validation.
func WriteInventory(filename string, kind InventoryKind, document []byte) error {
	spec, err := inventoryFormat(kind)
	if err != nil {
		return err
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(document, &header); err != nil {
		return err
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(header[spec.collection], &entries); err != nil {
		return err
	}
	delete(header, spec.collection)
	header["storage"], _ = json.Marshal(inventoryStorage)
	files, err := partitionInventory(entries, spec)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		header["empty"] = json.RawMessage("true")
	} else {
		delete(header, "empty")
	}
	data, err := canonicalJSON(header)
	if err != nil {
		return err
	}
	return replaceInventory(filename, string(kind), data, files)
}

func partitionInventory(entries []map[string]json.RawMessage, spec inventorySpec) (map[string][]byte, error) {
	grouped := map[string][]map[string]json.RawMessage{}
	seen := map[string]bool{}
	for _, entry := range entries {
		if err := validateInventoryFields(entry, spec, ""); err != nil {
			return nil, err
		}
		key, err := inventoryKey(entry, spec.identity)
		if err != nil {
			return nil, err
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicate inventory identity %s", key)
		}
		seen[key] = true
		var owner string
		if err := json.Unmarshal(entry[spec.owner], &owner); err != nil || owner == "" {
			return nil, fmt.Errorf("missing inventory owner for %s", key)
		}
		halves := inventoryHalves(entry, spec)
		for side, half := range halves {
			name := filepath.Join(side, url.PathEscape(owner)+".json")
			grouped[name] = append(grouped[name], half)
		}
	}
	files := map[string][]byte{}
	for name, records := range grouped {
		sort.Slice(records, func(i, j int) bool {
			a, _ := inventoryKey(records[i], spec.identity)
			b, _ := inventoryKey(records[j], spec.identity)
			return a < b
		})
		data, err := canonicalJSON(map[string]any{spec.collection: records})
		if err != nil {
			return nil, err
		}
		files[name] = data
	}
	return files, nil
}

func canonicalJSON(value any) ([]byte, error) {
	// Re-decode to normalize nested RawMessages too. Stored fragment keys have
	// one canonical order independent of callers' Go struct declaration order.
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(document, "", "  ")
	return append(data, '\n'), err
}

func replaceInventory(filename, kind string, header []byte, files map[string][]byte) error {
	parent := filepath.Dir(filename)
	staged, err := os.MkdirTemp(parent, "."+kind+"-next-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staged) }()
	for _, side := range []string{"reviews", "evidence"} {
		if err := os.MkdirAll(filepath.Join(staged, side), 0755); err != nil {
			return err
		}
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(staged, name), data, 0644); err != nil {
			return err
		}
	}
	return publishInventory(filename, kind, staged, header)
}

func publishInventory(filename, kind, staged string, header []byte) error {
	directory := filepath.Join(filepath.Dir(filename), kind)
	if err := assertInventoryDirectory(directory); err != nil {
		return err
	}
	headerFile, err := stageInventoryHeader(filename, header)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(headerFile) }()
	backup, err := os.MkdirTemp(filepath.Dir(filename), "."+kind+"-previous-")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	previous := false
	if err := os.Rename(directory, backup); err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		previous = true
	}
	if err := os.Rename(staged, directory); err != nil {
		if previous {
			return errors.Join(err, os.Rename(backup, directory))
		}
		return err
	}
	if err := os.Rename(headerFile, filename); err != nil {
		cleanup := os.RemoveAll(directory)
		if previous {
			cleanup = errors.Join(cleanup, os.Rename(backup, directory))
		}
		return errors.Join(err, cleanup)
	}
	if previous {
		return os.RemoveAll(backup)
	}
	return nil
}

func stageInventoryHeader(filename string, header []byte) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(filename), ".inventory-header-")
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(header)
	err = errors.Join(writeErr, file.Chmod(0644), file.Close())
	if err != nil {
		return "", errors.Join(err, os.Remove(file.Name()))
	}
	return file.Name(), nil
}

func assertInventoryDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refuse to replace non-inventory path %s", directory)
	}
	sides, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, side := range sides {
		if (side.Name() != "reviews" && side.Name() != "evidence") || !side.IsDir() || side.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse to replace unrelated inventory entry %s", side.Name())
		}
		if err := assertInventoryFiles(filepath.Join(directory, side.Name())); err != nil {
			return err
		}
	}
	return nil
}

func assertInventoryFiles(directory string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() || file.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(file.Name(), ".json") {
			return fmt.Errorf("refuse to replace unrelated inventory file %s", file.Name())
		}
	}
	return nil
}

func inventoryHalves(entry map[string]json.RawMessage, spec inventorySpec) map[string]map[string]json.RawMessage {
	result := map[string]map[string]json.RawMessage{}
	for _, side := range []string{"reviews", "evidence"} {
		fields := append([]string{}, spec.identity...)
		if side == "reviews" {
			fields = append(fields, spec.review...)
		} else {
			fields = append(fields, spec.evidence...)
		}
		half := map[string]json.RawMessage{}
		for _, field := range fields {
			if value, exists := entry[field]; exists {
				half[field] = value
			}
		}
		result[side] = half
	}
	return result
}
