package students

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/api/common"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// photoUnlinker adapts api/common file deletion to the service's
// PhotoUnlinker dep. Lives here (not in services/users) to avoid an
// api/common ↔ services/users import cycle.
type photoUnlinker struct {
	logger    *slog.Logger
	publicDir string
}

func NewPhotoUnlinker(logger *slog.Logger, publicDir string) userService.PhotoUnlinker {
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
