package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// recordMissing marks a failure that carries the driver's no-rows error with
// the public ErrRecordMissing and keeps the store's text, so the HTTP
// adapters need not know the driver (#2736).
func recordMissing(err error) error {
	if err == nil || !errors.Is(err, transactionruntime.ErrNoRows) || errors.Is(err, identityaccess.ErrRecordMissing) {
		return err
	}
	return &translatedError{text: err.Error(), public: identityaccess.ErrRecordMissing, cause: err}
}
