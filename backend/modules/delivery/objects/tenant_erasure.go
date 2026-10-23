package objects

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
)

// TenantObjects counts or removes a complete tenant namespace, including
// unreferenced originals. Callers must stop all upload writers for the run.
// OpenRoot confines traversal; symlinks and special files fail closed. No
// filename or underlying filesystem error is exposed to evidence or logs.
func (l *Local) TenantObjects(ctx context.Context, kind, layout string, tenantID int64, remove bool) (int64, error) {
	if tenantID <= 0 || kind == "" || strings.ContainsAny(kind, "/\\.") || (layout != "tenantDir" && layout != "tenantPrefix") {
		return 0, ErrInvalidKey
	}
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		return 0, errors.New("storage: uploads root unavailable")
	}
	defer func() { _ = root.Close() }()
	if err := plainDirectory(root, kind); err != nil {
		return 0, err
	}
	dir, err := root.OpenRoot(kind)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, errors.New("storage: namespace unavailable")
	}
	defer func() { _ = dir.Close() }()
	id := strconv.FormatInt(tenantID, 10)
	if layout == "tenantDir" {
		if err := plainDirectory(dir, id); err != nil {
			return 0, err
		}
		tenant, err := dir.OpenRoot(id)
		if os.IsNotExist(err) {
			return 0, nil
		}
		if err != nil {
			return 0, errors.New("storage: tenant namespace unavailable")
		}
		defer func() { _ = tenant.Close() }()
		return visitTenantObjects(ctx, tenant, "", remove)
	}
	return visitTenantObjects(ctx, dir, id+"_", remove)
}

func plainDirectory(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("storage: unsafe namespace")
	}
	return nil
}

func visitTenantObjects(ctx context.Context, root *os.Root, prefix string, remove bool) (int64, error) {
	dir, err := root.Open(".")
	if err != nil {
		return 0, errors.New("storage: inventory unavailable")
	}
	defer func() { _ = dir.Close() }()

	var count int64
	for {
		entries, readErr := dir.ReadDir(128)
		n, err := visitTenantBatch(ctx, root, entries, prefix, remove)
		count += n
		if err != nil {
			return count, err
		}
		if errors.Is(readErr, io.EOF) {
			return count, nil
		}
		if readErr != nil {
			return count, errors.New("storage: inventory failed")
		}
	}
}

func visitTenantBatch(ctx context.Context, root *os.Root, entries []os.DirEntry, prefix string, remove bool) (int64, error) {
	var count int64
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		n, err := visitTenantEntry(ctx, root, entry.Name(), prefix, remove)
		count += n
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func visitTenantEntry(ctx context.Context, root *os.Root, name, prefix string, remove bool) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return 0, errors.New("storage: inventory failed")
	}
	var count int64
	if info.IsDir() && prefix == "" {
		child, err := root.OpenRoot(name)
		if err != nil {
			return 0, errors.New("storage: directory unavailable")
		}
		count, err = visitTenantObjects(ctx, child, "", remove)
		_ = child.Close()
		if err != nil {
			return count, err
		}
	} else if info.Mode().IsRegular() {
		count = 1
	} else {
		return 0, errors.New("storage: unsafe object")
	}
	if remove {
		if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
			return count, errors.New("storage: removal failed")
		}
	}
	return count, nil
}
