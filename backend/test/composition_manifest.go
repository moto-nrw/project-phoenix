package test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
)

// ReplaceCompositionFields updates only the named top-level fields. It
// serializes keys in a stable order and coordinates independent updater
// processes, so one updater cannot overwrite another updater's fields.
func ReplaceCompositionFields(path string, replacements map[string]any) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0) // #nosec G304 -- caller supplies the repository manifest path
	if err != nil {
		return fmt.Errorf("open composition manifest: %w", err)
	}
	defer func() { _ = file.Close() }()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock composition manifest: %w", err)
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }()
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read composition manifest: %w", err)
	}
	encoded, err := replaceCompositionFields(content, replacements)
	if err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("truncate composition manifest: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind composition manifest: %w", err)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write composition manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync composition manifest: %w", err)
	}
	return nil
}

func replaceCompositionFields(content []byte, replacements map[string]any) ([]byte, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("decode composition manifest: %w", err)
	}
	for key, value := range replacements {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode composition field %q: %w", key, err)
		}
		document[key] = encoded
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode composition manifest: %w", err)
	}
	return encoded, nil
}
