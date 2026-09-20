package common

import (
	"context"
	"io"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/storage"
)

// PrivateUploads binds managed tenant objects to the shared uploads backend.
// Callers supply the storage kind; tenant keys are validated before every access.
type PrivateUploads struct{ backend storage.Backend }

// PrivateUploadsBackend resolves the same uploads root as other document flows.
func PrivateUploadsBackend() (*PrivateUploads, error) {
	backend, err := UploadsBackend()
	if err != nil {
		return nil, err
	}
	return &PrivateUploads{backend: backend}, nil
}

func (s *PrivateUploads) SavePrivate(ctx context.Context, kind string, tenantID int64, storedName string, source io.Reader) (int64, error) {
	key, err := storage.TenantKey(kind, tenantID, storedName)
	if err != nil {
		return 0, err
	}
	return s.backend.Save(ctx, key, source, storage.SaveOptions{Private: true})
}

func (s *PrivateUploads) OpenPrivate(ctx context.Context, kind string, tenantID int64, storedName string) (interface {
	io.ReadSeekCloser
	ModTime() time.Time
}, error) {
	key, err := storage.TenantKey(kind, tenantID, storedName)
	if err != nil {
		return nil, err
	}
	return s.backend.Open(ctx, key)
}

func (s *PrivateUploads) RemovePrivate(ctx context.Context, kind string, tenantID int64, storedName string) error {
	key, err := storage.TenantKey(kind, tenantID, storedName)
	if err != nil {
		return err
	}
	return s.backend.Remove(ctx, key)
}
