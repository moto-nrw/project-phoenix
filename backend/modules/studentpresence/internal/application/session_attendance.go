package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

var (
	errInvalidParticipants   = errors.New("student presence: invalid participant IDs")
	errInvalidAttendanceTime = errors.New("student presence: invalid attendance instant")
)

func (s *Service) ListSessionAttendance(ctx context.Context, participantIDs []int64) (result []ports.SessionAttendance, err error) {
	err = s.run("list_session_attendance", func() (ports.Stats, error) {
		if !validIDs(participantIDs) {
			return ports.Stats{}, errInvalidParticipants
		}
		var stats ports.Stats
		result, stats, err = s.store.ListSessionAttendance(ctx, participantIDs)
		return stats, err
	})
	return result, err
}

func (s *Service) participantWrite(ctx context.Context, operation string, participantIDs []int64, at time.Time, fn func(context.Context) (ports.Stats, error)) (int64, error) {
	var rows int64
	err := s.runWrite(ctx, operation, func(txCtx context.Context) (ports.Stats, error) {
		if !validIDs(participantIDs) {
			return ports.Stats{}, errInvalidParticipants
		}
		if at.IsZero() {
			return ports.Stats{}, errInvalidAttendanceTime
		}
		stats, err := fn(txCtx)
		rows = stats.Rows
		return stats, err
	})
	return rows, err
}

func (s *Service) CheckInParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error) {
	return s.participantWrite(ctx, "check_in_participants", participantIDs, at, func(txCtx context.Context) (ports.Stats, error) {
		return s.store.CheckInParticipants(txCtx, participantIDs, at, false)
	})
}

func (s *Service) CheckInWalkIn(ctx context.Context, participantID int64, at time.Time) (result ports.SessionAttendance, err error) {
	err = s.runWrite(ctx, "check_in_walk_in", func(txCtx context.Context) (ports.Stats, error) {
		if participantID <= 0 {
			return ports.Stats{}, errInvalidParticipants
		}
		if at.IsZero() {
			return ports.Stats{}, errInvalidAttendanceTime
		}
		stats, checkInErr := s.store.CheckInParticipants(txCtx, []int64{participantID}, at, true)
		if checkInErr != nil {
			return stats, checkInErr
		}
		value, found, findStats, findErr := s.store.FindSessionAttendance(txCtx, participantID)
		stats.Queries += findStats.Queries
		stats.StatementDuration += findStats.StatementDuration
		if findErr != nil {
			return stats, findErr
		}
		if !found {
			return stats, ports.ErrSessionAttendanceNotFound
		}
		result = value
		return stats, nil
	})
	return result, err
}

func (s *Service) CheckOutParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error) {
	return s.participantWrite(ctx, "check_out_participants", participantIDs, at, func(txCtx context.Context) (ports.Stats, error) {
		return s.store.CheckOutParticipants(txCtx, participantIDs, at)
	})
}

func (s *Service) CloseOpenParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error) {
	return s.participantWrite(ctx, "close_open_participants", participantIDs, at, func(txCtx context.Context) (ports.Stats, error) {
		return s.store.CloseOpenParticipants(txCtx, participantIDs, at)
	})
}

func (s *Service) ReconcileParticipantInterval(ctx context.Context, participantID int64, previousCheckIn time.Time, previousCheckOut *time.Time, checkIn time.Time, checkOut *time.Time) (updated bool, err error) {
	err = s.runWrite(ctx, "reconcile_participant_interval", func(txCtx context.Context) (ports.Stats, error) {
		if participantID <= 0 || previousCheckIn.IsZero() || checkIn.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid participant interval")
		}
		stats, reconcileErr := s.store.ReconcileParticipantInterval(txCtx, participantID, previousCheckIn, previousCheckOut, checkIn, checkOut)
		updated = stats.Rows > 0
		return stats, reconcileErr
	})
	return updated, err
}

