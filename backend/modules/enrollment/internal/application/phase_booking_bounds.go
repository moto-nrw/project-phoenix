package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	var conflict *enrollment.PhaseBookingBoundsConflictError
	if errors.As(err, &conflict) {
		conflict.ChildName = s.applicationChildName(ctx, after.ID, conflict.RequestChildID)
	}
	return fmt.Errorf("phase update: move booking bounds: %w", err)
}

// applicationChildName names the child of a request child of the phase for
// a refusal. A failed lookup leaves the name empty; the refusal stands.
func (s *Phases) applicationChildName(ctx context.Context, phaseID, requestChildID int64) string {
	rows, err := s.deps.Records.PhaseResponseChildren(ctx, phaseID, enrollment.ChildStatuses())
	if err != nil {
		return ""
	}
	for _, row := range rows {
		if row.Child != nil && row.Child.ID == requestChildID {
			return strings.TrimSpace(row.Child.FirstName + " " + row.Child.LastName)
		}
	}
	return ""
}
