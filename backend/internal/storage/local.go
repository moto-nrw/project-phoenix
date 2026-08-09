package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	publicDirMode  os.FileMode = 0o755
	publicFileMode os.FileMode = 0o644
	// Private objects stay owner-only in both directions: a 0700 directory
	// keeps the names out of reach even when a file mode is later widened.
	privateDirMode  os.FileMode = 0o700
	privateFileMode os.FileMode = 0o600
)

// Local stores objects on the local filesystem below Root.
//
// Every path is rebuilt from Root plus the validated key and then verified to
// still live inside Root, so a key that slips past Key() validation still
// cannot escape.
type Local struct {
	Root string
}

// NewLocal returns a filesystem backend rooted at root.
func NewLocal(root string) *Local {
	return &Local{Root: root}
}

// localFile adapts *os.File to Object by caching the modification time
// captured at open time.
type localFile struct {
	*os.File
	modTime time.Time
}

func (f *localFile) ModTime() time.Time { return f.modTime }

func (l *Local) resolve(key string) (string, error) {
	if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return "", ErrInvalidKey
	}
	absRoot, err := filepath.Abs(l.Root)
	if err != nil {
		return "", ErrInvalidKey
	}
	full := filepath.Join(absRoot, filepath.FromSlash(key))
	if full != absRoot && !strings.HasPrefix(full, absRoot+string(os.PathSeparator)) {
		return "", ErrInvalidKey
	}
	return full, nil
}

// contextReader aborts a copy between chunks once ctx is done.
//
// It bounds the io.Copy LOOP, not a single blocked syscall: a read that hangs
// inside the kernel still hangs. That is enough for what this guards — a
// caller whose deadline is the only thing keeping its cleanup bookkeeping
// consistent must not keep writing for minutes after that deadline passed.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *contextReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// Save writes r to the resolved path. The object is written directly rather
// than through a temp file and rename: a failed write is removed before the
// error returns, and every consumer treats a missing object as "upload did
// not happen" anyway.
//
// The copy honours ctx. An upload whose deadline exists to keep it inside its
// cleanup window would otherwise be free to finish minutes late, re-creating
// bytes whose cleanup intent the scheduler had already settled — leaving an
// object no sweep can find again.
func (l *Local) Save(ctx context.Context, key string, r io.Reader, opts SaveOptions) (int64, error) {
	full, err := l.resolve(key)
	if err != nil {
		return 0, err
	}

	dirMode, fileMode := publicDirMode, publicFileMode
	if opts.Private {
		dirMode, fileMode = privateDirMode, privateFileMode
	}

	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return 0, errors.New("storage: failed to create directory")
	}
	if opts.Private {
		// MkdirAll leaves an existing directory's mode alone, so a directory
		// created before this key became private is tightened here.
		if err := os.Chmod(dir, privateDirMode); err != nil {
			return 0, errors.New("storage: failed to secure directory")
		}
	}

	dst, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileMode)
	if err != nil {
		return 0, errors.New("storage: failed to create object")
	}
	// Chmod after create: the open mode is masked by umask, which would leave
	// a private object group-readable on a permissive host.
	if err := dst.Chmod(fileMode); err != nil {
		_ = dst.Close()
		_ = os.Remove(full)
		return 0, errors.New("storage: failed to secure object")
	}

	written, copyErr := io.Copy(dst, &contextReader{ctx: ctx, r: r})
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(full)
		if copyErr != nil && ctx.Err() != nil {
			// Surface the cause: the caller decides differently for a
			// deadline than for a disk error.
			return 0, ctx.Err()
		}
		return 0, errors.New("storage: failed to write object")
	}
	return written, nil
}

// Open returns the stored object, or ErrNotFound.
func (l *Local) Open(_ context.Context, key string) (Object, error) {
	full, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, errors.New("storage: failed to open object")
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, errors.New("storage: failed to stat object")
	}
	if info.IsDir() {
		_ = file.Close()
		return nil, ErrNotFound
	}
	return &localFile{File: file, modTime: info.ModTime()}, nil
}

// Remove deletes the object. A missing object is success so cleanup retries
// converge instead of looping forever on an already-removed file.
func (l *Local) Remove(_ context.Context, key string) error {
	full, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return errors.New("storage: failed to remove object")
	}
	return nil
}

// Stat returns the stored size in bytes.
func (l *Local) Stat(_ context.Context, key string) (int64, error) {
	full, err := l.resolve(key)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrNotFound
		}
		return 0, errors.New("storage: failed to stat object")
	}
	return info.Size(), nil
}
