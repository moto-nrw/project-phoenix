// backend/database/repositories/active/visit.go
package active

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableActiveVisits            = "active.visits"
	tableExprActiveVisitsAsVisit = `active.visits AS "visit"`
)

// VisitRepository implements active.VisitRepository interface
type VisitRepository struct {
	*base.Repository[*active.Visit]
	db       *bun.DB
	students StudentDirectory
	rooms    RoomDirectory
}

// BindRoomDirectory installs the Facilities directory the visit reads
// resolve room names through (#2665).
func (r *VisitRepository) BindRoomDirectory(rooms RoomDirectory) {
	r.rooms = rooms
}

// BindStudentDirectory installs the People Directory the open-visit display
// resolves student columns through (#2662).
func (r *VisitRepository) BindStudentDirectory(students StudentDirectory) {
	r.students = students
}

// NewVisitRepository creates a new VisitRepository
func NewVisitRepository(db *bun.DB) active.VisitRepository {
	repo := base.NewRepository[*active.Visit](db, tableActiveVisits, "Visit")
	repo.TenantScoped = true
	return &VisitRepository{
		Repository: repo,
		db:         db,
	}
}

// FindActiveByStudentID finds all active visits for a specific student
func (r *VisitRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	var visits []*active.Visit
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id = ? AND "visit".exit_time IS NULL`, studentID)

	query = base.WithTenantFilter(ctx, query, "visit")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by student ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return visits, nil
}

// FindByActiveGroupID finds all visits for a specific active group
func (r *VisitRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	var visits []*active.Visit
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".active_group_id = ?`, activeGroupID)

	query = base.WithTenantFilter(ctx, query, "visit")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return visits, nil
}

// FindByActiveGroupIDs finds all visits belonging to any of the given active
// groups in a single query. It is the bulk counterpart of FindByActiveGroupID:
// callers that resolve many groups (a whole day of slots) get one query instead
// of one per group. An empty input returns no visits without hitting the DB.
func (r *VisitRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*active.Visit, error) {
	if len(activeGroupIDs) == 0 {
		return nil, nil
	}
	var visits []*active.Visit
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".active_group_id IN (?)`, bun.List(activeGroupIDs))

	query = base.WithTenantFilter(ctx, query, "visit")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	return visits, nil
}

// FindByTimeRange finds all visits active during a specific time range
func (r *VisitRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	var visits []*active.Visit
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".entry_time <= ? AND ("visit".exit_time IS NULL OR "visit".exit_time >= ?)`, end, start)

	query = base.WithTenantFilter(ctx, query, "visit")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by time range",
			Err: base.TranslateNotFound(err),
		}
	}

	return visits, nil
}

// visitWithGroupRoom is a result struct for FindByStudentAndTimeRange that
// captures active-group + room columns via explicit JOINs. BUN's Relation()
// does not resolve `schema:active` / `schema:facilities` tags for relation
// sub-queries, so we must load them manually (see line 245 of this file).
type visitWithGroupRoom struct {
	active.Visit
	GroupRoomID int64 `bun:"group__room_id"`
}

// FindByStudentAndTimeRange finds all visits (active or ended) for a specific
// student whose entry_time falls within [start, end], ordered by entry_time desc.
// Eagerly loads the active group's room_id and room name via explicit JOINs
// (not BUN Relation — see comment above).
func (r *VisitRepository) FindByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*active.Visit, error) {
	var results []visitWithGroupRoom

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&results).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		ColumnExpr(`"visit".*`).
		ColumnExpr(`"group".room_id AS "group__room_id"`).
		Join(`LEFT JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".entry_time >= ?`, start).
		Where(`"visit".entry_time <= ?`, end).
		OrderExpr(`"visit".entry_time ASC`)

	query = base.WithTenantFilter(ctx, query, "visit")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and time range",
			Err: base.TranslateNotFound(err),
		}
	}

	// Materialize into []*active.Visit with ActiveGroup + Room populated.
	// When the LEFT JOIN finds no group, GroupRoomID is 0 — leave ActiveGroup nil.
	// The room owner resolves the names; a room it cannot see yields "",
	// mirroring the former LEFT JOIN (#2665).
	roomIDs := make([]int64, 0, len(results))
	for i := range results {
		roomIDs = append(roomIDs, results[i].GroupRoomID)
	}
	rooms, err := roomsByID(ctx, r.rooms, roomIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by student and time range", Err: err}
	}
	visits := make([]*active.Visit, 0, len(results))
	for i := range results {
		v := results[i].Visit
		if results[i].GroupRoomID != 0 {
			v.ActiveGroup = &active.Group{
				RoomID: results[i].GroupRoomID,
				Room: &facilities.Room{
					Name: rooms[results[i].GroupRoomID].Name,
				},
			}
		}
		visits = append(visits, &v)
	}
	return visits, nil
}

