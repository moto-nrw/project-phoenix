package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type busyCompanionGraph struct{ err error }

func (g busyCompanionGraph) LockCompanionGraph(context.Context, []int64, []int64) error { return g.err }

func (busyCompanionGraph) VerifyCompanionStrandingBatch(context.Context) error { return nil }

// errStudentCompanionLockBusy stands in for the student write's retriable
// conflict the root binds as CompanionLockBusy.
var errStudentCompanionLockBusy = errors.New("companion lock busy")

func companionLockChangeRequests(graph CompanionGraphCoordinator) *ChangeRequests {
	return NewChangeRequests(ChangeRequestDependencies{
		Companions:        graph,
		CompanionLockBusy: errStudentCompanionLockBusy,
		ParentsURL:        "https://parents.example.test",
	})
}

// A linked child held by another writer is a retriable conflict. Care Plan
// reports it with its own sentinel since #3427; the approval must still hand
// the handler the student write's sentinel, which it maps to 409 instead of a
// server error.
func TestLockApprovedStudentsKeepsTheRetriableLockConflict(t *testing.T) {
	t.Parallel()
	studentID := int64(7)
	children := []*RequestChild{{CreatedStudentID: &studentID}}

	s := companionLockChangeRequests(busyCompanionGraph{err: careplan.ErrCompanionLockBusy})
	err := s.lockApprovedStudents(context.Background(), children)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStudentCompanionLockBusy)

	other := errors.New("database unavailable")
	s = companionLockChangeRequests(busyCompanionGraph{err: other})
	err = s.lockApprovedStudents(context.Background(), children)
	require.ErrorIs(t, err, other)
	assert.NotErrorIs(t, err, errStudentCompanionLockBusy)
}
