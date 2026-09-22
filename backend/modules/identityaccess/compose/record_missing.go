package compose

import (
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// recordMissing marks a failure that carries the driver's no-rows error with
// the public ErrRecordMissing and keeps the store's text, so the HTTP
// adapters need not know the driver (#2736).
func recordMissing(err error) error {
	if err == nil || !errors.Is(err, sql.ErrNoRows) || errors.Is(err, identityaccess.ErrRecordMissing) {
		return err
	}
	return &translatedError{text: err.Error(), public: identityaccess.ErrRecordMissing, cause: err}
}
