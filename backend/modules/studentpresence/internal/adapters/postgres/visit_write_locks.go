package postgres

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

type visitWritePlan struct {
	scope             studentpresence.VisitWriteScope
	sources, sessions []int64
	visits            []studentpresence.Visit
	rooms             []visitWriteRoom
}

type visitWriteRoom struct{ ID, RoomID int64 }

// PrepareVisitWrites uses a second READ COMMITTED snapshot after session
// locks to detect additions. A changed cohort is a conflict, never a reason
// to acquire another visit lock while holding a session or room row.
func (s *Store) PrepareVisitWrites(ctx context.Context, scope studentpresence.VisitWriteScope) (ports.Stats, error) {
	_, stats, err := s.prepareVisitWritePlan(ctx, scope)
	return stats, err
}

func (s *Store) prepareVisitWritePlan(ctx context.Context, scope studentpresence.VisitWriteScope) (plan visitWritePlan, stats ports.Stats, err error) {
	db, school, err := s.database(ctx)
	if err != nil {
		return plan, stats, err
	}
	started := time.Now()
	defer func() { stats.StatementDuration = time.Since(started) }()
	plan, err = prepareVisitWritePlan(ctx, db, school, scope, &stats)
	if err != nil {
		return plan, stats, err
	}
	if err = s.lockVisitWriteSessions(ctx, db, school, plan, &stats); err != nil {
		return plan, stats, err
	}
	err = validateVisitWritePlan(ctx, db, school, plan, &stats)
	stats.Rows = int64(len(plan.visits) + len(plan.sessions))
	return plan, stats, err
}

func visitWriteIDs(visits []studentpresence.Visit) []int64 {
	ids := make([]int64, len(visits))
	for i, visit := range visits {
		ids[i] = visit.ID
	}
	return ids
}

func prepareVisitWritePlan(ctx context.Context, db bun.IDB, school int64, scope studentpresence.VisitWriteScope, stats *ports.Stats) (visitWritePlan, error) {
	plan := visitWritePlan{scope: scope}
	var err error
	plan.sources, err = writeSourceGroups(ctx, db, school, scope)
	stats.Queries++
	if err != nil {
		return plan, err
	}
	roomSessions, err := writeRoomSessions(ctx, db, school, scope.RoomID)
	if scope.RoomID > 0 {
		stats.Queries++
	}
	if err != nil {
		return plan, err
	}
	plan.visits, err = prepareWriteVisits(ctx, db, school, scope, plan.sources, stats)
	if err != nil {
		return plan, err
	}
	plan.sessions = append(slices.Clone(plan.sources), scope.SessionIDs...)
	plan.sessions = append(plan.sessions, roomSessions...)
	for _, visit := range plan.visits {
		plan.sessions = append(plan.sessions, visit.ActiveGroupID)
	}
	plan.sessions = sortedWriteIDs(plan.sessions)
	plan.rooms, err = writeSessionRooms(ctx, db, school, plan.sessions)
	if len(plan.sessions) > 0 {
		stats.Queries++
	}
	return plan, err
}

func (s *Store) lockVisitWriteSessions(ctx context.Context, db bun.IDB, school int64, plan visitWritePlan, stats *ports.Stats) error {
	rooms := []int64{plan.scope.RoomID}
	for _, room := range plan.rooms {
		rooms = append(rooms, room.RoomID)
	}
	for _, room := range sortedWriteIDs(rooms) {
		if _, err := s.LockRoomSessionWrites(ctx, room); err != nil {
			return err
		}
		stats.Queries++
	}
	if len(plan.sessions) == 0 {
		return nil
	}
	var locked []int64
	stats.Queries++
	return db.NewRaw("SELECT id FROM active.groups WHERE tenant_id = ? AND id IN (?) ORDER BY id FOR UPDATE", school, bun.List(plan.sessions)).Scan(ctx, &locked)
}

func validateVisitWritePlan(ctx context.Context, db bun.IDB, school int64, plan visitWritePlan, stats *ports.Stats) error {
	sources, err := writeSourceGroups(ctx, db, school, plan.scope)
	stats.Queries++
	if err != nil {
		return err
	}
	if !writeIDsContain(plan.sources, sources) {
		return studentpresence.ErrVisitWriteConflict
	}
	roomSessions, err := writeRoomSessions(ctx, db, school, plan.scope.RoomID)
	if plan.scope.RoomID > 0 {
		stats.Queries++
	}
	if err != nil {
		return err
	}
	if !writeIDsContain(plan.sessions, roomSessions) {
		return studentpresence.ErrVisitWriteConflict
	}
	rooms, err := writeSessionRooms(ctx, db, school, plan.sessions)
	if len(plan.sessions) > 0 {
		stats.Queries++
	}
	if err != nil {
		return err
	}
	if !slices.Equal(plan.rooms, rooms) {
		return studentpresence.ErrVisitWriteConflict
	}
	visits, err := writeVisits(ctx, db, school, plan.scope, plan.sources, false)
	stats.Queries++
	if err != nil {
		return err
	}
	locked := make(map[int64]studentpresence.Visit, len(plan.visits))
	for _, v := range plan.visits {
		locked[v.ID] = v
	}
	for _, v := range visits {
		old, ok := locked[v.ID]
		if !ok || old.ActiveGroupID != v.ActiveGroupID {
			return studentpresence.ErrVisitWriteConflict
		}
	}
	return nil
}

