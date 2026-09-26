package services

import (
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

// StudentHTTPTestWithdrawalFilter pages the pending withdrawal tasks the
// student route suites read back through the retained repository.
type StudentHTTPTestWithdrawalFilter = userModels.CareWithdrawalCompletionFilter

// StudentHTTPTestRepositoryNotFound is the retained repositories' not-found
// sentinel (models/base.ErrNotFound), for suites that pin api/common's
// classifier against it.
var StudentHTTPTestRepositoryNotFound = timetracking.ErrNotFound
