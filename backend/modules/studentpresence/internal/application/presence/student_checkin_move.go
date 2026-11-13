package presence

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
)

const opCheckInStudentsToSession = "CheckInStudentsToSession"

// CheckInStudentsToSessionAuthorized checks children in to the school and
// places them in a running session in one unit of work (#6953): the app path
// that equals a scan at the room's terminal. A child never passes through
// "Unterwegs", and a rejected placement rolls the check-in back.
//
// Children this call checks in have no current place, so the check-in right
// (users:checkin, school-wide since #2329) covers them; the target must
// still be the only session in its room with a running supervision, unless
// the caller supervises it or the school extends the attendance scope, and it
// must have a running supervision for every caller, administrators included:
// the app never starts a supervision and never puts an arriving child into a
// room without an adult. Children already present keep every rule of the
// ordinary move, including the rights over their current place. With
// keepPresent they stay where they are: the check-in mode places only the
// children it checks in, so a card that still showed a child as absent while
// a terminal checked it in elsewhere never moves it.
func (s *service) CheckInStudentsToSessionAuthorized(ctx context.Context, studentIDs []int64, activeGroupID, staffID int64, auth StudentMoveAuthorization, keepPresent bool) (*StudentMoveResult, error) {
	return s.checkInStudentsToSession(ctx, studentIDs, activeGroupID, staffID, auth, moveOptions{keepPresent: keepPresent})
}

// CheckInStudentsToOpenRoomSessionAuthorized is the check-in into the room
// session of a released room. The open-room workflow resolves that session
// and the release; a released room needs no supervision (#3066).
func (s *service) CheckInStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID, staffID int64, auth StudentMoveAuthorization, keepPresent bool) (*StudentMoveResult, error) {
	return s.checkInStudentsToSession(ctx, studentIDs, roomSessionID, staffID, auth, moveOptions{openRoomTarget: true, keepPresent: keepPresent})
}

func (s *service) checkInStudentsToSession(ctx context.Context, studentIDs []int64, activeGroupID, staffID int64, auth StudentMoveAuthorization, opts moveOptions) (*StudentMoveResult, error) {
	if activeGroupID <= 0 || staffID <= 0 {
		return nil, &ActiveError{Op: opCheckInStudentsToSession, Err: ErrInvalidData}
	}
	uniqueIDs, err := normalizeMoveStudentIDs(studentIDs, opCheckInStudentsToSession)
	if err != nil {
		return nil, err
	}
	// The move's unit of work runs the check-in first (moveOptions.checkInBy).
	opts.checkInBy = staffID
	return s.moveStudentsToActiveGroup(ctx, uniqueIDs, activeGroupID, &auth, opts)
}

// checkInStudentsToSessionLocked runs the school check-in first, which locks
// the children in ascending order like every attendance writer, and then the
// move, which re-enters the same child locks before it takes attendance,
// visit and session locks. Both join the caller's transaction.
func (s *service) checkInStudentsToSessionLocked(ctx context.Context, studentIDs []int64, activeGroupID int64, auth StudentMoveAuthorization, opts moveOptions) (*StudentMoveResult, error) {
	batch, err := s.processSchoolCheckinBatch(ctx, studentIDs, opts.checkInBy, SchoolCheckinActionIn, "")
	if err != nil {
		return nil, err
	}

	placeable := make([]int64, 0, len(batch.Results))
	checkedIn := make([]int64, 0, len(batch.Results))
	rejected := make([]StudentMoveSkipped, 0)
	kept := make([]int64, 0)
	opts.arrivals = make(map[int64]struct{}, len(batch.Results))
	for _, item := range batch.Results {
		if !item.OK {
			rejected = append(rejected, StudentMoveSkipped{StudentID: item.StudentID, Reason: studentpresence.StudentMoveSkipNotFound})
			continue
		}
		if item.Changed {
			checkedIn = append(checkedIn, item.StudentID)
			opts.arrivals[item.StudentID] = struct{}{}
		} else if opts.keepPresent {
			kept = append(kept, item.StudentID)
			continue
		}
		placeable = append(placeable, item.StudentID)
	}
	if len(placeable) == 0 {
		result := newStudentMoveResult(&activeGroupID, nil)
		result.Unchanged = append(result.Unchanged, kept...)
		result.Skipped = rejected
		return result, nil
	}

	result, err := s.moveStudentsToActiveGroupLocked(ctx, placeable, activeGroupID, &auth, opts)
	if err != nil {
		return nil, err
	}
	result.CheckedIn = checkedIn
	result.Unchanged = append(result.Unchanged, kept...)
	result.Skipped = append(rejected, result.Skipped...)
	return result, nil
}
