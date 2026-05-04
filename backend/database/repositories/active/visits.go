// backend/database/repositories/active/visit.go
package active

import (
	"context"
	"database/sql"
	"fmt"
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
	db *bun.DB
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

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by student ID",
			Err: err,
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

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group ID",
			Err: err,
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

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by time range",
			Err: err,
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
	GroupRoomID   int64  `bun:"group__room_id"`
	GroupRoomName string `bun:"room__name"`
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
		ColumnExpr(`"room".name AS "room__name"`).
		Join(`LEFT JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".entry_time >= ?`, start).
		Where(`"visit".entry_time <= ?`, end).
		OrderExpr(`"visit".entry_time ASC`)

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and time range",
			Err: err,
		}
	}

	// Materialize into []*active.Visit with ActiveGroup + Room populated.
	// When the LEFT JOIN finds no group, GroupRoomID is 0 — leave ActiveGroup nil.
	visits := make([]*active.Visit, 0, len(results))
	for i := range results {
		v := results[i].Visit
		if results[i].GroupRoomID != 0 {
			v.ActiveGroup = &active.Group{
				RoomID: results[i].GroupRoomID,
				Room: &facilities.Room{
					Name: results[i].GroupRoomName,
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
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "end visit")
}

// Create overrides base Create to handle validation
func (r *VisitRepository) Create(ctx context.Context, visit *active.Visit) error {
	if visit == nil {
		return fmt.Errorf("visit cannot be nil")
	}

	// Validate visit
	if err := visit.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, visit)
}

// List overrides the base List method to accept the new QueryOptions type
func (r *VisitRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.Visit, error) {
	var visits []*active.Visit
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`)

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return visits, nil
}

// FindWithStudent retrieves visits with student details
func (r *VisitRepository) FindWithStudent(ctx context.Context, id int64) (*active.Visit, error) {
	visit := new(active.Visit)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(visit).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Relation("Student").
		Where(`"visit".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with student",
			Err: err,
		}
	}

	return visit, nil
}

// FindWithActiveGroup retrieves visits with active group details
func (r *VisitRepository) FindWithActiveGroup(ctx context.Context, id int64) (*active.Visit, error) {
	visit := new(active.Visit)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(visit).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with active group",
			Err: err,
		}
	}

	// Load active group separately — BUN Relation("ActiveGroup") does not
	// resolve the schema:active tag for relation sub-queries.
	group := new(active.Group)
	err = base.GetDB(ctx, r.db).NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id = ?`, visit.ActiveGroupID).
		Scan(ctx)

	if err == nil {
		visit.ActiveGroup = group
	}

	return visit, nil
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
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get affected rows from visit transfer",
			Err: err,
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

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete expired visits",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}

	return rowsAffected, nil
}

// DeleteVisitsBeforeDate deletes visits created before a specific date for a student
func (r *VisitRepository) DeleteVisitsBeforeDate(ctx context.Context, studentID int64, beforeDate time.Time) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*active.Visit)(nil)).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".created_at < ?`, beforeDate).
		Where(`"visit".exit_time IS NOT NULL`) // Only delete completed visits

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete visits before date",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
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

	if where, val, ok := base.TenantWhere(ctx, "v"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx, &results)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get visit retention stats",
			Err: err,
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

	if where, val, ok := base.TenantWhere(ctx, "v"); ok {
		query = query.Where(where, val)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count expired visits",
			Err: err,
		}
	}

	return int64(count), nil
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

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by student ID",
			Err: err,
		}
	}

	return visit, nil
}