func (s *Service) PatchSessionAttendance(ctx context.Context, participantID int64, patch ports.SessionAttendancePatch) error {
	return s.runWrite(ctx, "patch_session_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if participantID <= 0 || !validSessionAttendancePatch(patch) {
			return ports.Stats{}, errors.New("student presence: invalid session attendance patch")
		}
		return s.store.PatchSessionAttendance(txCtx, participantID, patch, time.Now().UTC())
	})
}

func validSessionAttendancePatch(patch ports.SessionAttendancePatch) bool {
	if patch.Status == nil && patch.Substatus == nil && !patch.SubstatusClear && patch.Note == nil && !patch.NoteClear {
		return false
	}
	if patch.Status != nil && !validSessionAttendanceStatus(*patch.Status) {
		return false
	}
	if patch.Substatus != nil && !validSessionAttendanceSubstatus(*patch.Substatus) {
		return false
	}
	return patch.Note == nil || len(*patch.Note) <= 500
}

func validSessionAttendanceStatus(value string) bool {
	return value == "expected" || value == "present" || value == "absent"
}

func validSessionAttendanceSubstatus(value string) bool {
	switch value {
	case "late", "excused", "sick", "field_trip", "other":
		return true
	default:
		return false
	}
}

func (s *Service) TransitionParticipants(ctx context.Context, participantIDs []int64, from, to string, at time.Time) (int64, error) {
	if !validSessionAttendanceStatus(from) || !validSessionAttendanceStatus(to) {
		return 0, s.run("transition_participants", func() (ports.Stats, error) {
			return ports.Stats{}, errors.New("student presence: invalid attendance transition")
		})
	}
	// A zero instant stamps the rows with the transaction time, so a
	// completion that reads the same time from the database sees the rows
	// as unchanged since then.
	var stamp *time.Time
	if !at.IsZero() {
		stamp = &at
	}
	var rows int64
	err := s.runWrite(ctx, "transition_participants", func(txCtx context.Context) (ports.Stats, error) {
		if !validIDs(participantIDs) {
			return ports.Stats{}, errInvalidParticipants
		}
		stats, err := s.store.TransitionParticipants(txCtx, participantIDs, from, to, stamp)
		rows = stats.Rows
		return stats, err
	})
	return rows, err
}

func (s *Service) MarkParticipantsNotScheduled(ctx context.Context, participantIDs []int64) (int64, error) {
	return s.participantWrite(ctx, "mark_participants_not_scheduled", participantIDs, time.Now(), func(txCtx context.Context) (ports.Stats, error) {
		return s.store.MarkParticipantsNotScheduled(txCtx, participantIDs)
	})
}

func (s *Service) RestoreSessionAttendance(ctx context.Context, rows []ports.SessionAttendanceRestore) error {
	return s.runWrite(ctx, "restore_session_attendance", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		for _, row := range rows {
			if row.ParticipantID <= 0 || !validSessionAttendanceStatus(row.Status) {
				return stats, errors.New("student presence: invalid session attendance restore")
			}
			rowStats, err := s.store.RestoreSessionAttendance(txCtx, row)
			stats.Queries += rowStats.Queries
			stats.Rows += rowStats.Rows
			stats.StatementDuration += rowStats.StatementDuration
			if err != nil {
				return stats, err
			}
		}
		return stats, nil
	})
}

func (s *Service) ReconnectParticipantPickupExceptions(ctx context.Context, links []ports.ParticipantPickupException) error {
	return s.runWrite(ctx, "reconnect_participant_pickup_exceptions", func(txCtx context.Context) (ports.Stats, error) {
		for _, link := range links {
			if link.ParticipantID <= 0 || link.PickupExceptionID <= 0 {
				return ports.Stats{}, errors.New("student presence: invalid participant pickup exception")
			}
		}
		return s.store.ReconnectParticipantPickupExceptions(txCtx, links)
	})
}

func (s *Service) LockSessionAttendance(ctx context.Context, participantIDs []int64) error {
	return s.run("lock_session_attendance", func() (ports.Stats, error) {
		if !validIDs(participantIDs) {
			return ports.Stats{}, errInvalidParticipants
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockSessionAttendance(ctx, participantIDs)
	})
}
