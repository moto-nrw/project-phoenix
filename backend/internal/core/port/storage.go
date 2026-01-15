// Package port defines interfaces (contracts) for adapters.
// These interfaces are defined by what the domain/service layer needs,
// following the Dependency Inversion Principle.
package port

import (
	"context"
	"errors"
	"io"
	"time"
)

// FileStorage defines the contract for file storage operations.
// Implementations can be local filesystem, S3, MinIO, etc.
// This follows 12-Factor App Factor 6 (Stateless Processes).
type FileStorage interface {
	// Save stores a file and returns its public URL path.
	// The key is a unique identifier for the file (e.g., "avatars/123_abc.jpg").
	Save(ctx context.Context, key string, content io.Reader, contentType string) (string, error)

	// Delete removes a file by its key.
	Delete(ctx context.Context, key string) error

	// Exists checks if a file exists.
	Exists(ctx context.Context, key string) (bool, error)

	// Open retrieves a file by key for streaming.
	Open(ctx context.Context, key string) (StoredFile, error)

	// GetPath returns the full filesystem path for a key (filesystem adapters only).
	// For non-filesystem storage, this should return an error.
	GetPath(ctx context.Context, key string) (string, error)
}

// StoredFile represents a readable file with metadata.
type StoredFile struct {
	Reader      io.ReadCloser
	Size        int64
	ModTime     time.Time
	ContentType string
}

// ErrFileNotFound indicates a missing file in storage.
var ErrFileNotFound = errors.New("storage: file not found")

// StorageConfig holds configuration for storage adapters.
type StorageConfig struct {
	// BasePath is the root directory for filesystem storage (if implemented).
	BasePath string

	// PublicURLPrefix is the URL prefix for public access (e.g., "/uploads")
	PublicURLPrefix string
}
