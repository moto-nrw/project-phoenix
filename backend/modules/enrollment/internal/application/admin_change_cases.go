package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// Admin lists run in one tenant transaction. Batch already-terminal cases,
// decoding each request and its children only in list order. Initially open
// cases retain live scalar reads, even if they finish during assembly.
type adminChangeCaseReads struct {
	terminal   map[int64]bool
	requests   map[int64]*enrollment.Request
	children   map[int64][]*enrollment.RequestChild
	messages   map[int64][]*enrollment.ChangeRequestMessage
	phases     map[int64]*enrollment.Phase
	requestIDs []int64
	caseIDs    []int64
}

func (s *ChangeRequests) adminChangeCases(ctx context.Context, rows []*ChangeRequest) ([]*changeRequestCase, error) {
	out := make([]*changeRequestCase, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	reads := adminChangeCaseReads{terminal: make(map[int64]bool, len(rows))}
	seen := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if !terminalPublicCase(row) {
			continue
		}
		reads.terminal[row.ID] = true
		reads.caseIDs = append(reads.caseIDs, row.ID)
		if !seen[row.RequestID] {
			seen[row.RequestID] = true
			reads.requestIDs = append(reads.requestIDs, row.RequestID)
		}
	}
	for _, row := range rows {
		agg, err := s.adminChangeCase(ctx, row.ID, &reads)
		if err != nil {
			return nil, err
		}
		out = append(out, agg)
	}
	return out, nil
}

func (s *ChangeRequests) adminChangeCase(ctx context.Context, id int64, reads *adminChangeCaseReads) (*changeRequestCase, error) {
	// Keep the live case read: a request may have been decided or removed
	// since ListChangeRequests selected its IDs.
	row, err := s.readChangeRequest(ctx, id, false)
	if err != nil && !s.deps.Runtime.NotFound(err) {
		return nil, err
	}
	if err != nil || row == nil {
		return nil, enrollment.ErrChangeRequestNotFound
	}
	if !reads.terminal[id] || !terminalPublicCase(row) {
		return s.aggregateFromRow(ctx, row, true)
	}
	req, err := s.adminCaseRequest(ctx, row.RequestID, reads)
	if err != nil {
		return nil, err
	}
	children, err := s.adminCaseChildren(ctx, req.ID, reads)
	if err != nil {
		return nil, fmt.Errorf("change request: list aggregate children: %w", err)
	}
	messages, err := s.adminCaseMessages(ctx, row.ID, reads)
	if err != nil {
		return nil, err
	}
	phase, err := s.adminCasePhase(ctx, req.PhaseID, reads)
	if err != nil && !s.deps.Runtime.NotFound(err) {
		return nil, err
	}
	return &changeRequestCase{ChangeRequest: row, Request: req, Children: children, Messages: messages, Phase: phase}, nil
}

func (s *ChangeRequests) adminCaseChildren(ctx context.Context, requestID int64, reads *adminChangeCaseReads) ([]*enrollment.RequestChild, error) {
	if reads.children == nil {
		children, err := s.deps.Children.ChildrenForRequests(ctx, reads.requestIDs)
		if err != nil {
			return nil, err
		}
		reads.children = make(map[int64][]*enrollment.RequestChild, len(reads.requestIDs))
		for _, child := range children {
			reads.children[child.RequestID] = append(reads.children[child.RequestID], child)
		}
	}
	if reads.requests[requestID] == nil {
		return s.deps.Children.ChildrenForRequest(ctx, requestID, false)
	}
	return reads.children[requestID], nil
}

func (s *ChangeRequests) adminCasePhase(ctx context.Context, phaseID int64, reads *adminChangeCaseReads) (*enrollment.Phase, error) {
	if s.deps.Catalog == nil {
		return nil, nil
	}
	bulk, ok := s.deps.Catalog.(interface {
		PhasesByID(context.Context, []int64) ([]*enrollment.Phase, error)
	})
	if !ok {
		return s.intake.intakePhase(ctx, phaseID)
	}
	if reads.phases == nil {
		ids := make(map[int64]struct{}, len(reads.requests))
		for _, req := range reads.requests {
			ids[req.PhaseID] = struct{}{}
		}
		ids[phaseID] = struct{}{}
		phases, err := bulk.PhasesByID(ctx, int64SetKeys(ids))
		if err != nil {
			return nil, err
		}
		reads.phases = make(map[int64]*enrollment.Phase, len(phases))
		for _, phase := range phases {
			reads.phases[phase.ID] = phase
		}
	}
	if phase := reads.phases[phaseID]; phase != nil {
		return phase, nil
	}
	return s.intake.intakePhase(ctx, phaseID)
}

func (s *ChangeRequests) adminCaseRequest(ctx context.Context, requestID int64, reads *adminChangeCaseReads) (*enrollment.Request, error) {
	if reads.requests == nil {
		requests, err := s.deps.Requests.RequestsByID(ctx, reads.requestIDs)
		if err != nil {
			return nil, err
		}
		reads.requests = make(map[int64]*enrollment.Request, len(requests))
		for _, request := range requests {
			reads.requests[request.ID] = request
		}
	}
	if raw := reads.requests[requestID]; raw != nil {
		return raw, nil
	}
	return s.requestByID(ctx, requestID, false)
}

func (s *ChangeRequests) adminCaseMessages(ctx context.Context, caseID int64, reads *adminChangeCaseReads) ([]*enrollment.ChangeRequestMessage, error) {
	if reads.messages == nil {
		messages, err := s.deps.Requests.ChangeRequestMessages(ctx, reads.caseIDs, true)
		if err != nil {
			return nil, err
		}
		reads.messages = make(map[int64][]*enrollment.ChangeRequestMessage, len(reads.caseIDs))
		for _, message := range messages {
			reads.messages[message.ChangeRequestID] = append(reads.messages[message.ChangeRequestID], message)
		}
	}
	return reads.messages[caseID], nil
}
