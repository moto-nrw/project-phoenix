package common

import (
	"context"
	"io"
	"time"

	storage "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
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
	return s.backend.WriteObject(ctx, key, source, storage.SaveOptions{Private: true})
}

func (s *PrivateUploads) OpenPrivate(ctx context.Context, kind string, tenantID int64, storedName string) (interface {
	io.ReadSeekCloser
	ModTime() time.Time
}, error) {
	key, err := storage.TenantKey(kind, tenantID, storedName)
	if err != nil {
		return nil, err
	}
	object, err := s.backend.OpenObject(ctx, key)
	if err != nil {
		return nil, err
	}
	return &privateUploadObject{ReadSeekCloser: object.Content, modifiedAt: object.ModifiedAt}, nil
}

func (s *PrivateUploads) RemovePrivate(ctx context.Context, kind string, tenantID int64, storedName string) error {
	key, err := storage.TenantKey(kind, tenantID, storedName)
	if err != nil {
		return err
	}
	return s.backend.RemoveObject(ctx, key)
}

type privateUploadObject struct {
	io.ReadSeekCloser
	modifiedAt time.Time
}

func (o *privateUploadObject) ModTime() time.Time { return o.modifiedAt }
