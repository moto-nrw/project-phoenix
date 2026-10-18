package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// notifyDecision appends both guardian audiences before the decision commits.
func (w *ExcusedRequests) notifyDecision(ctx context.Context, req *careplan.ExcusedAbsenceRequest, event ports.RequestEvent) error {
	if err := w.emitRequestPill(ctx, req, event); err != nil {
		return err
	}
	return w.notifyOtherGuardians(ctx, req, event)
}
