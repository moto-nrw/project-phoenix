package postgres

import (
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// LegacyWorkTimeModelNotFound restores the sql.ErrNoRows the legacy
// work-time repositories returned for a missing work-time model, which their
// callers still classify on.
func LegacyWorkTimeModelNotFound(err error) error {
	if errors.Is(err, workforce.ErrWorkTimeModelNotFound) {
		return sql.ErrNoRows
	}
	return err
}
