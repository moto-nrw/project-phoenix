package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// StudentExtraFieldRequests keeps the decision-history read independent from
// the extra-field read graph. All reads join the caller's tenant transaction.
func (d *Decisions) StudentExtraFieldRequests(ctx context.Context, studentID int64) (result []*enrollment.DecisionSummary, err error) {
	defer func() { err = PublicError(err) }()
	if studentID <= 0 {
		return nil, fmt.Errorf("decision: student_id required")
	}
	return d.listDecisionRequests(ctx, enrollment.RequestListFilters{CreatedStudentID: studentID}, "decision: list requests by student")
}

// assembleRequests assembles every request from one phase, one child and one
// guardian read shared by all of them (#5778, decision E2: an early shared
// read may replace the per-request READ COMMITTED reads; these listings take
// no lock). A positive studentID keeps only that student's children and
// drops requests without one. A failed or incomplete bulk read falls back to
// the per-request assembly, which keeps its phase tolerance and errors.
func (d *Decisions) assembleRequests(ctx context.Context, requests []*enrollment.Request, studentID int64) ([]*enrollment.DecisionSummary, error) {
	requestIDs := make([]int64, 0, len(requests))
	phaseIDs := make([]int64, 0, len(requests))
	seen := make(map[int64]bool, len(requests))
	for _, request := range requests {
		requestIDs = append(requestIDs, request.ID)
		if !seen[request.PhaseID] {
			seen[request.PhaseID] = true
			phaseIDs = append(phaseIDs, request.PhaseID)
		}
	}
	phases, phaseErr := d.deps.Phases.PhasesByID(ctx, phaseIDs)
	children, err := d.deps.Children.ChildrenForRequests(ctx, requestIDs)
	if err != nil {
		return d.assembleAll(ctx, requests, studentID)
	}
	var guardians []*enrollment.RequestGuardian
	if d.deps.Guardians != nil {
		guardians, err = d.deps.Guardians.RequestGuardians(ctx, requestIDs)
		if err != nil {
			return d.assembleAll(ctx, requests, studentID)
		}
	}
	byRequest := assembledSummaries(requests, phases, children, guardians)
	out := make([]*enrollment.DecisionSummary, 0, len(requests))
	for _, request := range requests {
		summary, err := d.validateAssembledSummary(ctx, byRequest[request.ID], phaseErr != nil)
		if err != nil {
			return nil, err
		}
		if studentID <= 0 {
			out = append(out, summary)
			continue
		}
		filterSummaryChildren(summary, studentID)
		if len(summary.Children) > 0 {
			out = append(out, summary)
		}
	}
	return out, nil
}

func (d *Decisions) validateAssembledSummary(ctx context.Context, summary *enrollment.DecisionSummary, reloadPhase bool) (*enrollment.DecisionSummary, error) {
	if reloadPhase || summary.Phase == nil {
		// Missing/unreadable phases are tolerated by assemble; keep its
		// scalar error and warning behavior for these requests.
		return d.assemble(ctx, summary.Request)
	}
	return summary, nil
}

func assembledSummaries(requests []*enrollment.Request, phases []*enrollment.Phase, children []*enrollment.RequestChild, guardians []*enrollment.RequestGuardian) map[int64]*enrollment.DecisionSummary {
	byPhase := make(map[int64]*enrollment.Phase, len(phases))
	for _, phase := range phases {
		byPhase[phase.ID] = phase
	}
	byRequest := make(map[int64]*enrollment.DecisionSummary, len(requests))
	for _, request := range requests {
		byRequest[request.ID] = &enrollment.DecisionSummary{Request: request, Phase: byPhase[request.PhaseID]}
	}
	for _, child := range children {
		if summary := byRequest[child.RequestID]; summary != nil {
			summary.Children = append(summary.Children, child)
		}
	}
	for _, guardian := range guardians {
		if summary := byRequest[guardian.RequestID]; summary != nil {
			summary.Guardians = append(summary.Guardians, guardian)
		}
	}
	return byRequest
}
