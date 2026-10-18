package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
)

// studentAbsenceNotifier binds the student routes to Delivery without exposing
// the producer's application vocabulary to the HTTP consumer.
type studentAbsenceNotifier struct {
	notifier deliveryCompose.AbsenceNotifier
}

func (n studentAbsenceNotifier) NotifyAbsenceReported(ctx context.Context, report studentpresence.AbsenceReport) error {
	if n.notifier == nil {
		return nil
	}
	return n.notifier.NotifyAbsenceReported(ctx, deliveryCompose.AbsenceReport{
		TenantID: report.TenantID, StudentIDs: report.StudentIDs, Status: report.Status,
		Dates: report.Dates, FromParent: report.FromParent, ActorAccountID: report.ActorAccountID,
	})
}