// EndVisit marks a visit as ended at the current time
func (r *VisitRepository) EndVisit(ctx context.Context, id int64) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Table(tableActiveVisits).
		Set(`exit_time = ?`, time.Now()).
		Where(`id = ? AND exit_time IS NULL`, id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "end visit",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "end visit")
}

// EndVisitsByIDs ends the given visits at one shared instant in a single
// state-checked UPDATE and returns the rows actually ended. Concurrently
// ended visits fail the WHERE and are absorbed (missing from the result),
// mirroring EndVisit's already-ended tolerance without a per-visit re-read.
func (r *VisitRepository) EndVisitsByIDs(ctx context.Context, ids []int64, at time.Time) ([]*active.Visit, error) {
	if len(ids) == 0 {
		return []*active.Visit{}, nil
	}

	// Model + ModelTableExpr, never Model + Table: adding Table on top of a
	// typed Model makes bun reference active.visits twice and the WHERE's
	// "id" becomes ambiguous.
	var ended []*active.Visit
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.Visit)(nil)).
		ModelTableExpr(tableActiveVisits+` AS "visit"`).
		Set(`exit_time = ?`, at).
		Where(`"visit".id IN (?) AND "visit".exit_time IS NULL`, bun.List(ids)).
		Returning("*")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"visit".tenant_id = ?`, tenantID)
	}

	if err := query.Scan(ctx, &ended); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*active.Visit{}, nil
		}
		return nil, &modelBase.DatabaseError{Op: "end visits by ids", Err: base.TranslateNotFound(err)}
	}
	return ended, nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *VisitRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.Visit, error) {
	return r.ListWithOptions(ctx, options)
}

// TransferVisitsFromRecentSessions transfers active visits from recent ended sessions on the same device to a new session
func (r *VisitRepository) TransferVisitsFromRecentSessions(ctx context.Context, newActiveGroupID, deviceID int64) (int, error) {
	// Transfer active visits from recent sessions (ended within last hour) on the same device
	query := base.GetDB(ctx, r.db).NewUpdate().
		Table(tableActiveVisits).
		Set("active_group_id = ?", newActiveGroupID).
		Where(`active_group_id IN (
			SELECT id FROM active.groups
			WHERE device_id = ?
			AND end_time IS NOT NULL
			AND end_time > NOW() - INTERVAL '1 hour'
		) AND exit_time IS NULL`, deviceID)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "transfer visits from recent sessions",
			Err: base.TranslateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get affected rows from visit transfer",
			Err: base.TranslateNotFound(err),
		}
	}

	return int(rowsAffected), nil
}

// TransferActiveVisitsBetweenGroups transfers only currently open visits from
// one active group to another. The exit_time predicate makes the transfer safe
// against concurrent checkout/timeout flows.
func (r *VisitRepository) TransferActiveVisitsBetweenGroups(ctx context.Context, oldActiveGroupID, newActiveGroupID int64) (int, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Table(tableActiveVisits).
		Set("active_group_id = ?", newActiveGroupID).
		Set("updated_at = ?", time.Now()).
		Where("active_group_id = ?", oldActiveGroupID).
		Where("exit_time IS NULL")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "transfer active visits between groups",
			Err: base.TranslateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get affected rows from active visit transfer",
			Err: base.TranslateNotFound(err),
		}
	}

	return int(rowsAffected), nil
}

// DeleteExpiredVisits deletes visits older than retention days for a specific student
func (r *VisitRepository) DeleteExpiredVisits(ctx context.Context, studentID int64, retentionDays int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*active.Visit)(nil)).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".created_at < ?`, cutoffDate).
		Where(`"visit".exit_time IS NOT NULL`) // Only delete completed visits

	query = base.WithTenantFilter(ctx, query, "visit")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete expired visits",
			Err: base.TranslateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: base.TranslateNotFound(err),
		}
	}

	return rowsAffected, nil
}

