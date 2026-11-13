package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
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
	return NewChangeRequests(ChangeRequestDependencies{Settings: intakeSettingsFake{},
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
	children := []*enrollment.RequestChild{{CreatedStudentID: &studentID}}

	s := companionLockChangeRequests(busyCompanionGraph{err: careplan.ErrCompanionLockBusy})
	err := s.lockApprovedStudents(context.Background(), children, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, errStudentCompanionLockBusy)

	other := errors.New("database unavailable")
	s = companionLockChangeRequests(busyCompanionGraph{err: other})
	err = s.lockApprovedStudents(context.Background(), children, nil)
	require.ErrorIs(t, err, other)
	assert.NotErrorIs(t, err, errStudentCompanionLockBusy)
}

// skippingGuardianHold models an identity whose missing profile or inactive
// membership returns no grants before the guard reaches the child locks.
type skippingGuardianHold struct {
	GuardianStudentHold
	graphs [][]int64
}

func (h *skippingGuardianHold) HoldGuardianEmailStudentsPermission(_ context.Context, _ string, ids, graph []int64, _ int64, _ string) (map[int64]bool, error) {
	h.graphs = append(h.graphs, append([]int64(nil), graph...))
	if len(h.graphs) == 1 {
		return nil, nil
	}
	grants := map[int64]bool{}
	for _, id := range ids {
		grants[id] = true
	}
	return grants, nil
}

func TestApprovalKeepsCompanionGraphAfterAnIdentitySkipsTheLocks(t *testing.T) {
	t.Parallel()
	hold := &skippingGuardianHold{}
	svc := NewChangeRequests(ChangeRequestDependencies{Settings: intakeSettingsFake{},
		GuardianHold: hold, Companions: busyCompanionGraph{}, ParentsURL: "http://localhost",
	})
	run := &approval{req: &enrollment.Request{}, reads: &approvalAuthorityReads{holds: map[approvalAuthorityKey]renewalHolds{}}}
	run.req.TenantID = 77
	authority := newApprovalAuthority(svc, run)
	studentID := int64(90)
	authority.graph = []int64{studentID}
	for _, email := range []string{"absent@example.test", "present@example.test"} {
		var submitter *reEnrollmentSubmitter
		require.NoError(t, authority.add(context.Background(), &enrollment.Request{GuardianEmail: email}, &submitter, studentID, false))
	}
	require.NoError(t, authority.hold(context.Background()))
	require.Len(t, hold.graphs, 2)
	require.Equal(t, []int64{studentID}, hold.graphs[0])
	require.Equal(t, []int64{studentID}, hold.graphs[1],
		"the next identity must lock companions with its students when the first identity acquired no locks")
}
