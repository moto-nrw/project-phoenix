package careplanhttp

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// CareResumption is this HTTP adapter's workflow entry port. Root binds it
// directly to the care-exit workflow, never through the domain lifecycle.
// Care Plan supplies the input value and error identities, not the command.
type CareResumption func(context.Context, careplan.CareResumeInput) error