// GetVisitRetentionStats gets statistics about visits that are candidates for deletion
func (r *VisitRepository) GetVisitRetentionStats(ctx context.Context) (map[int64]int, error) {
	type studentVisitCount struct {
		StudentID  int64 `bun:"student_id"`
		VisitCount int   `bun:"visit_count"`
	}

	var results []studentVisitCount
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("active.visits AS v").
		ColumnExpr("v.student_id").
		ColumnExpr("COUNT(*) AS visit_count").
		Join("INNER JOIN users.privacy_consents AS pc ON pc.student_id = v.student_id").
		Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => pc.data_retention_days)").
		Group("v.student_id")

	query = base.WithTenantFilter(ctx, query, "v")

	err := query.Scan(ctx, &results)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get visit retention stats",
			Err: base.TranslateNotFound(err),
		}
	}

	// Convert to map
	stats := make(map[int64]int)
	for _, result := range results {
		stats[result.StudentID] = result.VisitCount
	}

	return stats, nil
}

// CountExpiredVisits counts visits that are older than retention period for all students
func (r *VisitRepository) CountExpiredVisits(ctx context.Context) (int64, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("active.visits AS v").
		Join("INNER JOIN users.privacy_consents AS pc ON pc.student_id = v.student_id").
		Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => pc.data_retention_days)")

	query = base.WithTenantFilter(ctx, query, "v")

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count expired visits",
			Err: base.TranslateNotFound(err),
		}
	}

	return int64(count), nil
}

// OldestExpiredVisitDate returns the created_at of the oldest visit past its
// per-student retention window (same predicate as CountExpiredVisits), or nil
// when no visit is expired. Custom method: the privacy-consent join is not
// expressible via the generic helpers. Feeds the GDPR retention statistics
// and cleanup preview.
func (r *VisitRepository) OldestExpiredVisitDate(ctx context.Context) (*time.Time, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("active.visits AS v").
		ColumnExpr("MIN(v.created_at)").
		Join("INNER JOIN users.privacy_consents AS pc ON pc.student_id = v.student_id").
		Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => pc.data_retention_days)")

	query = base.WithTenantFilter(ctx, query, "v")

	var oldest *time.Time
	if err := query.Scan(ctx, &oldest); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "oldest expired visit date",
			Err: base.TranslateNotFound(err),
		}
	}

	return oldest, nil
}

// ExpiredVisitMonthlyCounts groups expired visits (same predicate as
// CountExpiredVisits) by calendar month of created_at, keyed YYYY-MM.
// Custom method: the privacy-consent join is not expressible via the
// generic helpers. Feeds the GDPR retention statistics.
func (r *VisitRepository) ExpiredVisitMonthlyCounts(ctx context.Context) (map[string]int64, error) {
	type monthlyCount struct {
		Month string `bun:"month"`
		Count int64  `bun:"count"`
	}

	var results []monthlyCount
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("active.visits AS v").
		ColumnExpr("TO_CHAR(v.created_at, 'YYYY-MM') AS month").
		ColumnExpr("COUNT(*) AS count").
		Join("INNER JOIN users.privacy_consents AS pc ON pc.student_id = v.student_id").
		Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => pc.data_retention_days)").
		GroupExpr("TO_CHAR(v.created_at, 'YYYY-MM')")

	query = base.WithTenantFilter(ctx, query, "v")

	if err := query.Scan(ctx, &results); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "expired visit monthly counts",
			Err: base.TranslateNotFound(err),
		}
	}

	counts := make(map[string]int64, len(results))
	for _, row := range results {
		counts[row.Month] = row.Count
	}

	return counts, nil
}

