package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/require"
)

// Compare the route-specific batch with the retained scalar decision read,
// including validation of excluded siblings and tolerant phase lookups.
func TestStudentExtraFieldRequestsPreservesScalarResults(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"historical phases", "missing phase", "phase batch failure", "children batch failure", "guardians batch failure", "invalid sibling", "invalid later child", "invalid request", "optional guardians", "empty", "invalid student"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			records := extraFieldRequestRecords(t)
			studentID := int64(41)
			var guardians DecisionGuardians = records
			switch scenario {
			case "missing phase":
				records.phases = records.phases[:1]
			case "phase batch failure", "children batch failure", "guardians batch failure":
				records.batchFailure = scenario
			case "invalid sibling":
				records.children[0].CustomData = json.RawMessage(`[]`)
			case "invalid later child":
				records.children[1].CustomData = json.RawMessage(`[]`)
			case "invalid request":
				records.requests[1].CustomData = json.RawMessage(`[]`)
			case "optional guardians":
				guardians = nil
			case "empty":
				records.requests = nil
			case "invalid student":
				studentID = 0
			}
			decisions := NewDecisions(DecisionDependencies{Requests: records, Children: records, Phases: records, Guardians: guardians, Settings: unusedDecisionSettings{}})
			want, wantErr := decisions.StudentDecisionRequests(t.Context(), studentID)
			got, err := decisions.StudentExtraFieldRequests(t.Context(), studentID)
			if wantErr != nil {
				require.EqualError(t, err, wantErr.Error())
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

type extraFieldRequestStub struct {
	DecisionRequests
	DecisionChildren
	DecisionPhases
	DecisionGuardians
	requests     []*enrollment.Request
	children     []*enrollment.RequestChild
	phases       []*enrollment.Phase
	guardians    []*enrollment.RequestGuardian
	batchFailure string
}

func extraFieldRequestRecords(t *testing.T) *extraFieldRequestStub {
	t.Helper()
	studentID, siblingID := int64(41), int64(42)
	return &extraFieldRequestStub{
		requests: []*enrollment.Request{{ID: 2, PhaseID: 20}, {ID: 1, PhaseID: 10}},
		// Owner batches order by request_id, then sort_order/id, not the
		// newest-first submission order. Keep both independent orders.
		children: []*enrollment.RequestChild{
			{ID: 11, RequestID: 1, CreatedStudentID: &siblingID, DateOfBirth: "2018-01-02"},
			{ID: 12, RequestID: 1, CreatedStudentID: &studentID, DateOfBirth: "2018-02-03"},
			{ID: 21, RequestID: 2, CreatedStudentID: &studentID, DateOfBirth: "2018-02-03"},
			{ID: 22, RequestID: 2, CreatedStudentID: &studentID, DateOfBirth: "2018-02-03"},
		},
		phases:    []*enrollment.Phase{{ID: 10, Name: "Historical", IsActive: false}, {ID: 20, Name: "Current", IsActive: true}},
		guardians: []*enrollment.RequestGuardian{{ID: 1, RequestID: 1}, {ID: 2, RequestID: 2}, {ID: 3, RequestID: 2}},
	}
}

func (s *extraFieldRequestStub) AdminRequests(_ context.Context, filters enrollment.RequestListFilters) ([]*enrollment.Request, error) {
	if filters.CreatedStudentID != 41 {
		return nil, errors.New("student filter missing")
	}
	return s.requests, nil
}

func (s *extraFieldRequestStub) ChildrenForRequest(_ context.Context, requestID int64, lock bool) ([]*enrollment.RequestChild, error) {
	if lock {
		return nil, errors.New("unexpected child lock")
	}
	var rows []*enrollment.RequestChild
	for _, row := range s.children {
		if row.RequestID == requestID {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (s *extraFieldRequestStub) ChildrenForRequests(context.Context, []int64) ([]*enrollment.RequestChild, error) {
	if s.batchFailure == "children batch failure" {
		return nil, errors.New(s.batchFailure)
	}
	return s.children, nil
}

func (s *extraFieldRequestStub) Phase(_ context.Context, id int64) (*enrollment.Phase, error) {
	for _, phase := range s.phases {
		if phase.ID == id {
			return phase, nil
		}
	}
	return nil, errors.New("phase missing")
}

func (s *extraFieldRequestStub) PhasesByID(context.Context, []int64) ([]*enrollment.Phase, error) {
	if s.batchFailure == "phase batch failure" {
		return nil, errors.New(s.batchFailure)
	}
	return s.phases, nil
}

func (s *extraFieldRequestStub) RequestGuardians(_ context.Context, ids []int64) ([]*enrollment.RequestGuardian, error) {
	if len(ids) > 1 && s.batchFailure == "guardians batch failure" {
		return nil, errors.New(s.batchFailure)
	}
	var rows []*enrollment.RequestGuardian
	for _, row := range s.guardians {
		for _, id := range ids {
			if row.RequestID == id {
				rows = append(rows, row)
			}
		}
	}
	return rows, nil
}
