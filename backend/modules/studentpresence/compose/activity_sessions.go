package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func activitySessionToPublic(row ports.ActivitySession) studentpresence.ActivitySession {
	return studentpresence.ActivitySession{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		InstanceID: row.InstanceID, Status: row.Status, ActiveGroupID: row.ActiveGroupID, StartedBy: row.StartedBy,
		StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, CompletedBy: row.CompletedBy, ReopenUntil: row.ReopenUntil,
		CompletionSnapshot: row.CompletionSnapshot,
	}
}

func activitySessionError(err error) error {
	switch {
	case errors.Is(err, ports.ErrActivitySessionNotFound):
		return studentpresence.ErrActivitySessionNotFound
	case errors.Is(err, ports.ErrActivitySessionExists):
		return studentpresence.ErrActivitySessionExists
	}
	return err
}

func (e engine) FindActivitySession(ctx context.Context, instanceID int64) (studentpresence.ActivitySession, error) {
	row, err := e.Service.FindActivitySession(ctx, instanceID)
	if err != nil {
		return studentpresence.ActivitySession{}, activitySessionError(err)
	}
	return activitySessionToPublic(row), nil
}

func (e engine) ListActivitySessions(ctx context.Context, filter studentpresence.ActivitySessionFilter) ([]studentpresence.ActivitySession, error) {
	rows, err := e.Service.ListActivitySessions(ctx, ports.ActivitySessionFilter(filter))
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.ActivitySession, 0, len(rows))
	for _, row := range rows {
		result = append(result, activitySessionToPublic(row))
	}
	return result, nil
}

func (e engine) StartActivitySession(ctx context.Context, start studentpresence.ActivitySessionStart) (studentpresence.ActivitySession, error) {
	row, err := e.Service.StartActivitySession(ctx, ports.ActivitySessionStart(start))
	if err != nil {
		return studentpresence.ActivitySession{}, activitySessionError(err)
	}
	return activitySessionToPublic(row), nil
}

func (e engine) CompleteActivitySession(ctx context.Context, completion studentpresence.ActivitySessionCompletion) (studentpresence.ActivitySession, error) {
	row, err := e.Service.CompleteActivitySession(ctx, ports.ActivitySessionCompletion(completion))
	if err != nil {
		return studentpresence.ActivitySession{}, activitySessionError(err)
	}
	return activitySessionToPublic(row), nil
}

func (e engine) RecordActivitySessionCompleted(ctx context.Context, instanceID int64, at time.Time) error {
	return e.Service.RecordActivitySessionCompleted(ctx, instanceID, at)
}

func (e engine) ReopenActivitySession(ctx context.Context, instanceID, activeGroupID int64) (studentpresence.ActivitySession, error) {
	row, err := e.Service.ReopenActivitySession(ctx, instanceID, activeGroupID)
	if err != nil {
		return studentpresence.ActivitySession{}, activitySessionError(err)
	}
	return activitySessionToPublic(row), nil
}

func (e engine) CompleteActivitySessionsByGroups(ctx context.Context, activeGroupIDs []int64, at time.Time) (int64, error) {
	return e.Service.CompleteActivitySessionsByGroups(ctx, activeGroupIDs, at)
}

func (e engine) DiscardActivitySession(ctx context.Context, instanceID int64) error {
	return e.Service.DiscardActivitySession(ctx, instanceID)
}

func (e engine) SessionExecution(ctx context.Context, filter studentpresence.SessionExecutionFilter) (studentpresence.SessionExecution, error) {
	result, err := e.Service.SessionExecution(ctx, ports.SessionExecutionFilter(filter))
	return studentpresence.SessionExecution(result), err
}
