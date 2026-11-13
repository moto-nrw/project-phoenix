package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

type publicCaseMessageReader func(context.Context, *ChangeRequest) ([]*enrollment.ChangeRequestMessage, error)

// publicCaseMessages batches only already-terminal threads. Public reply,
// question and review commands cannot append to these cases. Open cases keep
// their live reads, including cases that finish while the list is assembled.
func (s *ChangeRequests) publicCaseMessages(rows []*ChangeRequest) publicCaseMessageReader {
	ids := make([]int64, 0, len(rows))
	terminal := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if terminalPublicCase(row) {
			ids = append(ids, row.ID)
			terminal[row.ID] = true
		}
	}
	var byCase map[int64][]*enrollment.ChangeRequestMessage
	return func(ctx context.Context, row *ChangeRequest) ([]*enrollment.ChangeRequestMessage, error) {
		if !terminal[row.ID] || !terminalPublicCase(row) {
			return s.deps.Requests.ChangeRequestMessages(ctx, []int64{row.ID}, false)
		}
		if byCase == nil {
			messages, err := s.deps.Requests.ChangeRequestMessages(ctx, ids, false)
			if err != nil {
				return nil, err
			}
			byCase = make(map[int64][]*enrollment.ChangeRequestMessage, len(ids))
			for _, message := range messages {
				byCase[message.ChangeRequestID] = append(byCase[message.ChangeRequestID], message)
			}
		}
		return byCase[row.ID], nil
	}
}

func terminalPublicCase(row *ChangeRequest) bool {
	return row.Status == enrollment.ChangeRequestStatusApproved || row.Status == enrollment.ChangeRequestStatusRejected
}

// Keep the scalar validation order and each case's independent tenant frame.
// In particular, do not read even the immutable message set until the fresh
// request and its children have decoded successfully. Phases remain live.
func (s *ChangeRequests) loadPublicCase(ctx context.Context, id int64, readMessages publicCaseMessageReader) (*changeRequestCase, error) {
	row, err := s.readChangeRequest(ctx, id, false)
	if err != nil && !s.deps.Runtime.NotFound(err) {
		return nil, err
	}
	if err != nil || row == nil {
		return nil, enrollment.ErrChangeRequestNotFound
	}
	req, err := s.requestByID(ctx, row.RequestID, false)
	if err != nil {
		return nil, err
	}
	children, err := s.deps.Children.ChildrenForRequest(ctx, req.ID, false)
	if err != nil {
		return nil, fmt.Errorf("change request: list aggregate children: %w", err)
	}
	messages, err := readMessages(ctx, row)
	if err != nil {
		return nil, err
	}
	var phase *enrollment.Phase
	if s.deps.Catalog != nil {
		phase, err = s.intake.intakePhase(ctx, req.PhaseID)
		if err != nil && !s.deps.Runtime.NotFound(err) {
			return nil, err
		}
	}
	return &changeRequestCase{ChangeRequest: row, Request: req, Children: children, Messages: messages, Phase: phase}, nil
}
