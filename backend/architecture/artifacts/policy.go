// Package artifacts reads authored, owner-local architecture policy sources.
// It owns storage composition only, never architectural permission decisions.
package artifacts

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
)

const policyStorage = "owner-fragments-v1"

var ownerFile = regexp.MustCompile(`^[a-z][a-z0-9-]*\.json$`)
var policySections = map[string]string{
	"packages": "path", "data_objects": "name", "read_projections": "id", "rules": "id",
}

//go:embed policy.go policy_parts.go inventory.go git.go
var sources embed.FS

// SourceFingerprint lets evidence scanners include storage semantics in their
// reviewed scan scope, rather than leaving a transitive reader unhashed.
func SourceFingerprint() string {
	sum := sha256.New()
	for _, name := range []string{"policy.go", "policy_parts.go", "inventory.go", "git.go"} {
		data, _ := sources.ReadFile(name)
		_, _ = sum.Write([]byte(name))
		_, _ = sum.Write(data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// ReadPolicy accepts historical materialized documents and current fragments.
// A declared fragmented document never falls back when a part is missing.
func ReadPolicy(filename string) ([]byte, error) {
	return ReadPolicyFS(os.DirFS(filepath.Dir(filename)), filepath.Base(filename))
}

// ReadPolicyFS composes exactly the supplied filesystem. Immutable-base callers
// supply a Git tree, never candidate paths or candidate fragment lists.
func ReadPolicyFS(source fs.FS, filename string) ([]byte, error) {
	data, err := fs.ReadFile(source, filename)
	if err != nil {
		if orphan := rejectOrphanParts(source, filename, "policy"); orphan != nil {
			return nil, orphan
		}
		return nil, err
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, err
	}
	marker, fragmented := header["storage"]
	if !fragmented {
		if err := rejectOrphanParts(source, filename, "policy"); err != nil {
			return nil, err
		}
		return data, nil
	}
	var format string
	if err := json.Unmarshal(marker, &format); err != nil || format != policyStorage {
		return nil, fmt.Errorf("unsupported policy storage: %s", marker)
	}
	delete(header, "storage")
	for section := range policySections {
		if _, exists := header[section]; exists {
			return nil, fmt.Errorf("mixed policy storage: %s", section)
		}
	}
	directory := path.Join(path.Dir(filename), "policy")
	parts, err := readPolicyParts(source, directory)
	if err != nil {
		return nil, err
	}
	return assemblePolicy(header, parts)
}

// A missing or historical header cannot hide remaining authored fragments.
func rejectOrphanParts(source fs.FS, filename, directory string) error {
	entries, err := fs.ReadDir(source, path.Dir(filename))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect artifact siblings: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == directory {
			return fmt.Errorf("orphan %s fragments require a fragmented %s header", directory, filename)
		}
	}
	return nil
}

// Validate each authored directory boundary before listing. os.DirFS follows a
// directory symlink, whereas an immutable Git tree cannot traverse it.
func readArtifactDirectory(source fs.FS, directory string) ([]fs.DirEntry, error) {
	if directory == "." {
		return fs.ReadDir(source, directory)
	}
	entries, err := readArtifactDirectory(source, path.Dir(directory))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Name() != path.Base(directory) {
			continue
		}
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("artifact directory %s is not a regular directory", directory)
		}
		return fs.ReadDir(source, directory)
	}
	return nil, &fs.PathError{Op: "readdir", Path: directory, Err: fs.ErrNotExist}
}
