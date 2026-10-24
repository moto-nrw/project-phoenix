package planning

import "errors"

// isRecordNotFound reports whether err carries the repository not-found
// marker anywhere in its chain: the missing-row answer of the row adapters
// the composition binds, matched by shape rather than by identity.
func isRecordNotFound(err error) bool {
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}