// GetCurrentByStudentID finds the current active visit for a student
func (r *VisitRepository) GetCurrentByStudentID(ctx context.Context, studentID int64) (*active.Visit, error) {
	visit := new(active.Visit)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(visit).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id = ? AND "visit".exit_time IS NULL`, studentID).
		Order(`entry_time DESC`).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "visit")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by student ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return visit, nil
}

// GetCurrentByStudentIDWithRoom finds the current active visit for a student and loads the active group and room.
func (r *VisitRepository) GetCurrentByStudentIDWithRoom(ctx context.Context, studentID int64) (*active.Visit, error) {
	type currentVisitRow struct {
		VisitID             int64         `bun:"visit_id"`
		VisitStudentID      int64         `bun:"visit_student_id"`
		VisitActiveGroupID  int64         `bun:"visit_active_group_id"`
		VisitEntryTime      time.Time     `bun:"visit_entry_time"`
		VisitExitTime       *time.Time    `bun:"visit_exit_time"`
		VisitCreatedAt      time.Time     `bun:"visit_created_at"`
		VisitUpdatedAt      time.Time     `bun:"visit_updated_at"`
		GroupID             sql.NullInt64 `bun:"group_id"`
		GroupStartTime      time.Time     `bun:"group_start_time"`
		GroupEndTime        *time.Time    `bun:"group_end_time"`
		GroupLastActivity   time.Time     `bun:"group_last_activity"`
		GroupTimeoutMinutes sql.NullInt64 `bun:"group_timeout_minutes"`
		GroupGroupID        sql.NullInt64 `bun:"group_group_id"`
		GroupDeviceID       sql.NullInt64 `bun:"group_device_id"`
		GroupRoomID         sql.NullInt64 `bun:"group_room_id"`
		GroupCreatedAt      time.Time     `bun:"group_created_at"`
		GroupUpdatedAt      time.Time     `bun:"group_updated_at"`
	}

	row := new(currentVisitRow)
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.visits AS "visit"`).
		ColumnExpr(`"visit".id AS visit_id`).
		ColumnExpr(`"visit".student_id AS visit_student_id`).
		ColumnExpr(`"visit".active_group_id AS visit_active_group_id`).
		ColumnExpr(`"visit".entry_time AS visit_entry_time`).
		ColumnExpr(`"visit".exit_time AS visit_exit_time`).
		ColumnExpr(`"visit".created_at AS visit_created_at`).
		ColumnExpr(`"visit".updated_at AS visit_updated_at`).
		ColumnExpr(`"group".id AS group_id`).
		ColumnExpr(`"group".start_time AS group_start_time`).
		ColumnExpr(`"group".end_time AS group_end_time`).
		ColumnExpr(`"group".last_activity AS group_last_activity`).
		ColumnExpr(`"group".timeout_minutes AS group_timeout_minutes`).
		ColumnExpr(`"group".group_id AS group_group_id`).
		ColumnExpr(`"group".device_id AS group_device_id`).
		ColumnExpr(`"group".room_id AS group_room_id`).
		ColumnExpr(`"group".created_at AS group_created_at`).
		ColumnExpr(`"group".updated_at AS group_updated_at`).
		Join(`LEFT JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Where(`"visit".student_id = ? AND "visit".exit_time IS NULL`, studentID).
		OrderExpr(`"visit".entry_time DESC`).
		Limit(1).
		Scan(ctx, row)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by student ID with room",
			Err: base.TranslateNotFound(err),
		}
	}

	visit := &active.Visit{
		Model: modelBase.Model{
			ID:        row.VisitID,
			CreatedAt: row.VisitCreatedAt,
			UpdatedAt: row.VisitUpdatedAt,
		},
		StudentID:     row.VisitStudentID,
		ActiveGroupID: row.VisitActiveGroupID,
		EntryTime:     row.VisitEntryTime,
		ExitTime:      row.VisitExitTime,
	}

	if row.GroupID.Valid && row.GroupRoomID.Valid {
		// Row.GroupGroupID may legitimately be NULL for spontaneous sessions
		// (WP-B6) — map it to *int64, leaving nil when the row carries no
		// parent template.
		var templateID *int64
		if row.GroupGroupID.Valid {
			v := row.GroupGroupID.Int64
			templateID = &v
		}
		group := &active.Group{
			Model: modelBase.Model{
				ID:        row.GroupID.Int64,
				CreatedAt: row.GroupCreatedAt,
				UpdatedAt: row.GroupUpdatedAt,
			},
			StartTime:    row.GroupStartTime,
			EndTime:      row.GroupEndTime,
			LastActivity: row.GroupLastActivity,
			GroupID:      templateID,
			RoomID:       row.GroupRoomID.Int64,
		}
		if row.GroupTimeoutMinutes.Valid {
			group.TimeoutMinutes = int(row.GroupTimeoutMinutes.Int64)
		}
		if row.GroupDeviceID.Valid {
			deviceID := row.GroupDeviceID.Int64
			group.DeviceID = &deviceID
		}
		// The room owner resolves the room the former LEFT JOIN projected; a
		// room it cannot see leaves Room nil, as the join did (#2665).
		rooms, err := roomsByID(ctx, r.rooms, []int64{row.GroupRoomID.Int64})
		if err != nil {
			return nil, &modelBase.DatabaseError{Op: "get current by student ID with room", Err: err}
		}
		if room, ok := rooms[row.GroupRoomID.Int64]; ok {
			group.Room = &facilities.Room{
				ID:        room.ID,
				CreatedAt: room.CreatedAt,
				UpdatedAt: room.UpdatedAt,
				Name:      room.Name,
			}
		}
		visit.ActiveGroup = group
	}

	return visit, nil
}

// GetCurrentByStudentIDs finds current active visits for multiple students in a single query
func (r *VisitRepository) GetCurrentByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error) {
	result := make(map[int64]*active.Visit, len(studentIDs))

	if len(studentIDs) == 0 {
		return result, nil
	}

	uniqueIDs := make([]int64, 0, len(studentIDs))
	seen := make(map[int64]struct{}, len(studentIDs))
	for _, id := range studentIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	var visits []*active.Visit
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id IN (?)`, bun.List(uniqueIDs)).
		Where(`"visit".exit_time IS NULL`).
		OrderExpr(`"visit".student_id ASC`).
		OrderExpr(`"visit".entry_time DESC`)

	query = base.WithTenantFilter(ctx, query, "visit")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by student IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	for _, visit := range visits {
		if _, exists := result[visit.StudentID]; !exists {
			result[visit.StudentID] = visit
		}
	}

	return result, nil
}

