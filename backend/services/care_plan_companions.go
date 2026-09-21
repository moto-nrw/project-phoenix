package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// companionGraphCoordinator serves the enrollment approval's multi-child
// companion write: Care Plan's lock protocol over the companion graph, and
// People Directory's stranding verdict over the final departure plans.
type companionGraphCoordinator struct {
	careplan.CompanionLocks
	strandings interface {
		VerifyCompanionStrandingBatch(ctx context.Context) error
	}
}

func (c companionGraphCoordinator) VerifyCompanionStrandingBatch(ctx context.Context) error {
	return c.strandings.VerifyCompanionStrandingBatch(ctx)
}
