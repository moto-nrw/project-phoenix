package testutil

import "github.com/moto-nrw/project-phoenix/services"

// StudentRouteCareLifecycleConfig tunes the Care Plan lifecycle
// NewStudentRouteCareLifecycle composes.
type StudentRouteCareLifecycleConfig = services.StudentHTTPTestCareLifecycle

// StudentRouteWithdrawalFilter pages the pending withdrawal tasks a student
// route suite reads back.
type StudentRouteWithdrawalFilter = services.StudentHTTPTestWithdrawalFilter

// StudentRouteRepositoryNotFound is the retained repositories' not-found
// sentinel.
var StudentRouteRepositoryNotFound = services.StudentHTTPTestRepositoryNotFound
