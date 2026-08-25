package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

type phaseExpiryRepositoryStub struct {
	snapshots      []*enrollmentModels.PhaseExpirySnapshot
	asOf           timezone.Date
	warningThrough timezone.Date
}

func (s *phaseExpiryRepositoryStub) ListSnapshots(
	_ context.Context,
	asOf, warningThrough timezone.Date,
) ([]*enrollmentModels.PhaseExpirySnapshot, error) {
	s.asOf = asOf
	s.warningThrough = warningThrough
	return s.snapshots, nil
}

func TestPhaseExpiryService_ListWarnings_UsesThirtyDayHorizonAndOmitsCompletedSuccessor(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2027, 1, 2)
	successorID := int64(12)
	repo := &phaseExpiryRepositoryStub{snapshots: []*enrollmentModels.PhaseExpirySnapshot{
		{
			SourcePhaseID:      3,
			SourcePhaseName:    "1. Halbjahr",
			FirstAffectedDate:  timezone.NewDate(2027, 2, 1),
			AffectedChildren:   204,
			UnresolvedChildren: 204,
		},
		{
			SourcePhaseID:      4,
			SourcePhaseName:    "Vorjahr",
			SuccessorPhaseID:   &successorID,
			FirstAffectedDate:  timezone.NewDate(2027, 1, 1),
			AffectedChildren:   20,
			UnresolvedChildren: 2,
		},
		{
			SourcePhaseID:      5,
			SourcePhaseName:    "Erledigt",
			SuccessorPhaseID:   &successorID,
			FirstAffectedDate:  timezone.NewDate(2027, 1, 1),
			AffectedChildren:   20,
			UnresolvedChildren: 0,
		},
	}}

	warnings, err := NewPhaseExpiryService(repo).ListWarnings(context.Background(), today)
	require.NoError(t, err)
	assert.Equal(t, today, repo.asOf)
	assert.Equal(t, timezone.NewDate(2027, 2, 1), repo.warningThrough)
	require.Len(t, warnings, 2)

	assert.Equal(t, PhaseExpiryStateMissingSuccessor, warnings[0].State)
	assert.False(t, warnings[0].Overdue)
	assert.Equal(t, 204, warnings[0].UnresolvedChildren)

	assert.Equal(t, PhaseExpiryStateIncomplete, warnings[1].State)
	assert.True(t, warnings[1].Overdue)
	assert.Equal(t, 2, warnings[1].UnresolvedChildren)
}
