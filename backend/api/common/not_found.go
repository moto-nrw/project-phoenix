package common

// notFoundSentinelText is the message of the repository not-found sentinel
// (models/base.ErrNotFound), which repositories join onto sql.ErrNoRows.
//
// The HTTP layer may import neither the model package nor database/sql
// (architecture policy: inbound-common/http may not import
// transaction-runtime/domain, nor the orm-sql external class), so the
// sentinel is matched by its stable message instead of by identity. The
// chain is walked by hand because errors.Is needs the sentinel value itself.
const notFoundSentinelText = "repository: not found"

// IsNotFound reports whether err carries the repository not-found sentinel,
// so a handler can answer 404 instead of 500. Mirrors models/base.IsNoRows
// for the api packages that must not import that package.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if err.Error() == notFoundSentinelText {
		return true
	}
	switch unwrapper := err.(type) {
	case interface{ Unwrap() error }:
		return IsNotFound(unwrapper.Unwrap())
	case interface{ Unwrap() []error }:
		for _, wrapped := range unwrapper.Unwrap() {
			if IsNotFound(wrapped) {
				return true
			}
		}
	}
	return false
}
