// Package storage provides file storage implementations.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/sirupsen/logrus"
)

// MemoryStorage implements port.FileStorage using in-memory storage.
// This is suitable for development or tests and is non-persistent.
type MemoryStorage struct {
	publicURLPrefix string
	logger          *logrus.Logger

	mu    sync.RWMutex
	files map[string]memoryFile
}

type memoryFile struct {
	data        []byte
	contentType string
	modTime     time.Time
}

// NewMemoryStorage creates a new in-memory storage adapter.
func NewMemoryStorage(cfg port.StorageConfig, logger *logrus.Logger) (*MemoryStorage, error) {
	if strings.TrimSpace(cfg.PublicURLPrefix) == "" {
		return nil, errors.New("storage: PublicURLPrefix is required")
	}

	return &MemoryStorage{
		publicURLPrefix: strings.TrimRight(cfg.PublicURLPrefix, "/"),
		logger:          logger,
		files:           make(map[string]memoryFile),
	}, nil
}

// Save stores a file and returns its public URL path.
func (s *MemoryStorage) Save(ctx context.Context, key string, content io.Reader, contentType string) (string, error) {
	if err := s.validateKey(key); err != nil {
		return "", err
	}

	data, err := io.ReadAll(content)
	if err != nil {
		return "", fmt.Errorf("storage: failed to read content: %w", err)
	}

	s.mu.Lock()
	s.files[key] = memoryFile{
		data:        data,
		contentType: contentType,
		modTime:     time.Now().UTC(),
	}
	s.mu.Unlock()

	publicURL := s.publicURLPrefix + "/" + strings.TrimLeft(filepath.ToSlash(key), "/")
	return publicURL, nil
}

// Delete removes a file by its key.
func (s *MemoryStorage) Delete(ctx context.Context, key string) error {
	if err := s.validateKey(key); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.files, key)
	s.mu.Unlock()
	return nil
}

// Exists checks if a file exists.
func (s *MemoryStorage) Exists(ctx context.Context, key string) (bool, error) {
	if err := s.validateKey(key); err != nil {
		return false, err
	}

	s.mu.RLock()
	_, ok := s.files[key]
	s.mu.RUnlock()
	return ok, nil
}

// Open retrieves a file by its key for streaming.
func (s *MemoryStorage) Open(ctx context.Context, key string) (port.StoredFile, error) {
	if err := s.validateKey(key); err != nil {
		return port.StoredFile{}, err
	}

	s.mu.RLock()
	file, ok := s.files[key]
	s.mu.RUnlock()
	if !ok {
		return port.StoredFile{}, port.ErrFileNotFound
	}

	reader := io.NopCloser(bytes.NewReader(file.data))
	return port.StoredFile{
		Reader:      reader,
		Size:        int64(len(file.data)),
		ModTime:     file.modTime,
		ContentType: file.contentType,
	}, nil
}

// GetPath returns an error for memory storage (no filesystem path).
func (s *MemoryStorage) GetPath(ctx context.Context, key string) (string, error) {
	return "", errors.New("storage: filesystem path not available for memory storage")
}

// validateKey ensures the key doesn't contain path traversal attempts.
func (s *MemoryStorage) validateKey(key string) error {
	if key == "" {
		return errors.New("storage: key cannot be empty")
	}
	if strings.Contains(key, "..") {
		return errors.New("storage: invalid key (path traversal)")
	}
	if filepath.IsAbs(key) {
		return errors.New("storage: key must be relative")
	}
	return nil
}

// Ensure MemoryStorage implements port.FileStorage.
var _ port.FileStorage = (*MemoryStorage)(nil)
