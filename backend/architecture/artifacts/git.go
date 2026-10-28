package artifacts

import (
	"fmt"
	"io/fs"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

// gitTree pins every read and directory listing to one immutable repository
// revision. It never consults candidate paths to discover base fragments.
func gitTree(root, sha, directory string) (fs.FS, error) {
	if !regexp.MustCompile(`^[0-9a-fA-F]{40}$`).MatchString(sha) || !fs.ValidPath(directory) {
		return nil, fmt.Errorf("invalid immutable artifact tree")
	}
	return gitTreeFS{root: root, sha: sha, prefix: directory}, nil
}
func treeGitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("read immutable artifact tree: %w", err)
	}
	return string(output), nil
}

// The resolved base tree, including its directory listing, is the only source.
// Candidate files and symlinks cannot choose which base records get compared.
type gitTreeFS struct{ root, sha, prefix string }

func (source gitTreeFS) Open(name string) (fs.File, error) {
	return nil, fmt.Errorf("base policy Open %q: %w", name, fs.ErrInvalid)
}

func (source gitTreeFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	data, err := treeGitOutput(source.root, "cat-file", "blob", source.sha+":"+path.Join(source.prefix, name))
	return []byte(data), err
}

func (source gitTreeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	tree := source.sha
	if directory := path.Join(source.prefix, name); directory != "." {
		tree += ":" + directory
	}
	output, err := treeGitOutput(source.root, "ls-tree", "-z", tree)
	if err != nil {
		// ls-tree <sha>:<directory> uses exit 128 for absence as well as real Git
		// errors. An independent path lookup distinguishes those cases without
		// treating a corrupt/unresolved revision as an empty base.
		listing, lookupErr := treeGitOutput(source.root, "ls-tree", "--full-tree", "-z", source.sha, "--", path.Join(source.prefix, name))
		if lookupErr == nil && listing == "" {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
		}
		return nil, err
	}
	var entries []fs.DirEntry
	for _, line := range strings.Split(output, "\x00") {
		if line == "" {
			continue
		}
		metadata, filename, ok := strings.Cut(line, "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("invalid Git tree entry %q", line)
		}
		mode := fs.FileMode(0)
		switch fields[0] {
		case "040000":
			mode = fs.ModeDir
		case "120000":
			mode = fs.ModeSymlink
		case "100644", "100755":
		default:
			return nil, fmt.Errorf("unsupported Git policy entry mode %s", fields[0])
		}
		entries = append(entries, gitTreeEntry{name: filename, mode: mode})
	}
	return entries, nil
}

type gitTreeEntry struct {
	name string
	mode fs.FileMode
}

func (entry gitTreeEntry) Name() string               { return entry.name }
func (entry gitTreeEntry) IsDir() bool                { return entry.mode&fs.ModeDir != 0 }
func (entry gitTreeEntry) Type() fs.FileMode          { return entry.mode }
func (entry gitTreeEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrInvalid }

// ReadPolicyAtRevision reads a complete policy from the pinned base tree.
func ReadPolicyAtRevision(root, sha, directory, filename string) ([]byte, error) {
	source, err := gitTree(root, sha, directory)
	if err != nil {
		return nil, err
	}
	return ReadPolicyFS(source, filename)
}

// ReadInventoryAtRevision reads reviewed expectations from an immutable base.
func ReadInventoryAtRevision(root, sha, directory, filename string, kind InventoryKind, canonical bool) (Inventory, error) {
	source, err := gitTree(root, sha, directory)
	if err != nil {
		return Inventory{}, err
	}
	return ReadInventoryFS(source, filename, kind, canonical)
}
