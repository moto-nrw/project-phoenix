// Package storage provides file storage implementations.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/sirupsen/logrus"
)

// LocalStorage implements port.FileStorage using the local filesystem.
// This is suitable for development but should be replaced with cloud storage
// (S3, MinIO) in production for 12-Factor compliance (Factor 6: Stateless Processes).
type LocalStorage struct {
	basePath        string
	publicURLPrefix string
	logger          *logrus.Logger
}

// NewLocalStorage creates a new LocalStorage adapter.
func NewLocalStorage(cfg port.StorageConfig, logger *logrus.Logger) (*LocalStorage, error) {
	if cfg.BasePath == "" {
		return nil, errors.New("storage: BasePath is required")
	}
	if strings.TrimSpace(cfg.PublicURLPrefix) == "" {
		return nil, errors.New("storage: PublicURLPrefix is required")
	}

	// Ensure base directory exists
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("storage: failed to create base directory: %w", err)
	}

	return &LocalStorage{
		basePath:        cfg.BasePath,
		publicURLPrefix: strings.TrimRight(cfg.PublicURLPrefix, "/"),
		logger:          logger,
	}, nil
}

// Save stores a file and returns its public URL path.
func (s *LocalStorage) Save(ctx context.Context, key string, content io.Reader, contentType string) (string, error) {
	// Validate key to prevent path traversal
	if err := s.validateKey(key); err != nil {
		return "", err
	}

	fullPath := filepath.Join(s.basePath, key)

	// Ensure parent directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("storage: failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("storage: failed to create file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			if s.logger != nil {
				s.logger.WithError(cerr).WithField("path", fullPath).Warn("Failed to close file")
			}
		}
	}()

	// Write content
	if _, err := io.Copy(file, content); err != nil {
		// Attempt cleanup on failure
		_ = os.Remove(fullPath)
		return "", fmt.Errorf("storage: failed to write file: %w", err)
	}

	// Return public URL
	publicURL := s.publicURLPrefix + "/" + strings.TrimLeft(filepath.ToSlash(key), "/")
	return publicURL, nil
}

// Delete removes a file by its key.
func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	if err := s.validateKey(key); err != nil {
		return err
	}

	fullPath := filepath.Join(s.basePath, key)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, not an error for delete operations
			return nil
		}
		return fmt.Errorf("storage: failed to delete file: %w", err)
	}

	return nil
}

// Exists checks if a file exists.
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	if err := s.validateKey(key); err != nil {
		return false, err
	}

	fullPath := filepath.Join(s.basePath, key)

	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("storage: failed to check file: %w", err)
}

// Open retrieves a file by its key for streaming.
func (s *LocalStorage) Open(ctx context.Context, key string) (port.StoredFile, error) {
	if err := s.validateKey(key); err != nil {
		return port.StoredFile{}, err
	}

	fullPath := filepath.Join(s.basePath, key)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return port.StoredFile{}, port.ErrFileNotFound
		}
		return port.StoredFile{}, fmt.Errorf("storage: failed to open file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return port.StoredFile{}, fmt.Errorf("storage: failed to stat file: %w", err)
	}

	return port.StoredFile{
		Reader:  file,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

// GetPath returns the full filesystem path for a key.
func (s *LocalStorage) GetPath(ctx context.Context, key string) (string, error) {
	if err := s.validateKey(key); err != nil {
		return "", err
	}

	fullPath := filepath.Join(s.basePath, key)

	// Verify the path is within base directory (security check)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("storage: failed to resolve path: %w", err)
	}

	absBase, err := filepath.Abs(s.basePath)
	if err != nil {
		return "", fmt.Errorf("storage: failed to resolve base path: %w", err)
	}

	if !strings.HasPrefix(absPath, absBase) {
		return "", errors.New("storage: path traversal detected")
	}

	return fullPath, nil
}

// validateKey ensures the key doesn't contain path traversal attempts.
func (s *LocalStorage) validateKey(key string) error {
	if key == "" {
		return errors.New("storage: key cannot be empty")
	}

	// Check for path traversal
	if strings.Contains(key, "..") {
		return errors.New("storage: invalid key (path traversal)")
	}

	// Check for absolute paths
	if filepath.IsAbs(key) {
		return errors.New("storage: key must be relative")
	}

	return nil
}

// Ensure LocalStorage implements port.FileStorage
var _ port.FileStorage = (*LocalStorage)(nil)
