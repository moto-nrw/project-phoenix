package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// PickupExceptionLister lists a child's pickup exceptions (Care Plan).
type PickupExceptionLister interface {
	ListPickupExceptions(context.Context, ports.PickupExceptionFilter) ([]ports.PickupExceptionProjection, error)
}

// PartialAbsencePreview lists the blocks of a child's day an earlier pickup
// still excuses. Since the presence cutover (#2762) the joined read of the
// plan and the attendance lives in the Timetable projection; the two facts
// the read needs from other owners stay with them: People Directory says
// whether the child is still enrolled (a graduate gets no course blocks
// the roster does not list yet), Care Plan names the automatic excusals
// whose rows an approval may claim again.
type PartialAbsencePreview struct {
	*application.PartialAbsencePreview
}

// NewPartialAbsencePreview binds the preview: enrolled answers whether the
// child is still enrolled, exceptions lists the child's pickup exceptions.
func NewPartialAbsencePreview(reads PresenceReads, enrolled func(context.Context, int64) (bool, error), exceptions PickupExceptionLister) PartialAbsencePreview {
	return PartialAbsencePreview{application.NewPartialAbsencePreview(reads, enrolled, exceptions, tenant.NewTransactionRunner().RunInTx)}
}
