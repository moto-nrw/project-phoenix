package common

import "errors"

// notFoundMarker is the shape of the repository not-found sentinel
// (models/base.ErrNotFound): the HTTP layer may import neither the model
// package nor database/sql (architecture policy: inbound-common/http may not
// import transaction-runtime/domain, nor the orm-sql external class), so the
// sentinel is recognised by its marker method instead of by identity.
type notFoundMarker interface {
	RepositoryNotFound()
}

// IsNotFound reports whether err carries the repository not-found sentinel,
// so a handler can answer 404 instead of 500. Mirrors models/base.IsNoRows
// for the api packages that must not import that package; errors.As walks
// the same chain errors.Is does, custom Is/As methods and joined errors
// included.
func IsNotFound(err error) bool {
	var target notFoundMarker
	return errors.As(err, &target)
}
