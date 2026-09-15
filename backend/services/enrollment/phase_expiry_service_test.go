package enrollment

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type phaseExpiryRepositoryStub struct {
	snapshots      []*capability.PhaseExpirySnapshot
	asOf           timezone.Date
	warningThrough timezone.Date
}

func TestPhaseExpiryServiceRejectsMalformedDateWithoutPartialWarnings(t *testing.T) {
	t.Parallel()
	for _, completed := range []bool{false, true} {
		t.Run(fmt.Sprintf("completed=%t", completed), func(t *testing.T) {
			t.Parallel()
			invalid := &capability.PhaseExpirySnapshot{
				SourcePhaseID: 4, FirstAffectedDate: "2027-02-30",
				AffectedChildren: 1, UnresolvedChildren: 1,
			}
			if completed {
				successorID := int64(12)
				invalid.SuccessorPhaseID = &successorID
				invalid.UnresolvedChildren = 0
			}
			repo := &phaseExpiryRepositoryStub{snapshots: []*capability.PhaseExpirySnapshot{
				{SourcePhaseID: 3, FirstAffectedDate: "2027-02-01", AffectedChildren: 1, UnresolvedChildren: 1},
				invalid,
			}}
			warnings, err := NewPhaseExpiryService(repo).ListWarnings(context.Background(), timezone.NewDate(2027, 1, 2))
			require.Error(t, err)
			require.ErrorContains(t, err, "list phase expiry warnings:")
			assert.Nil(t, warnings)
		})
	}
}

func (s *phaseExpiryRepositoryStub) ListSnapshots(
	_ context.Context,
	asOf, warningThrough timezone.Date,
) ([]*capability.PhaseExpirySnapshot, error) {
	s.asOf = asOf
	s.warningThrough = warningThrough
	return s.snapshots, nil
}

func TestPhaseExpiryService_ListWarnings_UsesThirtyDayHorizonAndOmitsCompletedSuccessor(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2027, 1, 2)
	successorID := int64(12)
	repo := &phaseExpiryRepositoryStub{snapshots: []*capability.PhaseExpirySnapshot{
		{
			SourcePhaseID:      3,
			SourcePhaseName:    "1. Halbjahr",
			FirstAffectedDate:  capability.Date("2027-02-01"),
			AffectedChildren:   204,
			UnresolvedChildren: 204,
		},
		{
			SourcePhaseID:      4,
			SourcePhaseName:    "Vorjahr",
			SuccessorPhaseID:   &successorID,
			FirstAffectedDate:  capability.Date("2027-01-01"),
			AffectedChildren:   20,
			UnresolvedChildren: 2,
		},
		{
			SourcePhaseID:      5,
			SourcePhaseName:    "Erledigt",
			SuccessorPhaseID:   &successorID,
			FirstAffectedDate:  capability.Date("2027-01-01"),
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
