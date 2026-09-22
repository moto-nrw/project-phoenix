package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) FindActivitySession(ctx context.Context, instanceID int64) (result ports.ActivitySession, err error) {
	err = s.run("find_activity_session", func() (ports.Stats, error) {
		if instanceID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid instance ID")
		}
		value, found, stats, findErr := s.store.FindActivitySession(ctx, instanceID)
		if findErr != nil {
			return stats, findErr
		}
		if !found {
			return stats, ports.ErrActivitySessionNotFound
		}
		result = value
		return stats, nil
	})
	return result, err
}

func (s *Service) ListActivitySessions(ctx context.Context, filter ports.ActivitySessionFilter) (result []ports.ActivitySession, err error) {
	err = s.run("list_activity_sessions", func() (ports.Stats, error) {
		if !validIDs(filter.InstanceIDs) || !validIDs(filter.ActiveGroupIDs) || !validActivitySessionStatus(filter.Status, true) {
			return ports.Stats{}, errors.New("student presence: invalid activity session filter")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListActivitySessions(ctx, filter)
		return stats, err
	})
	return result, err
}

// SessionExecution answers, in one statement, which of the instances ended
// and which of the participants are marked as not booked. Timetable asks it
// before offering a block to a later pickup.
func (s *Service) SessionExecution(ctx context.Context, filter ports.SessionExecutionFilter) (result ports.SessionExecution, err error) {
	err = s.run("session_execution", func() (ports.Stats, error) {
		if !validIDs(filter.InstanceIDs) || !validIDs(filter.ParticipantIDs) {
			return ports.Stats{}, errors.New("student presence: invalid session execution filter")
		}
		if len(filter.InstanceIDs) == 0 && len(filter.ParticipantIDs) == 0 {
			return ports.Stats{}, nil
		}
		var stats ports.Stats
		result, stats, err = s.store.SessionExecution(ctx, filter)
		return stats, err
	})
	return result, err
}

func validActivitySessionStatus(status string, optional bool) bool {
	switch status {
	case ports.ActivitySessionActive, ports.ActivitySessionCompleted:
		return true
	case "":
		return optional
	default:
		return false
	}
}

// StartActivitySession joins the caller's transaction: the start of a block
// writes the live group, its supervisors and the session together.
func (s *Service) StartActivitySession(ctx context.Context, start ports.ActivitySessionStart) (result ports.ActivitySession, err error) {
	err = s.runWrite(ctx, "start_activity_session", func(txCtx context.Context) (ports.Stats, error) {
		if start.InstanceID <= 0 || start.ActiveGroupID <= 0 || start.StartedAt.IsZero() || (start.StartedBy != nil && *start.StartedBy <= 0) {
			return ports.Stats{}, errors.New("student presence: invalid activity session start")
		}
		var stats ports.Stats
		result, stats, err = s.store.StartActivitySession(txCtx, start)
		if base.IsUniqueViolation(err) {
			// One session per instance: a second start races the first.
			return stats, ports.ErrActivitySessionExists
		}
		return stats, err
	})
	return result, err
}

func (s *Service) CompleteActivitySession(ctx context.Context, completion ports.ActivitySessionCompletion) (result ports.ActivitySession, err error) {
	err = s.runWrite(ctx, "complete_activity_session", func(txCtx context.Context) (ports.Stats, error) {
		if completion.InstanceID <= 0 || completion.CompletedAt.IsZero() || (completion.CompletedBy != nil && *completion.CompletedBy <= 0) {
			return ports.Stats{}, errors.New("student presence: invalid activity session completion")
		}
		value, found, stats, completeErr := s.store.CompleteActivitySession(txCtx, completion)
		if completeErr != nil {
			return stats, completeErr
		}
		if !found {
			return stats, ports.ErrActivitySessionNotFound
		}
		result = value
		return stats, nil
	})
	return result, err
}

func (s *Service) RecordActivitySessionCompleted(ctx context.Context, instanceID int64, at time.Time) error {
	return s.runWrite(ctx, "record_activity_session_completed", func(txCtx context.Context) (ports.Stats, error) {
		if instanceID <= 0 || at.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid activity session completion")
		}
		return s.store.RecordActivitySessionCompleted(txCtx, instanceID, at)
	})
}

func (s *Service) ReopenActivitySession(ctx context.Context, instanceID, activeGroupID int64) (result ports.ActivitySession, err error) {
	err = s.run("reopen_activity_session", func() (ports.Stats, error) {
		if instanceID <= 0 || activeGroupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid activity session reopen")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		value, found, stats, reopenErr := s.store.ReopenActivitySession(ctx, instanceID, activeGroupID)
		if reopenErr != nil {
			return stats, reopenErr
		}
		if !found {
			return stats, ports.ErrActivitySessionNotFound
		}
		result = value
		return stats, nil
	})
	return result, err
}

func (s *Service) CompleteActivitySessionsByGroups(ctx context.Context, activeGroupIDs []int64, at time.Time) (result int64, err error) {
	err = s.runWrite(ctx, "complete_activity_sessions_by_groups", func(txCtx context.Context) (ports.Stats, error) {
		if !validIDs(activeGroupIDs) || at.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid bulk activity session completion")
		}
		stats, completeErr := s.store.CompleteActivitySessionsByGroups(txCtx, activeGroupIDs, at)
		result = stats.Rows
		return stats, completeErr
	})
	return result, err
}

func (s *Service) DiscardActivitySession(ctx context.Context, instanceID int64) error {
	return s.runWrite(ctx, "discard_activity_session", func(txCtx context.Context) (ports.Stats, error) {
		if instanceID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid instance ID")
		}
		return s.store.DiscardActivitySession(txCtx, instanceID)
	})
}
