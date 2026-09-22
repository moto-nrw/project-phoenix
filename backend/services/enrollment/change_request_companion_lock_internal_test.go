package enrollment

import (
	"context"
	"errors"
	"testing"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type busyCompanionGraph struct{ err error }

func (g busyCompanionGraph) LockCompanionGraph(context.Context, []int64, []int64) error { return g.err }

func (busyCompanionGraph) VerifyCompanionStrandingBatch(context.Context) error { return nil }

// A linked child held by another writer is a retriable conflict. Care Plan
// reports it with its own sentinel since #3427; the approval must still hand
// the handler the student write's sentinel, which it maps to 409 instead of a
// server error.
func TestLockApprovedStudentsKeepsTheRetriableLockConflict(t *testing.T) {
	t.Parallel()
	studentID := int64(7)
	children := []*RequestChild{{CreatedStudentID: &studentID}}

	s := &changeRequestService{CompanionGraphLocker: busyCompanionGraph{err: careplan.ErrCompanionLockBusy}}
	err := s.lockApprovedStudents(context.Background(), children)
	require.Error(t, err)
	assert.ErrorIs(t, err, userModels.ErrCompanionLockBusy)

	other := errors.New("database unavailable")
	s = &changeRequestService{CompanionGraphLocker: busyCompanionGraph{err: other}}
	err = s.lockApprovedStudents(context.Background(), children)
	require.ErrorIs(t, err, other)
	assert.NotErrorIs(t, err, userModels.ErrCompanionLockBusy)
}
