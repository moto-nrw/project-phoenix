package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// followBookingBounds asks Care Plan to carry the bounds the phase set on its
// offerings' bookings with a service-window edit (#6597, ADR 0205), before the
// sourced templates resync from those bookings. Care Plan decides under the
// bookings' locks which bound is still the phase's. A booking the edit would
// leave empty refuses the whole edit.
func (s *Phases) followBookingBounds(ctx context.Context, before, after *enrollment.Phase) error {
	if s.deps.Bookings == nil {
		return nil
	}
	err := s.deps.Bookings.FollowPhaseBookingBounds(ctx, enrollment.PhaseBookingMove{
		PhaseID:     after.ID,
		StartBefore: calendar.Date(before.ServiceStartDate), StartAfter: calendar.Date(after.ServiceStartDate),
		EndBefore: calendar.Date(before.ServiceEndDate), EndAfter: calendar.Date(after.ServiceEndDate),
		Today: s.todayDate(),
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, enrollment.ErrPhaseBookingBoundsConflict) && s.deps.Runtime.MarkRollback != nil {
		s.deps.Runtime.MarkRollback(ctx)
	}
	return fmt.Errorf("phase update: move booking bounds: %w", err)
}
