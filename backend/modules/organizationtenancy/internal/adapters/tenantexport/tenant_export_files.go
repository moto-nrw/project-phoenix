package tenantexport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

func (r *exportRun) allowedObject(key string, accountReference bool) bool {
	if !fs.ValidPath(key) || strings.ContainsAny(key, `\`) {
		return false
	}
	school := strconv.FormatInt(r.options.SchoolID, 10)
	for _, kind := range r.catalog.Files {
		if kind.Export != "include" {
			continue
		}
		if matchesExportObject(kind, key, school, accountReference) {
			return true
		}
	}
	return false
}

func matchesExportObject(kind organizationtenancy.LifecycleFileKind, key, school string, accountReference bool) bool {
	switch kind.Layout {
	case "tenantDir":
		return strings.HasPrefix(key, kind.Kind+"/"+school+"/")
	case "tenantPrefix":
		return strings.HasPrefix(key, kind.Kind+"/"+school+"_") && strings.Count(key, "/") == 1
	case "account":
		return accountReference && strings.HasPrefix(key, kind.Kind+"/")
	default:
		return false
	}
}

func (r *exportRun) exportFiles(ctx context.Context) error {
	root, err := os.OpenRoot(r.options.Uploads)
	if err != nil {
		return errors.New("tenant export: uploads root unavailable")
	}
	defer func() { _ = root.Close() }()
	if err := os.Mkdir(filepath.Join(r.options.Output, "objects"), 0o700); err != nil {
		return errors.New("tenant export: cannot create object directory")
	}
	mapping, err := os.OpenFile(filepath.Join(r.options.Output, "objects.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("tenant export: cannot create object mapping")
	}
	defer func() { _ = mapping.Close() }()
	for _, kind := range r.catalog.Files {
		if kind.Export != "include" {
			return errors.New("tenant export: unknown file classification")
		}
		if err := r.exportFileKind(ctx, root, kind, mapping); err != nil {
			return err
		}
	}
	for _, missing := range r.references {
		if missing {
			return errors.New("tenant export: referenced original is missing")
		}
	}
	if mapping.Sync() != nil || mapping.Close() != nil {
		return errors.New("tenant export: cannot persist object mapping")
	}
	return r.recordArtifact("objects.jsonl")
}

func (r *exportRun) exportFileKind(ctx context.Context, root *os.Root, kind organizationtenancy.LifecycleFileKind, mapping io.Writer) error {
	keys, err := r.objectKeys(root, kind)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if r.person != nil && !r.references[key] {
			r.person.unassignedObjects++
			continue
		}
		if err := r.copyObject(ctx, root, key, mapping); err != nil {
			return err
		}
		r.references[key] = false
	}
	return nil
}

func (r *exportRun) objectKeys(root *os.Root, kind organizationtenancy.LifecycleFileKind) ([]string, error) {
	keys := []string{}
	if kind.Layout == "account" {
		for key := range r.references {
			if strings.HasPrefix(key, kind.Kind+"/") {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		return keys, nil
	}
	base := kind.Kind
	if kind.Layout == "tenantDir" {
		base += "/" + strconv.FormatInt(r.options.SchoolID, 10)
	}
	if kind.Layout != "tenantDir" && kind.Layout != "tenantPrefix" {
		return nil, errors.New("tenant export: unknown object layout")
	}
	err := fs.WalkDir(root.FS(), base, func(key string, entry fs.DirEntry, walkErr error) error {
		return r.visitExportObject(base, key, entry, walkErr, &keys)
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *exportRun) visitExportObject(base, key string, entry fs.DirEntry, walkErr error, keys *[]string) error {
	if errors.Is(walkErr, fs.ErrNotExist) && key == base {
		return fs.SkipDir
	}
	if walkErr != nil {
		return errors.New("tenant export: cannot enumerate originals")
	}
	if key == base {
		if !entry.IsDir() {
			return errors.New("tenant export: invalid object namespace")
		}
		return nil
	}
	if !r.allowedObject(key, false) {
		if entry.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	if entry.Type()&os.ModeSymlink != 0 {
		return errors.New("tenant export: symbolic link in originals")
	}
	if !entry.IsDir() {
		*keys = append(*keys, key)
	}
	return nil
}

// Open each directory component separately and reject links both before and
// after opening it. A file-level O_NOFOLLOW alone would still follow a link
// from A's directory into B's namespace.
func openExportOriginal(root *os.Root, key string) (*os.File, error) {
	parts := strings.Split(key, "/")
	current := root
	for _, part := range parts[:len(parts)-1] {
		info, err := current.Lstat(part)
		if err != nil || !info.IsDir() {
			return nil, errors.New("invalid directory")
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			return nil, errors.New("unavailable directory")
		}
		defer func() { _ = next.Close() }()
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			return nil, errors.New("changed directory")
		}
		current = next
	}
	return current.OpenFile(parts[len(parts)-1], os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

func (r *exportRun) copyObject(ctx context.Context, root *os.Root, key string, mapping io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	src, err := openExportOriginal(root, key)
	if err != nil {
		return errors.New("tenant export: original unavailable or linked")
	}
	defer func() { _ = src.Close() }()
	before, err := src.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return errors.New("tenant export: original is not a regular file")
	}
	name := path.Join("objects", strconv.Itoa(len(r.manifest.Artifacts)))
	dst, err := os.OpenFile(filepath.Join(r.options.Output, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("tenant export: cannot create original copy")
	}
	defer func() { _ = dst.Close() }()
	digest, size, err := r.fingerprints.Reader(io.TeeReader(exportContextReader{ctx: ctx, reader: src}, dst))
	if err != nil {
		return errors.New("tenant export: cannot copy original")
	}
	after, err := src.Stat()
	if err != nil || size != before.Size() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return errors.New("tenant export: original changed during copy")
	}
	if dst.Sync() != nil || dst.Close() != nil {
		return errors.New("tenant export: cannot persist original")
	}
	r.manifest.Artifacts = append(r.manifest.Artifacts, organizationtenancy.ExportArtifact{Path: name, SHA256: digest, Bytes: size})
	if json.NewEncoder(mapping).Encode(map[string]string{"key": key, "path": name}) != nil {
		return errors.New("tenant export: cannot write object mapping")
	}
	return nil
}

type exportContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r exportContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (r *exportRun) recordArtifact(name string) error {
	f, err := os.Open(filepath.Join(r.options.Output, name))
	if err != nil {
		return errors.New("tenant export: cannot verify artifact")
	}
	defer func() { _ = f.Close() }()
	digest, size, err := r.fingerprints.Reader(f)
	if err != nil {
		return errors.New("tenant export: cannot checksum artifact")
	}
	r.manifest.Artifacts = append(r.manifest.Artifacts, organizationtenancy.ExportArtifact{Path: name, SHA256: digest, Bytes: size})
	return nil
}