// CountActiveByRoomID counts active visits across all active groups in the given room.
func (r *VisitRepository) CountActiveByRoomID(ctx context.Context, roomID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.visits AS "visit"`).
		Join(`JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Where(`"group".room_id = ?`, roomID).
		Where(`"group".end_time IS NULL`).
		Where(`"visit".exit_time IS NULL`).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count active by room ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return count, nil
}

// ListActiveStudentIDsByRoomID returns the IDs of students currently
// checked-in to any active (end_time IS NULL) group in the given room.
// Callers push the IDs through the standard student list pipeline, which
// handles display fields, GDPR redaction, and pagination.
func (r *VisitRepository) ListActiveStudentIDsByRoomID(ctx context.Context, roomID int64) ([]int64, error) {
	var ids []int64
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveVisitsAsVisit).
		ColumnExpr(`"visit".student_id`).
		Join(`JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Where(`"group".room_id = ?`, roomID).
		Where(`"group".end_time IS NULL`).
		Where(`"visit".exit_time IS NULL`)

	query = base.WithTenantFilter(ctx, query, "visit")

	if err := query.Scan(ctx, &ids); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list active student IDs by room ID",
			Err: base.TranslateNotFound(err),
		}
	}
	return ids, nil
}

// ListOpenVisitStudentIDsByRoom returns every student of the tenant who is
// currently checked in, grouped by the room they are in. It is the whole-tenant
// counterpart of ListActiveStudentIDsByRoomID.
//
// Deliberately parameterless: a caller resolving many supervised rooms would
// otherwise issue one query per room, and a tenant only ever has a few hundred
// open visits, so narrowing by a room IN-list buys nothing while forcing two
// code paths.
//
// Student IDs are distinct per room: two open visits for the same child in two
// groups sharing a room collapse to one entry, so callers can range over the
// slice without building their own set.
func (r *VisitRepository) ListOpenVisitStudentIDsByRoom(ctx context.Context) (map[int64][]int64, error) {
	var rows []struct {
		RoomID    int64 `bun:"room_id"`
		StudentID int64 `bun:"student_id"`
	}

	query := base.GetDB(ctx, r.db).NewSelect().
		Distinct().
		TableExpr(tableExprActiveVisitsAsVisit).
		ColumnExpr(`"group".room_id, "visit".student_id`).
		Join(`JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Where(`"group".end_time IS NULL`).
		Where(`"visit".exit_time IS NULL`)

	query = base.WithTenantFilter(ctx, query, "visit")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list open visit student IDs by room",
			Err: base.TranslateNotFound(err),
		}
	}

	byRoom := make(map[int64][]int64)
	for _, row := range rows {
		byRoom[row.RoomID] = append(byRoom[row.RoomID], row.StudentID)
	}

	return byRoom, nil
}

