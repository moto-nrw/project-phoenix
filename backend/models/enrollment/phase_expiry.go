package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// PhaseExpirySnapshot is the repository projection used to decide whether an
// expiring school-year phase needs an administrator warning. Counts are per
// distinct active student, never per request or offering row.
type PhaseExpirySnapshot struct {
	SourcePhaseID      int64         `bun:"source_phase_id"`
	SourcePhaseName    string        `bun:"source_phase_name"`
	SuccessorPhaseID   *int64        `bun:"successor_phase_id"`
	SuccessorPhaseName *string       `bun:"successor_phase_name"`
	FirstAffectedDate  timezone.Date `bun:"first_affected_date"`
	AffectedChildren   int           `bun:"affected_children"`
	UnresolvedChildren int           `bun:"unresolved_children"`
}

// PhaseExpiryRepository projects the current tenant's phase-expiry risk up to
// a caller-supplied horizon. It deliberately returns completed successors as
// well; the service owns the warning-state decision.
type PhaseExpiryRepository interface {
	ListSnapshots(ctx context.Context, asOf, warningThrough timezone.Date) ([]*PhaseExpirySnapshot, error)
}
