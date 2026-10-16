//go:build testhooks

package api

import (
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// StudentHTTPTestRepositoryNotFound is the retained repositories' not-found
// sentinel (models/base.ErrNotFound), for suites that pin api/common's
// classifier against it.
var StudentHTTPTestRepositoryNotFound = workforceCompose.ErrRowNotFound
