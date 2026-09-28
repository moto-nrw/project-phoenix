package students

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// photoUnlinker adapts api/common file deletion to the service's
// photo unlinker the People Directory photo runtime needs; the root hands it
// to that runtime.
type photoUnlinker struct {
	logger    *slog.Logger
	publicDir string
}

// PhotoUnlinker deletes a previously stored photo file.
type PhotoUnlinker interface {
	UnlinkStored(storedURL string)
}

func NewPhotoUnlinker(logger *slog.Logger, publicDir string) PhotoUnlinker {
	return &photoUnlinker{logger: logger, publicDir: publicDir}
}

// UnlinkStored deletes the file backing storedURL. Errors are logged, never
// propagated — stale unlinks must not fail the surrounding DB write.
func (u *photoUnlinker) UnlinkStored(storedURL string) {
	if storedURL == "" {
		return
	}
	path, err := common.ResolveStoredPath(u.publicDir, storedURL, common.StudentPhotoStoredURLPrefix)
	if err != nil {
		if u.logger != nil {
			u.logger.Warn("could not resolve student photo path for cleanup",
				slog.String("stored_url", storedURL),
				slog.String("error", err.Error()),
			)
		}
		return
	}
	common.RemoveImage(path)
}