// GetCurrentByStudentIDWithRoom finds the current active visit for a student and loads the active group and room.
func (r *VisitRepository) GetCurrentByStudentIDWithRoom(ctx context.Context, studentID int64) (*active.Visit, error) {
	type currentVisitRow struct {
		VisitID             int64          `bun:"visit_id"`
		VisitStudentID      int64          `bun:"visit_student_id"`
		VisitActiveGroupID  int64          `bun:"visit_active_group_id"`
		VisitEntryTime      time.Time      `bun:"visit_entry_time"`
		VisitExitTime       *time.Time     `bun:"visit_exit_time"`
		VisitCreatedAt      time.Time      `bun:"visit_created_at"`
		VisitUpdatedAt      time.Time      `bun:"visit_updated_at"`
		GroupID             sql.NullInt64  `bun:"group_id"`
		GroupStartTime      time.Time      `bun:"group_start_time"`
		GroupEndTime        *time.Time     `bun:"group_end_time"`
		GroupLastActivity   time.Time      `bun:"group_last_activity"`
		GroupTimeoutMinutes sql.NullInt64  `bun:"group_timeout_minutes"`
		GroupGroupID        sql.NullInt64  `bun:"group_group_id"`
		GroupDeviceID       sql.NullInt64  `bun:"group_device_id"`
		GroupRoomID         sql.NullInt64  `bun:"group_room_id"`
		GroupCreatedAt      time.Time      `bun:"group_created_at"`
		GroupUpdatedAt      time.Time      `bun:"group_updated_at"`
		RoomID              sql.NullInt64  `bun:"room_id"`
		RoomName            sql.NullString `bun:"room_name"`
		RoomCreatedAt       time.Time      `bun:"room_created_at"`
		RoomUpdatedAt       time.Time      `bun:"room_updated_at"`
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
		ColumnExpr(`"room".id AS room_id`).
		ColumnExpr(`"room".name AS room_name`).
		ColumnExpr(`"room".created_at AS room_created_at`).
		ColumnExpr(`"room".updated_at AS room_updated_at`).
		Join(`LEFT JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`).
		Where(`"visit".student_id = ? AND "visit".exit_time IS NULL`, studentID).
		OrderExpr(`"visit".entry_time DESC`).
		Limit(1).
		Scan(ctx, row)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by student ID with room",
			Err: err,
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
		if row.RoomID.Valid {
			group.Room = &facilities.Room{
				Model: modelBase.Model{
					ID:        row.RoomID.Int64,
					CreatedAt: row.RoomCreatedAt,
					UpdatedAt: row.RoomUpdatedAt,
				},
				Name: row.RoomName.String,
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

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by student IDs",
			Err: err,
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
			Err: err,
		}
	}

	return count, nil
}

// ListActiveByRoomID returns one row per student currently checked-in to any
// active (end_time IS NULL) group in the given room. Joins users.students /
// users.persons for display fields and education.groups for the student's
// Stammgruppe name (used as the "Gruppe" label on the room-detail UI — keeps
// the semantic aligned with Kindersuche, which also surfaces the Stammgruppe
// rather than the active activity). Ordered by last name then first name so
// the API response is stable without an additional client sort.
func (r *VisitRepository) ListActiveByRoomID(ctx context.Context, roomID int64) ([]*active.StudentInRoomVisit, error) {
	type row struct {
		StudentID          int64          `bun:"student_id"`
		FirstName          string         `bun:"first_name"`
		LastName           string         `bun:"last_name"`
		EducationGroupID   sql.NullInt64  `bun:"education_group_id"`
		EducationGroupName sql.NullString `bun:"education_group_name"`
		ActiveGroupID      int64          `bun:"active_group_id"`
		EntryTime          time.Time      `bun:"entry_time"`
	}

	var rows []row
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveVisitsAsVisit).
		ColumnExpr(`"visit".student_id AS student_id`).
		ColumnExpr(`"person".first_name AS first_name`).
		ColumnExpr(`"person".last_name AS last_name`).
		ColumnExpr(`"student".group_id AS education_group_id`).
		ColumnExpr(`"edu_group".name AS education_group_name`).
		ColumnExpr(`"visit".active_group_id AS active_group_id`).
		ColumnExpr(`"visit".entry_time AS entry_time`).
		Join(`JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Join(`JOIN users.students AS "student" ON "student".id = "visit".student_id`).
		Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Join(`LEFT JOIN education.groups AS "edu_group" ON "edu_group".id = "student".group_id`).
		Where(`"group".room_id = ?`, roomID).
		Where(`"group".end_time IS NULL`).
		Where(`"visit".exit_time IS NULL`).
		OrderExpr(`"person".last_name ASC, "person".first_name ASC, "visit".entry_time ASC`)

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list active visits by room ID",
			Err: err,
		}
	}

	out := make([]*active.StudentInRoomVisit, len(rows))
	for i, row := range rows {
		v := &active.StudentInRoomVisit{
			StudentID:     row.StudentID,
			FirstName:     row.FirstName,
			LastName:      row.LastName,
			ActiveGroupID: row.ActiveGroupID,
			EntryTime:     row.EntryTime,
		}
		if row.EducationGroupID.Valid {
			gid := row.EducationGroupID.Int64
			v.EducationGroupID = &gid
		}
		if row.EducationGroupName.Valid {
			v.EducationGroupName = row.EducationGroupName.String
		}
		out[i] = v
	}
	return out, nil
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
			Err: err,
		}
	}

	return count, nil
}

// CountActiveByStudentID counts active visits (exit_time IS NULL) for a student.
func (r *VisitRepository) CountActiveByStudentID(ctx context.Context, studentID int64) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.visits AS "visit"`).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".exit_time IS NULL`)

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count active by student ID",
			Err: err,
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

	query := base.GetDB(ctx, r.db).NewUpdate().
		Table(tableActiveVisits).
		Set("exit_time = now()").
		Where("active_group_id IN (?)", bun.List(activeGroupIDs)).
		Where("exit_time IS NULL")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end visits by active group IDs",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end visits by active group IDs (rows affected)",
			Err: err,
		}
	}

	return rowsAffected, nil
}

// visitGroupNames captures the activity + room names for a student visit (DB scan target).
type visitGroupNames struct {
	StudentID         int64  `bun:"student_id"`
	ActivityGroupName string `bun:"activity_group_name"`
	RoomName          string `bun:"room_name"`
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
		ColumnExpr(`COALESCE("room"."name", '') AS room_name`).
		Join(`LEFT JOIN active.groups AS "group" ON "group".id = "visit".active_group_id`).
		Join(`LEFT JOIN activities.groups AS "activity" ON "activity".id = "group".group_id`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`).
		Where(`"visit".student_id IN (?)`, bun.List(uniqueIDs)).
		Where(`"visit".entry_time >= ?`, today)

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx, &results); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get today visit names for students",
			Err: err,
		}
	}

	// Convert to model type.
	out := make([]active.VisitGroupNames, len(results))
	for i, r := range results {
		out[i] = active.VisitGroupNames{
			StudentID:         r.StudentID,
			ActivityGroupName: r.ActivityGroupName,
			RoomName:          r.RoomName,
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

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and active group IDs",
			Err: err,
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

	if where, val, ok := base.TenantWhere(ctx, "visit"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active visits",
			Err: err,
		}
	}

	return visits, nil
}
