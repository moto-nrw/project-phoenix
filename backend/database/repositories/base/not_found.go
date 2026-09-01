package base

import (
	"database/sql"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// TranslateNotFound converts the SQL adapter's missing-row identity to the
// persistence-neutral repository contract while preserving sql.ErrNoRows for
// repository-internal compatibility checks.
func TranslateNotFound(err error) error {
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return errors.Join(modelBase.ErrNotFound, err)
}
