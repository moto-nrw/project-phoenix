package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

// Session lock order of every visit write (#5128). A change of existing
// visits locks each session (active.groups) whose visits it changes FOR
// UPDATE, in ascending id order, before it touches a visit row. A visit
// insert names one session; the check-in locks it first (CreateVisit), and
// the trigger's lock covers other inserts. The visit revision trigger raises
// the same sessions, and a session end locks its session before its visits;
// one order for all of them keeps concurrent writers waiting instead of
// deadlocking.
//
// A mutation statement embeds the lock as the uncorrelated subquery below:
// PostgreSQL evaluates it once, before the first visit row reaches the
// mutation. A transaction that changes several sessions in several
// statements (a scan's checkout and check-in) takes all of them first with
// LockVisitSessions. A session a transaction creates itself needs no
// earlier lock: no other transaction can see it, and its id is higher than
// every session locked before it.
const (
	sessionsLockedFirstPrefix = `(SELECT count(*) FROM (
		SELECT g.id FROM active.groups AS g WHERE g.tenant_id = ? AND (`
	sessionsLockedFirstSuffix = `)
		ORDER BY g.id FOR UPDATE OF g) AS locked_sessions) >= 0`

	// Sessions of the given visits (closed ones included).
	visitSessionsPredicate = `g.id IN (SELECT v.active_group_id FROM active.visits AS v
		WHERE v.tenant_id = g.tenant_id AND v.id IN (?))`
	// Sessions of the given open visits.
	openVisitSessionsPredicate = `g.id IN (SELECT v.active_group_id FROM active.visits AS v
		WHERE v.tenant_id = g.tenant_id AND v.id IN (?) AND v.exit_time IS NULL)`
	// Sessions of the given students' open visits.
	studentSessionsPredicate = `g.id IN (SELECT v.active_group_id FROM active.visits AS v
		WHERE v.tenant_id = g.tenant_id AND v.student_id IN (?) AND v.exit_time IS NULL)`

	lockVisitSessionsFirst        = sessionsLockedFirstPrefix + visitSessionsPredicate + sessionsLockedFirstSuffix
	lockOpenVisitSessionsFirst    = sessionsLockedFirstPrefix + openVisitSessionsPredicate + sessionsLockedFirstSuffix
	lockStudentSessionsFirst      = sessionsLockedFirstPrefix + studentSessionsPredicate + sessionsLockedFirstSuffix
	lockListedSessionsFirst       = sessionsLockedFirstPrefix + `g.id IN (?)` + sessionsLockedFirstSuffix
	lockRevisedVisitSessionsFirst = sessionsLockedFirstPrefix + `g.id = ? OR ` + visitSessionsPredicate + sessionsLockedFirstSuffix
	lockDeviceTransferSessions    = sessionsLockedFirstPrefix + `g.id = ? OR (g.device_id = ?
		AND g.end_time IS NOT NULL AND g.end_time > NOW() - INTERVAL '1 hour')` + sessionsLockedFirstSuffix

	// The sessions a scan of one student into one room can change: the
	// session of the student's open visit and every open session of the
	// room. A room ID of 0 matches no room.
	lockScanSessions = `SELECT g.id FROM active.groups AS g WHERE g.tenant_id = ? AND (` + studentSessionsPredicate + `
		OR (g.room_id = ? AND g.end_time IS NULL)) ORDER BY g.id FOR UPDATE OF g`
	lockStudentSessions = `SELECT g.id FROM active.groups AS g WHERE g.tenant_id = ? AND (` + studentSessionsPredicate + `)
		ORDER BY g.id FOR UPDATE OF g`
)

// LockVisitSessions takes the session locks of a scan up front: the session
// of the student's open visit and every open session of the room, in the
// session lock order. The scan's later checkout and check-in then lock
// nothing new on these sessions, so two scans moving children in opposite
// directions wait for each other instead of deadlocking.
func (s *Store) LockVisitSessions(ctx context.Context, studentID, roomID int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	var locked []int64
	started := time.Now()
	err = db.NewRaw(lockScanSessions, tenantID, bun.List([]int64{studentID}), roomID).Scan(ctx, &locked)
	stats := ports.Stats{Queries: 1, Rows: int64(len(locked)), StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("lock visit sessions: %w", err)
	}
	return stats, nil
}
