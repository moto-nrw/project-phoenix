package enrollment

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrPhaseBoundsLock reports that the recurrence gate in front of the phase
// read could not be taken. The phase was not read.
var ErrPhaseBoundsLock = errors.New("enrollment: lock phase bounds")

// PhaseRows reads one phase of the school in context.
type PhaseRows interface {
	Phase(ctx context.Context, id int64) (*Phase, error)
}

// PhaseBoundsReader is the one reader of the phase bounds a write copies
// into bookings, enrollment windows or plans (#6657). In the caller's
// transaction it first takes the recurrence gate the phase edit holds while
// it moves the bounds (ADR 0205), then reads the phase. A write that starts
// while an edit is open waits for its commit and reads the new bounds; an
// edit that starts while the write is open waits for the write and then
// moves the bookings the write left. Writers take the gate before their
// request, child and booking locks. A phase read before the gate serves at
// most as a non-binding pre-check; its bounds must not be written.
type PhaseBoundsReader struct {
	phases PhaseRows
	lock   func(context.Context) error
}

// NewPhaseBoundsReader binds the reader to the phase rows and the
// recurrence gate. The root always binds the gate; focused tests without
// transactions may leave it nil and read without it.
func NewPhaseBoundsReader(phases PhaseRows, lock func(context.Context) error) PhaseBoundsReader {
	return PhaseBoundsReader{phases: phases, lock: lock}
}

// LockedPhase takes the recurrence gate and reads the phase. A gate failure
// wraps ErrPhaseBoundsLock; a read failure is returned as the phase rows
// report it, so callers keep their not-found mapping.
func (r PhaseBoundsReader) LockedPhase(ctx context.Context, id int64) (LockedPhase, error) {
	if r.phases == nil {
		return LockedPhase{}, errors.New("enrollment: phase bounds reader has no phase rows")
	}
	if r.lock != nil {
		if err := r.lock(ctx); err != nil {
			return LockedPhase{}, fmt.Errorf("%w: %w", ErrPhaseBoundsLock, err)
		}
	}
	phase, err := r.phases.Phase(ctx, id)
	if err != nil {
		return LockedPhase{}, err
	}
	if phase == nil {
		return LockedPhase{}, fmt.Errorf("enrollment: phase %d read as nil", id)
	}
	return LockedPhase{phase: phase}, nil
}

// LockedPhase is a phase PhaseBoundsReader read under the recurrence gate.
// Only the reader builds one; writers of phase-derived bounds take it
// instead of a *Phase, so a phase read elsewhere cannot reach them.
type LockedPhase struct {
	phase *Phase
}

// Phase is the phase as read under the gate; nil for the zero value.
func (p LockedPhase) Phase() *Phase { return p.phase }

// BookingWindow is the phase's service window as booking bounds: the first
// day of care and the exclusive day after the last one.
func (p LockedPhase) BookingWindow() (from, until calendar.Date, err error) {
	if p.phase == nil {
		return "", "", errors.New("enrollment: booking window of a phase not read under the recurrence gate")
	}
	return calendar.Date(p.phase.ServiceStartDate), calendar.Date(p.phase.ServiceEndDate).AddDays(1), nil
}

// ServiceWindow is the phase's first and last day of care.
func (p LockedPhase) ServiceWindow() (start, end calendar.Date, err error) {
	if p.phase == nil {
		return "", "", errors.New("enrollment: service window of a phase not read under the recurrence gate")
	}
	return calendar.Date(p.phase.ServiceStartDate), calendar.Date(p.phase.ServiceEndDate), nil
}
