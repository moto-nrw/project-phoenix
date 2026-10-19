package owners

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// requestEditAdapter reads the plan facts an edited pickup request records
// again; decisions build on it.
type requestEditAdapter struct{ requestSubmissionAdapter }

func legacyEditError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrParentRequestStale):
		return parentrequests.ErrStale
	case errors.Is(err, careplan.ErrParentRequestNotPast):
		return parentrequests.ErrNotPast
	default:
		return err
	}
}
