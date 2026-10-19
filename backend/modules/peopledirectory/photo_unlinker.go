package peopledirectory

import (
	"context"
	"log/slog"
)

// studentPhotoStoredURLPrefix is the persisted prefix of
// users.students.photo_path.
const studentPhotoStoredURLPrefix = "/uploads/student-photos/"

// PhotoFiles resolves and removes the stored file behind a photo URL. The
// root binds the Delivery Platform's upload files.
type PhotoFiles interface {
	StoredPath(publicDir, urlPath, requiredPrefix string) (string, error)
	RemoveUploadContext(context.Context, string) error
}

// PhotoUnlinker deletes the stored file behind a student photo URL; it serves
// the photo lifecycle's unlinker port.
type PhotoUnlinker struct {
	files     PhotoFiles
	logger    *slog.Logger
	publicDir string
}

// NewPhotoUnlinker builds the photo file cleanup over the upload files below
// publicDir. The files are required; a nil logger logs nothing.
func NewPhotoUnlinker(logger *slog.Logger, publicDir string, files PhotoFiles) *PhotoUnlinker {
	if files == nil {
		panic("student photo unlinker: upload storage is required")
	}
	return &PhotoUnlinker{logger: logger, publicDir: publicDir, files: files}
}

// UnlinkStored deletes the file backing storedURL. Errors are logged, never
// propagated — stale unlinks must not fail the surrounding DB write.
func (u *PhotoUnlinker) UnlinkStored(storedURL string) {
	if err := u.RemoveStored(context.Background(), storedURL); err != nil && u.logger != nil {
		u.logger.Warn("could not resolve student photo path for cleanup",
			slog.String("stored_url", storedURL),
			slog.String("error", err.Error()),
		)
	}
}

// RemoveStored returns removal failures to the existing cleanup outbox sweep.
func (u *PhotoUnlinker) RemoveStored(ctx context.Context, storedURL string) error {
	if storedURL == "" {
		return nil
	}
	path, err := u.files.StoredPath(u.publicDir, storedURL, studentPhotoStoredURLPrefix)
	if err != nil {
		return err
	}
	return u.files.RemoveUploadContext(ctx, path)
}
