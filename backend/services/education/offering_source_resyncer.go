package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// OfferingSourceResyncer re-reconciles every offering-sourced timetable
// template of the tenant (#2137) after a class rewrite. A promotion moves
// children between Jahrgängen, so templates with a Jahrgang filter must drop
// the children that left the filter and pick up the ones that entered it.
// Implemented by the enrollment decision service; the student handlers and
// the grade transition workflow composition consume it.
type OfferingSourceResyncer interface {
	ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom timezone.Date) error
}
