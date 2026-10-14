package enrollmenttest

import "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

// The Enrollment compositions other owners' contract suites drive next to
// their own owner, bound as the composition binds them: the decision flow
// with its public refusal values, the care offering records over Care Plan's
// offerings and the manual planning query.
var (
	NewPublicDecisions     = compose.PublicDecisions
	NewCareOfferingRecords = compose.NewCareOfferingRecords
	NewManualPlanningQuery = compose.NewManualPlanningQuery
)