func writeSessionRooms(ctx context.Context, db bun.IDB, school int64, sessions []int64) ([]visitWriteRoom, error) {
	if len(sessions) == 0 {
		return nil, nil
	}
	var rooms []visitWriteRoom
	err := db.NewRaw("SELECT id, room_id FROM active.groups WHERE tenant_id = ? AND id IN (?) ORDER BY id", school, bun.List(sessions)).Scan(ctx, &rooms)
	return rooms, err
}

func writeSourceGroups(ctx context.Context, db bun.IDB, school int64, scope studentpresence.VisitWriteScope) ([]int64, error) {
	roomID := int64(0)
	if scope.RoomVisits {
		roomID = scope.RoomID
	}
	var ids []int64
	err := db.NewRaw(`SELECT id FROM active.groups WHERE tenant_id = ? AND (
  id IN (?) OR (device_id = ? AND (end_time IS NULL OR end_time > NOW() - INTERVAL '1 hour'))
  OR (room_id = ? AND end_time IS NULL)) ORDER BY id`,
		school, bun.List(scope.GroupIDs), scope.DeviceID, roomID).Scan(ctx, &ids)
	return ids, err
}

func writeVisits(ctx context.Context, db bun.IDB, school int64, scope studentpresence.VisitWriteScope, groups []int64, lock bool) ([]studentpresence.Visit, error) {
	rows := []*visitRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`active.visits AS "visit"`).
		Where("visit.tenant_id = ?", school).
		Where("(visit.id IN (?) OR (visit.exit_time IS NULL AND (visit.student_id IN (?) OR visit.active_group_id IN (?))))", bun.List(scope.VisitIDs), bun.List(scope.StudentIDs), bun.List(groups)).
		OrderExpr("visit.id ASC")
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("prepare visit writes: %w", err)
	}
	values := make([]studentpresence.Visit, len(rows))
	for i, row := range rows {
		values[i] = *row.record()
	}
	return values, nil
}

func sortedWriteIDs(ids []int64) []int64 {
	ids = slices.DeleteFunc(ids, func(id int64) bool { return id <= 0 })
	slices.Sort(ids)
	return slices.Compact(ids)
}

func writeRoomSessions(ctx context.Context, db bun.IDB, school, roomID int64) ([]int64, error) {
	if roomID == 0 {
		return nil, nil
	}
	var ids []int64
	err := db.NewRaw("SELECT id FROM active.groups WHERE tenant_id = ? AND room_id = ? AND end_time IS NULL ORDER BY id", school, roomID).Scan(ctx, &ids)
	return ids, err
}

func writeIDsContain(locked, current []int64) bool {
	for _, id := range current {
		if _, found := slices.BinarySearch(locked, id); !found {
			return false
		}
	}
	return true
}

// A scan can replace a selected visit while our first statement waits. Read
// a fresh snapshot before any advisory/session lock and include those new
// intervals only in ascending ID order. An older invisible row is a conflict,
// not permission to invert the order. Later additions are refused by validation.
func prepareWriteVisits(ctx context.Context, db bun.IDB, school int64, scope studentpresence.VisitWriteScope, sources []int64, stats *ports.Stats) ([]studentpresence.Visit, error) {
	visits, err := writeVisits(ctx, db, school, scope, sources, true)
	stats.Queries++
	if err != nil {
		return nil, err
	}
	current, err := writeVisits(ctx, db, school, scope, sources, false)
	stats.Queries++
	if err != nil {
		return nil, err
	}
	locked := make(map[int64]bool, len(visits))
	var last int64
	for _, v := range visits {
		locked[v.ID] = true
		last = max(last, v.ID)
	}
	var missing []int64
	for _, v := range current {
		if locked[v.ID] {
			continue
		}
		if v.ID <= last {
			return nil, studentpresence.ErrVisitWriteConflict
		}
		missing = append(missing, v.ID)
	}
	if len(missing) == 0 {
		return visits, nil
	}
	extra, err := writeVisits(ctx, db, school, studentpresence.VisitWriteScope{VisitIDs: missing}, nil, true)
	stats.Queries++
	return append(visits, extra...), err
}
