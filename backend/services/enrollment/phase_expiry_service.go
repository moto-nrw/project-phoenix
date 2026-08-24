package enrollment

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

const phaseExpiryWarningDays = 30

const (
	PhaseExpiryStateMissingSuccessor = "missing_successor"
	PhaseExpiryStateIncomplete       = "incomplete"
)

// PhaseExpiryWarning is the administrator-facing decision: either the school
// still needs a successor phase or an existing successor still lacks effective
// bookings. Overdue switches the same warning from yellow to red.
type PhaseExpiryWarning struct {
	SourcePhaseID      int64         `json:"source_phase_id"`
	SourcePhaseName    string        `json:"source_phase_name"`
	SuccessorPhaseID   *int64        `json:"successor_phase_id,omitempty"`
	SuccessorPhaseName *string       `json:"successor_phase_name,omitempty"`
	FirstAffectedDate  timezone.Date `json:"first_affected_date"`
	AffectedChildren   int           `json:"affected_children"`
	UnresolvedChildren int           `json:"unresolved_children"`
	State              string        `json:"state"`
	Overdue            bool          `json:"overdue"`
}

type PhaseExpiryService interface {
	ListWarnings(ctx context.Context, asOf timezone.Date) ([]*PhaseExpiryWarning, error)
}

type phaseExpiryService struct {
	repo enrollmentModels.PhaseExpiryRepository
}

func NewPhaseExpiryService(repo enrollmentModels.PhaseExpiryRepository) PhaseExpiryService {
	return &phaseExpiryService{repo: repo}
}

func (s *phaseExpiryService) ListWarnings(
	ctx context.Context,
	asOf timezone.Date,
) ([]*PhaseExpiryWarning, error) {
	if asOf.IsZero() {
		return nil, errors.New("phase expiry report date is required")
	}
	if s.repo == nil {
		return nil, errors.New("phase expiry repository is required")
	}

	snapshots, err := s.repo.ListSnapshots(ctx, asOf, asOf.AddDays(phaseExpiryWarningDays))
	if err != nil {
		return nil, fmt.Errorf("list phase expiry warnings: %w", err)
	}

	warnings := make([]*PhaseExpiryWarning, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if err := validatePhaseExpirySnapshot(snapshot); err != nil {
			return nil, err
		}
		if snapshot.SuccessorPhaseID != nil && snapshot.UnresolvedChildren == 0 {
			continue
		}

		state := PhaseExpiryStateMissingSuccessor
		if snapshot.SuccessorPhaseID != nil {
			state = PhaseExpiryStateIncomplete
		}
		warnings = append(warnings, &PhaseExpiryWarning{
			SourcePhaseID:      snapshot.SourcePhaseID,
			SourcePhaseName:    snapshot.SourcePhaseName,
			SuccessorPhaseID:   snapshot.SuccessorPhaseID,
			SuccessorPhaseName: snapshot.SuccessorPhaseName,
			FirstAffectedDate:  snapshot.FirstAffectedDate,
			AffectedChildren:   snapshot.AffectedChildren,
			UnresolvedChildren: snapshot.UnresolvedChildren,
			State:              state,
			Overdue:            !snapshot.FirstAffectedDate.After(asOf),
		})
	}
	return warnings, nil
}

func validatePhaseExpirySnapshot(snapshot *enrollmentModels.PhaseExpirySnapshot) error {
	if snapshot == nil {
		return errors.New("phase expiry repository returned a nil snapshot")
	}
	if snapshot.SourcePhaseID <= 0 || snapshot.FirstAffectedDate.IsZero() {
		return fmt.Errorf("phase expiry repository returned an invalid snapshot for phase %d", snapshot.SourcePhaseID)
	}
	if snapshot.AffectedChildren <= 0 {
		return fmt.Errorf("phase expiry repository returned no affected children for phase %d", snapshot.SourcePhaseID)
	}
	if snapshot.UnresolvedChildren < 0 || snapshot.UnresolvedChildren > snapshot.AffectedChildren {
		return fmt.Errorf("phase expiry repository returned invalid unresolved count for phase %d", snapshot.SourcePhaseID)
	}
	return nil
}