// CountActiveByGroupID counts active visits in the given active group.
func (r *VisitRepository) CountActiveByGroupID(ctx context.Context, activeGroupID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.visits AS "visit"`).
		Where(`"visit".active_group_id = ?`, activeGroupID).
		Where(`"visit".exit_time IS NULL`).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count active by group ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return count, nil
}

// EndVisitsByActiveGroupIDs ends all active visits for multiple group IDs in a single query.
// Returns the number of visits ended.
func (r *VisitRepository) EndVisitsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) (int64, error) {
	if len(activeGroupIDs) == 0 {
		return 0, nil
	}

	// GREATEST clamp: entry_time is written from the app host's clock (IoT/web
	// check-in), now() is the DB server's clock. When the app clock runs ahead
	// (Docker VM skew locally, NTP drift between hosts in production), a bulk
	// end shortly after a check-in would set exit_time < entry_time and trip
	// chk_entry_before_exit for the WHOLE batch. Clamping to entry_time keeps
	// the invariant and gives the just-checked-in visit a zero duration.
	query := base.GetDB(ctx, r.db).NewUpdate().
		Table(tableActiveVisits).
		Set("exit_time = GREATEST(now(), entry_time)").
		Where("active_group_id IN (?)", bun.List(activeGroupIDs)).
		Where("exit_time IS NULL")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end visits by active group IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end visits by active group IDs (rows affected)",
			Err: base.TranslateNotFound(err),
		}
	}

	return rowsAffected, nil
}

// visitGroupNames captures the activity + room names for a student visit (DB scan target).
type visitGroupNames struct {
	StudentID         int64  `bun:"student_id"`
	ActivityGroupName string `bun:"activity_group_name"`
	RoomID            *int64 `bun:"room_id"`
}

// GetTodayVisitNamesForStudents returns activity group + room names for all of
// today's visits for the given students. Used for tracking indicator matching.
func (r *VisitRepository) GetTodayVisitNamesForStudents(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}

	// Deduplicate incoming IDs.
	uniqueIDs := make([]int64, 0, len(studentIDs))
	seen := make(map[int64]struct{}, len(studentIDs))
	for _, id := range studentIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	var results []visitGroupNames

	today := timezone.Today()

	query := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		ColumnExpr(`"visit".student_id`).
		ColumnExpr(`COALESCE("activity"."name", '') AS activity_group_name`).
		ColumnExpr(`"group".room_id AS room_id`).
		Join(`LEFT JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Join(`LEFT JOIN activities.groups AS "activity" ON "activity".id = "group".group_id`).
		Where(`"visit".student_id IN (?)`, bun.List(uniqueIDs)).
		Where(`"visit".entry_time >= ?`, today)

	query = base.WithTenantFilter(ctx, query, "visit")

	if err := query.Scan(ctx, &results); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get today visit names for students",
			Err: base.TranslateNotFound(err),
		}
	}

	// The room owner resolves the names the former LEFT JOIN + COALESCE
	// projected; an invisible room yields "" (#2665).
	roomIDs := make([]int64, 0, len(results))
	for _, row := range results {
		if row.RoomID != nil {
			roomIDs = append(roomIDs, *row.RoomID)
		}
	}
	rooms, err := roomsByID(ctx, r.rooms, roomIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "get today visit names for students", Err: err}
	}

	// Convert to model type.
	out := make([]active.VisitGroupNames, len(results))
	for i, row := range results {
		name := ""
		if row.RoomID != nil {
			name = rooms[*row.RoomID].Name
		}
		out[i] = active.VisitGroupNames{
			StudentID:         row.StudentID,
			ActivityGroupName: row.ActivityGroupName,
			RoomName:          name,
		}
	}

	return out, nil
}

// FindByStudentAndActiveGroupIDs returns visits for the given student whose
// active_group_id is in the provided slice. Used by the timetable /student/
// {id}/day and /week endpoints to detect "unplanned" attendance — instances
// where the student was actually there (has a visit) without being enrolled
// (no instance_students row).
//
// Empty slice short-circuits to an empty result: bun would otherwise emit
// `IN ('{}')` which some driver paths handle oddly. Bailing early keeps the
// call cheap on the common "no active/completed instances" path.
func (r *VisitRepository) FindByStudentAndActiveGroupIDs(
	ctx context.Context, studentID int64, activeGroupIDs []int64,
) ([]*active.Visit, error) {
	if len(activeGroupIDs) == 0 {
		return []*active.Visit{}, nil
	}

	var visits []*active.Visit
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".active_group_id IN (?)`, bun.List(activeGroupIDs)).
		OrderExpr(`"visit".entry_time ASC`)

	query = base.WithTenantFilter(ctx, query, "visit")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and active group IDs",
			Err: base.TranslateNotFound(err),
		}
	}
	return visits, nil
}

