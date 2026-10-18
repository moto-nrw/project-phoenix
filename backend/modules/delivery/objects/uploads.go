package objects

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// UploadFiles binds upload byte operations to the existing local object backend.
// Paths, private modes, error mapping and directory discovery match every upload flow.
type UploadFiles struct{}

func (UploadFiles) PublicDir() (string, error) { return ResolvePublicDir() }
func (UploadFiles) StoredPath(publicDir, urlPath, requiredPrefix string) (string, error) {
	return ResolveStoredPath(publicDir, urlPath, requiredPrefix)
}

func (UploadFiles) SaveNamed(file io.Reader, targetDir, filename string, private bool) (string, error) {
	backend, dir, err := fileStore(targetDir)
	if err != nil {
		return "", errors.New("failed to create upload directory")
	}
	key, err := Key(filename)
	if err != nil {
		return "", errors.New("failed to save file")
	}
	if _, err := backend.WriteObject(context.Background(), key, file, SaveOptions{Private: private}); err != nil {
		return "", errors.New("failed to save file")
	}
	return filepath.Join(dir, filename), nil
}

func (u UploadFiles) RemoveUpload(filePath string) {
	if err := u.RemoveUploadContext(context.Background(), filePath); err != nil {
		slog.Default().Error("failed to remove file",
			slog.String("path", filePath),
			slog.String("error", err.Error()),
		)
	}
}

func (UploadFiles) RemoveUploadContext(ctx context.Context, filePath string) error {
	if filePath == "" {
		return nil
	}
	backend, _, err := fileStore(filepath.Dir(filePath))
	if err != nil {
		return err
	}
	key, err := Key(filepath.Base(filePath))
	if err != nil {
		return err
	}
	return backend.RemoveObject(ctx, key)
}

func fileStore(baseDir string) (Backend, string, error) {
	dir := baseDir
	if !filepath.IsAbs(dir) {
		if pubDir, err := ResolvePublicDir(); err == nil {
			// Strip leading "public/" since pubDir already points to it
			rel := strings.TrimPrefix(dir, "public/")
			rel = strings.TrimPrefix(rel, "public")
			if rel != "" {
				dir = filepath.Join(pubDir, rel)
			} else {
				dir = pubDir
			}
		}
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", errors.New("failed to resolve directory")
	}
	return NewLocal(absDir), absDir, nil
}

func UploadsBackend() (Backend, error) {
	backend, _, err := fileStore("public/uploads")
	return backend, err
}

func ResolveStoredPath(publicDir, urlPath, requiredPrefix string) (string, error) {
	if !strings.HasPrefix(urlPath, requiredPrefix) {
		return "", errors.New("invalid path")
	}

	if strings.Contains(urlPath, "..") {
		return "", errors.New("invalid path")
	}

	// Resolve relative production paths, but preserve an explicit absolute root.
	// Tests and callers with isolated storage must not be redirected to the
	// process-wide discovered public directory.
	baseDir := publicDir
	if !filepath.IsAbs(baseDir) {
		if resolved, err := ResolvePublicDir(); err == nil {
			baseDir = resolved
		}
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", errors.New("failed to resolve directory")
	}

	filePath := filepath.Join(baseDir, strings.TrimPrefix(urlPath, "/"))
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", errors.New("failed to resolve path")
	}

	// The URL starts with / (e.g. /uploads/...) so after TrimPrefix("/") we get "uploads/..."
	// which is inside the public dir. But verify to prevent traversal.
	if !strings.HasPrefix(absPath, absBase+string(os.PathSeparator)) {
		return "", errors.New("invalid path")
	}

	return absPath, nil
}

var (
	resolvedPublicDir string
	resolvedPublicErr error
	resolvePublicOnce sync.Once
)

// ResolvePublicDir finds the "public" directory by searching from the working directory
// and executable path. Handles both running from the backend/ dir and from the repo root.
// Result is cached after the first call.
func ResolvePublicDir() (string, error) {
	resolvePublicOnce.Do(func() {
		var candidates []string

		if wd, err := os.Getwd(); err == nil {
			candidates = append(candidates, publicDirCandidates(wd)...)
		}

		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, publicDirCandidates(filepath.Dir(exe))...)
		}

		for _, candidate := range candidates {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				continue
			}
			info, err := os.Stat(abs)
			if err == nil && info.IsDir() {
				resolvedPublicDir = abs
				return
			}
		}

		resolvedPublicErr = errors.New("public directory not found")
	})

	return resolvedPublicDir, resolvedPublicErr
}

// publicDirCandidates returns candidate paths for the public directory,
// walking up parent directories from base to find public/ or backend/public/.
func publicDirCandidates(base string) []string {
	var candidates []string
	for current := base; ; current = filepath.Dir(current) {
		candidates = append(candidates,
			filepath.Join(current, "public"),
			filepath.Join(current, "backend", "public"),
		)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return candidates
}

// Open resolves the same directory and key as writes, returning only stream facts.
func (UploadFiles) Open(ctx context.Context, baseDir, filename string) (io.ReadSeekCloser, time.Time, error) {
	backend, _, err := fileStore(baseDir)
	if err != nil {
		return nil, time.Time{}, err
	}
	key, err := Key(filename)
	if err != nil {
		return nil, time.Time{}, err
	}
	object, err := backend.OpenObject(ctx, key)
	if err != nil {
		return nil, time.Time{}, err
	}
	return object.Content, object.ModifiedAt, nil
}
