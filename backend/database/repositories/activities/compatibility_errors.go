package activities

import modelBase "github.com/moto-nrw/project-phoenix/models/base"

// WrapDatabaseError preserves the legacy repository error contract for
// composition-layer adapters that cannot import legacy model infrastructure.
func WrapDatabaseError(operation string, err error) error {
	return &modelBase.DatabaseError{Op: operation, Err: err}
}

// WrapNotFoundDatabaseError preserves the typed legacy not-found contract.
func WrapNotFoundDatabaseError(operation string) error {
	return WrapDatabaseError(operation, modelBase.ErrNotFound)
}