// FindActiveVisits finds all visits with no exit time (currently active)
func (r *VisitRepository) FindActiveVisits(ctx context.Context) ([]*active.Visit, error) {
	var visits []*active.Visit
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".exit_time IS NULL`).
		Order(`entry_time ASC`)

	query = base.WithTenantFilter(ctx, query, "visit")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active visits",
			Err: base.TranslateNotFound(err),
		}
	}

	return visits, nil
}

// GetCurrentRoomNamesForStudents returns the room name of each student's
// current open visit (visit without exit_time in a still-running active
// group), newest visit first per student. Students without an open visit are
// absent from the map. Custom method (backend-conventions Rule 2):
// DISTINCT ON projection joining active.groups for the emergency list; the
// room names come from the Facilities owner (#2665). Tenant scoping via defense-in-depth predicate on top of
// the caller's RLS transaction.
func (r *VisitRepository) GetCurrentRoomNamesForStudents(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	if len(studentIDs) == 0 {
		return map[int64]string{}, nil
	}

	type currentLocationRow struct {
		StudentID int64 `bun:"student_id"`
		RoomID    int64 `bun:"room_id"`
	}

	var rows []currentLocationRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.visits AS "visit"`).
		ColumnExpr(`DISTINCT ON ("visit".student_id) "visit".student_id`).
		ColumnExpr(`"group".room_id AS "room_id"`).
		Join(`JOIN active.groups AS "group" ON "group".id = "visit".active_group_id AND "group".end_time IS NULL`).
		Where(`"visit".student_id IN (?)`, bun.List(studentIDs)).
		Where(`"visit".exit_time IS NULL`).
		OrderExpr(`"visit".student_id ASC`).
		OrderExpr(`"visit".entry_time DESC`)

	query = base.WithTenantFilter(ctx, query, "visit")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current room names for students",
			Err: base.TranslateNotFound(err),
		}
	}

	// The room owner resolves the names; a student whose room it cannot see
	// stays absent, as the former INNER JOIN dropped the row (#2665).
	roomIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		roomIDs = append(roomIDs, row.RoomID)
	}
	rooms, err := roomsByID(ctx, r.rooms, roomIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "get current room names for students", Err: err}
	}
	locations := make(map[int64]string, len(rows))
	for _, row := range rows {
		if room, ok := rooms[row.RoomID]; ok {
			locations[row.StudentID] = room.Name
		}
	}
	return locations, nil
}

// FindActiveWithStudentDisplayByGroup returns the open visits of an active
// group with student display data, newest entry first (issue #584: moved
// out of api/active). The student columns come from the People Directory
// (#2662), the person names from the composition layer. A visit whose
// student the directory does not return is dropped, as the former inner
// join dropped it.
func (r *VisitRepository) FindActiveWithStudentDisplayByGroup(ctx context.Context, activeGroupID int64) ([]*active.VisitWithStudentDisplay, error) {
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	var results []*active.VisitWithStudentDisplay
	err := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr("v.id AS visit_id").
		ColumnExpr("v.student_id").
		ColumnExpr("v.active_group_id").
		ColumnExpr("v.entry_time").
		ColumnExpr("v.exit_time").
		ColumnExpr("v.created_at").
		ColumnExpr("v.updated_at").
		TableExpr("active.visits AS v").
		Where("v.active_group_id = ?", activeGroupID).
		Where("v.exit_time IS NULL").
		OrderExpr("v.entry_time DESC").
		Scan(ctx, &results)
	if err != nil || len(results) == 0 {
		return results, err
	}
	ids := make([]int64, 0, len(results))
	for _, row := range results {
		ids = append(ids, row.StudentID)
	}
	students, err := r.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]DirectoryStudent, len(students))
	for _, student := range students {
		byID[student.ID] = student
	}
	kept := make([]*active.VisitWithStudentDisplay, 0, len(results))
	for _, row := range results {
		student, found := byID[row.StudentID]
		if !found {
			continue
		}
		row.PersonID = student.PersonID
		row.SchoolClass = student.SchoolClass
		row.GroupID = student.GroupID
		row.Sick = student.Sick
		row.SickSince = student.SickSince
		row.Excused = student.Excused
		row.ExcusedSince = student.ExcusedSince
		row.PhotoPath = student.PhotoPath
		kept = append(kept, row)
	}
	return kept, nil
}
