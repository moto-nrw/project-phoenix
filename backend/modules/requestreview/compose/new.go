package compose

import "github.com/moto-nrw/project-phoenix/modules/requestreview"

type ProjectionDependencies = requestreview.Dependencies
type Queues = requestreview.Queues

// NewProjection validates native consumer ports. Each owner queue is composed
// separately; this constructor contains no owner rules or retained adapters.
func NewProjection(deps ProjectionDependencies) (requestreview.Query, error) {
	return requestreview.NewChecked(deps)
}
