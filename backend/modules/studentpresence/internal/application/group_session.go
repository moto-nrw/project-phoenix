package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *Service) LockGroup(ctx context.Context, groupID int64) (result studentpresence.LiveGroup, err error) {
	err = s.run("lock_group", func() (ports.Stats, error) {
		if groupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid active group ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.LockGroup(ctx, groupID)
		return stats, err
	})
	return result, err
}

// EndGroup joins the caller's transaction when there is one and otherwise
// runs in its own, like the visit commands the takeover paths call next to it.
func (s *Service) EndGroup(ctx context.Context, groupID int64, at time.Time) error {
	if groupID <= 0 || at.IsZero() {
		return s.runWrite(ctx, "end_group", func(ctx context.Context) (ports.Stats, error) {
			return ports.Stats{}, errors.New("student presence: invalid group end")
		})
	}
	return s.runWrite(ctx, "end_group", func(txCtx context.Context) (ports.Stats, error) {
		return s.store.EndGroup(txCtx, groupID, at)
	})
}

// EndGroupSessions ends the sessions; their supervisions end on the school's
// calendar day of the close instant.
func (s *Service) EndGroupSessions(ctx context.Context, groupIDs []int64, at time.Time) (result studentpresence.EndedGroupSessions, err error) {
	endDate := calendar.DateFromTime(at)
	err = s.run("end_group_sessions", func() (ports.Stats, error) {
		if !validIDs(groupIDs) || at.IsZero() || endDate.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid bulk group session end")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.EndGroupSessions(ctx, groupIDs, at, endDate)
		return stats, err
	})
	if err != nil {
		return studentpresence.EndedGroupSessions{}, err
	}
	return result, nil
}

// EndGroupSession ends the session; its supervisions end on the school's
// calendar day of the close instant.
func (s *Service) EndGroupSession(ctx context.Context, groupID int64, at time.Time) (studentpresence.EndedGroupSession, error) {
	endDate := calendar.DateFromTime(at)
	var row ports.EndedGroupSession
	err := s.run("end_group_session", func() (ports.Stats, error) {
		if groupID <= 0 || at.IsZero() || endDate.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid group session end")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		var err error
		row, stats, err = s.store.EndGroupSession(ctx, groupID, at, endDate)
		return stats, err
	})
	if err != nil {
		return studentpresence.EndedGroupSession{}, err
	}
	return studentpresence.EndedGroupSession{
		GroupID: row.GroupID, EndedAt: row.EndedAt,
		ClosedVisits: visitValues(row.ClosedVisits), EndedSupervisorIDs: row.EndedSupervisorIDs,
	}, nil
}

func (s *Service) LockSessionStart(ctx context.Context, activityID int64) error {
	return s.run("lock_session_start", func() (ports.Stats, error) {
		if activityID <= 0 {
			return ports.Stats{}, errors.New("lock session start: activity ID must be positive")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockSessionStart(ctx, activityID)
	})
}
