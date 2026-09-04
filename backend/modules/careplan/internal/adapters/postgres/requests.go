package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

const (
	excusedRequestLockClass int32 = 0x65786162
	requestPending                = "pending"
	requestApproved               = "approved"
	requestRejected               = "rejected"
	requestCareEnded              = "care_ended"
	dataTargetPerson              = "person"
	dataTargetDeparture           = "departure"
)

type requestStore struct{ *Store }

func NewRequestStore(store *Store) *requestStore {
	if store == nil {
		panic("care plan request store: store is required")
	}
	return &requestStore{Store: store}
}

type excusedAbsenceRequestRow struct {
	bun.BaseModel  `bun:"table:excused_absence_requests,alias:excused_absence_request"`
	ID             int64          `bun:"id,pk,autoincrement"`
	TenantID       int64          `bun:"tenant_id,notnull"`
	CreatedAt      time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID      int64          `bun:"student_id,notnull"`
	SubmittedBy    int64          `bun:"submitted_by,notnull"`
	Dates          []calendarDate `bun:"dates,type:jsonb,notnull"`
	Note           string         `bun:"note,notnull"`
	AbsenceStatus  string         `bun:"absence_status,notnull,default:'excused'"`
	Status         string         `bun:"status,notnull,default:'pending'"`
	DecisionReason *string        `bun:"decision_reason"`
	ReviewedBy     *int64         `bun:"reviewed_by"`
	ReviewedAt     *time.Time     `bun:"reviewed_at"`
	AppliedAt      *time.Time     `bun:"applied_at"`
}

type careScheduleRequestRow struct {
	bun.BaseModel    `bun:"table:care_schedule_change_requests,alias:care_schedule_change_request"`
	ID               int64           `bun:"id,pk,autoincrement"`
	TenantID         int64           `bun:"tenant_id,notnull"`
	CreatedAt        time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID        int64           `bun:"student_id,notnull"`
	SubmittedBy      int64           `bun:"submitted_by,notnull"`
	RequestKind      string          `bun:"request_kind,notnull"`
	Payload          json.RawMessage `bun:"payload,type:jsonb,notnull"`
	Status           string          `bun:"status,notnull,default:'pending'"`
	DecisionReason   *string         `bun:"decision_reason"`
	ReviewedBy       *int64          `bun:"reviewed_by"`
	ReviewedAt       *time.Time      `bun:"reviewed_at"`
	AppliedAt        *time.Time      `bun:"applied_at"`
	DecisionSnapshot json.RawMessage `bun:"decision_snapshot,type:jsonb"`
}

type studentDataRequestRow struct {
	bun.BaseModel `bun:"table:student_data_change_requests,alias:student_data_change_request"`
	ID            int64           `bun:"id,pk,autoincrement"`
	TenantID      int64           `bun:"tenant_id,notnull"`
	CreatedAt     time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time       `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID     int64           `bun:"student_id,notnull"`
	SubmittedBy   int64           `bun:"submitted_by,notnull"`
	Target        string          `bun:"target,notnull"`
	TargetRefID   *int64          `bun:"target_ref_id"`
	FieldKey      string          `bun:"field_key,notnull"`
	OldValue      json.RawMessage `bun:"old_value,type:jsonb"`
	NewValue      json.RawMessage `bun:"new_value,type:jsonb,notnull"`
	Status        string          `bun:"status,notnull,default:'pending'"`
	ReviewReason  *string         `bun:"review_reason"`
	ReviewedBy    *int64          `bun:"reviewed_by"`
	ReviewedAt    *time.Time      `bun:"reviewed_at"`
	AppliedAt     *time.Time      `bun:"applied_at"`
}

func (s *requestStore) CountOpenCareRequests(ctx context.Context, studentIDs []int64) (map[int64]int, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, carePlanCompose.RequestStoreStats{}, err
	}
	counts := make(map[int64]int, len(studentIDs))
	stats := carePlanCompose.RequestStoreStats{}
	started := time.Now()
	add := func(query *bun.SelectQuery) error {
		var rows []struct {
			StudentID int64 `bun:"student_id"`
			Total     int   `bun:"total"`
		}
		stats.Queries++
		if err := query.Scan(ctx, &rows); err != nil {
			return err
		}
		stats.Rows += int64(len(rows))
		for _, row := range rows {
			counts[row.StudentID] += row.Total
		}
		return nil
	}
	queries := []*bun.SelectQuery{
		withTenant(db.NewSelect().TableExpr(`users.student_data_change_requests AS "request"`).ColumnExpr(`"request".student_id, COUNT(*)::int AS total`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, requestPending).GroupExpr(`"request".student_id`), "request", tenantID),
		withTenant(db.NewSelect().TableExpr(`active.excused_absence_requests AS "request"`).ColumnExpr(`"request".student_id, COUNT(*)::int AS total`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, requestPending).GroupExpr(`"request".student_id`), "request", tenantID),
		withTenant(db.NewSelect().TableExpr(`schedule.care_schedule_change_requests AS "request"`).ColumnExpr(`"request".student_id, COUNT(*)::int AS total`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, requestPending).GroupExpr(`"request".student_id`), "request", tenantID),
	}
	for _, query := range queries {
		if err := add(query); err != nil {
			stats.StatementDuration = time.Since(started)
			return nil, stats, requestDBError("count open care requests", err)
		}
	}
	stats.StatementDuration = time.Since(started)
	return counts, stats, nil
}

func (s *requestStore) LockOpenCareRequests(ctx context.Context, studentIDs []int64) (carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	stats := carePlanCompose.RequestStoreStats{}
	started := time.Now()
	lock := func(query *bun.SelectQuery) error {
		var ids []int64
		stats.Queries++
		if err := query.Scan(ctx, &ids); err != nil {
			return err
		}
		stats.Rows += int64(len(ids))
		return nil
	}
	queries := []*bun.SelectQuery{
		withTenant(db.NewSelect().TableExpr(`users.student_data_change_requests AS "request"`).ColumnExpr(`"request".id`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, requestPending).OrderExpr(`"request".id ASC`).For("UPDATE"), "request", tenantID),
		withTenant(db.NewSelect().TableExpr(`active.excused_absence_requests AS "request"`).ColumnExpr(`"request".id`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, requestPending).OrderExpr(`"request".id ASC`).For("UPDATE"), "request", tenantID),
		withTenant(db.NewSelect().TableExpr(`schedule.care_schedule_change_requests AS "request"`).ColumnExpr(`"request".id`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, requestPending).OrderExpr(`"request".id ASC`).For("UPDATE"), "request", tenantID),
	}
	for _, query := range queries {
		if err := lock(query); err != nil {
			stats.StatementDuration = time.Since(started)
			return stats, requestDBError("lock open care requests", err)
		}
	}
	stats.StatementDuration = time.Since(started)
	return stats, nil
}

func (s *requestStore) CloseOpenCareRequests(ctx context.Context, studentIDs []int64, reason string, reviewedBy *int64, at time.Time) (int64, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, carePlanCompose.RequestStoreStats{}, err
	}
	stats := carePlanCompose.RequestStoreStats{}
	started := time.Now()
	var total int64
	closeRequest := func(query *bun.UpdateQuery) error {
		stats.Queries++
		result, err := query.Exec(ctx)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		stats.Rows += rows
		total += rows
		return nil
	}
	queries := []*bun.UpdateQuery{
		withTenant(db.NewUpdate().TableExpr(`users.student_data_change_requests AS "request"`).Set(`status = ?`, requestCareEnded).Set(`review_reason = ?`, reason).Set(`reviewed_by = ?`, reviewedBy).Set(`reviewed_at = ?`, at).Set(`updated_at = ?`, at).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, requestPending), "request", tenantID),
		withTenant(db.NewUpdate().TableExpr(`active.excused_absence_requests AS "request"`).Set(`status = ?`, requestCareEnded).Set(`decision_reason = ?`, reason).Set(`reviewed_by = ?`, reviewedBy).Set(`reviewed_at = ?`, at).Set(`updated_at = ?`, at).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, requestPending), "request", tenantID),
		withTenant(db.NewUpdate().TableExpr(`schedule.care_schedule_change_requests AS "request"`).Set(`status = ?`, requestCareEnded).Set(`decision_reason = ?`, reason).Set(`reviewed_by = ?`, reviewedBy).Set(`reviewed_at = ?`, at).Set(`updated_at = ?`, at).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, requestPending), "request", tenantID),
	}
	for _, query := range queries {
		if err := closeRequest(query); err != nil {
			stats.StatementDuration = time.Since(started)
			return 0, stats, requestDBError("close open care requests", err)
		}
	}
	stats.StatementDuration = time.Since(started)
	return total, stats, nil
}

func requestStats(started time.Time, rows int64) carePlanCompose.RequestStoreStats {
	return carePlanCompose.RequestStoreStats{Queries: 1, Rows: rows, StatementDuration: time.Since(started)}
}

func requestDBError(op string, err error) error {
	return fmt.Errorf("care plan postgres: %s: %w", op, err)
}

func applyRequestQueueFilters(query *bun.SelectQuery, alias, keysetColumn string, filter *careplan.RequestQueueFilter) *bun.SelectQuery {
	studentColumn := bun.Ident(alias + ".student_id")
	instantColumn := bun.Ident(alias + "." + keysetColumn)
	idColumn := bun.Ident(alias + ".id")
	if filter.StudentID > 0 {
		query = query.Where("? = ?", studentColumn, filter.StudentID)
	}
	if len(filter.StudentIDs) > 0 {
		query = query.Where("? IN (?)", studentColumn, bun.List(filter.StudentIDs))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		query = query.Where("?", bun.SafeQuery(`? IN (
			SELECT student.id
			FROM users.students AS student
			JOIN users.persons AS person ON person.id = student.person_id AND person.tenant_id = student.tenant_id
			WHERE student.tenant_id = ?
			AND (person.first_name || ' ' || person.last_name) ILIKE ? ESCAPE '\'
		)`, studentColumn, bun.Ident(alias+".tenant_id"), "%"+escapeILike(search)+"%"))
	}
	if !filter.BeforeInstant.IsZero() {
		query = query.Where("(?, ?) < (?, ?)", instantColumn, idColumn, filter.BeforeInstant, filter.BeforeID)
	}
	query = query.OrderExpr("? DESC", instantColumn).OrderExpr("? DESC", idColumn)
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	return query
}

func applyRequestUrgency(query *bun.SelectQuery, filter *careplan.RequestQueueFilter, expression string, args ...any) *bun.SelectQuery {
	if filter.UrgentOnly == nil {
		return query
	}
	if *filter.UrgentOnly {
		return query.Where("?", bun.SafeQuery(expression, args...))
	}
	return query.Where("NOT (?)", bun.SafeQuery(expression, args...))
}

func escapeILike(value string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(value)
}

func (s *requestStore) FindExcusedAbsenceRequest(ctx context.Context, id int64, lock bool) (careplan.ExcusedAbsenceRequest, bool, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return careplan.ExcusedAbsenceRequest{}, false, carePlanCompose.RequestStoreStats{}, err
	}
	row := new(excusedAbsenceRequestRow)
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(`active.excused_absence_requests AS "excused_absence_request"`).
		Where(`"excused_absence_request".id = ?`, id), "excused_absence_request", tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := requestStats(started, 1)
	if errors.Is(err, sql.ErrNoRows) {
		stats.Rows = 0
		return careplan.ExcusedAbsenceRequest{}, false, stats, nil
	}
	if err != nil {
		stats.Rows = 0
		return careplan.ExcusedAbsenceRequest{}, false, stats, requestDBError("find excused absence request", err)
	}
	return excusedRequestToPublic(row), true, stats, nil
}

func (s *requestStore) FindPendingExcusedAbsenceRequest(ctx context.Context, id int64) (careplan.ExcusedAbsenceRequest, carePlanCompose.RequestStoreStats, error) {
	row, found, stats, err := s.FindExcusedAbsenceRequest(ctx, id, true)
	if err != nil {
		return row, stats, err
	}
	if !found {
		return row, stats, careplan.ErrExcusedRequestNotFound
	}
	if row.Status != requestPending {
		return row, stats, careplan.ErrExcusedRequestNotPending
	}
	return row, stats, nil
}

func (s *requestStore) ListExcusedAbsenceRequests(ctx context.Context, filter careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, carePlanCompose.RequestStoreStats{}, err
	}
	rows := []excusedAbsenceRequestRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`active.excused_absence_requests AS "excused_absence_request"`), "excused_absence_request", tenantID)
	if filter.StudentID > 0 {
		query = query.Where(`"excused_absence_request".student_id = ?`, filter.StudentID)
	}
	if len(filter.Statuses) > 0 {
		query = query.Where(`"excused_absence_request".status IN (?)`, bun.List(filter.Statuses))
	}
	if !filter.RecentSince.IsZero() {
		query = query.Where(`COALESCE("excused_absence_request".reviewed_at, "excused_absence_request".updated_at) >= ?`, filter.RecentSince)
	}
	query = applyExcusedRequestOptions(query, filter)
	started := time.Now()
	err = query.Scan(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return nil, stats, requestDBError("list excused absence requests", err)
	}
	return excusedRequestsToPublic(rows), stats, nil
}

func applyExcusedRequestOptions(query *bun.SelectQuery, filter careplan.ExcusedAbsenceRequestFilter) *bun.SelectQuery {
	if filter.Queue != nil {
		if len(filter.Statuses) == 1 && filter.Statuses[0] == requestPending {
			query = applyRequestUrgency(query, filter.Queue, `"excused_absence_request".dates @> ?::jsonb`, `["`+filter.Queue.UrgentDate+`"]`)
			return applyRequestQueueFilters(query, "excused_absence_request", "created_at", filter.Queue)
		}
		return applyRequestQueueFilters(query, "excused_absence_request", "updated_at", filter.Queue)
	}
	if filter.Options != nil {
		query = applyStudentScheduleOptions(query, filter.Options, "excused_absence_request")
		if len(filter.Options.Sorting) > 0 {
			return query
		}
	}
	return query.OrderExpr(`"excused_absence_request".created_at DESC`).OrderExpr(`"excused_absence_request".id DESC`)
}

func (s *requestStore) CreateExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "create an excused absence request")
	if err != nil {
		return careplan.ExcusedAbsenceRequest{}, carePlanCompose.RequestStoreStats{}, err
	}
	row := excusedRequestFromPublic(value)
	row.TenantID = tenantID
	started := time.Now()
	err = db.NewInsert().Model(row).ModelTableExpr("active.excused_absence_requests").Returning("*").Scan(ctx)
	stats := requestStats(started, 1)
	if err != nil {
		stats.Rows = 0
		if isUniqueViolation(err) {
			stats.Conflicts = 1
		}
		return careplan.ExcusedAbsenceRequest{}, stats, requestDBError("create excused absence request", err)
	}
	return excusedRequestToPublic(row), stats, nil
}

func (s *requestStore) LockExcusedAbsenceRequests(ctx context.Context, studentID int64) (carePlanCompose.RequestStoreStats, error) {
	if studentID <= 0 || studentID > math.MaxInt32 {
		return carePlanCompose.RequestStoreStats{}, fmt.Errorf("LockStudentRequests: student_id %d out of advisory-lock range", studentID)
	}
	db, _, err := s.databaseForWrite(ctx, "lock excused absence requests")
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	started := time.Now()
	_, err = db.NewRaw("SELECT pg_advisory_xact_lock(?, ?)", excusedRequestLockClass, int32(studentID)).Exec(ctx)
	stats := requestStats(started, 0)
	if err != nil {
		return stats, requestDBError("lock student excused requests", err)
	}
	return stats, nil
}

func (s *requestStore) UpdatePendingExcusedAbsenceRequest(ctx context.Context, id int64, dates []careplan.Date, note, absenceStatus string) (carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "update an excused absence request")
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	rowDates := make([]calendarDate, len(dates))
	for i := range dates {
		rowDates[i] = calendarDate(dates[i])
	}
	query := withTenant(db.NewUpdate().Model((*excusedAbsenceRequestRow)(nil)).
		ModelTableExpr(`active.excused_absence_requests AS "excused_absence_request"`).
		Set("dates = ?", rowDates).Set("note = ?", note).Set("absence_status = ?", absenceStatus).Set("updated_at = ?", time.Now()).
		Where(`"excused_absence_request".id = ?`, id).Where(`"excused_absence_request".status = ?`, requestPending), "excused_absence_request", tenantID)
	return execRequest(ctx, query, "update pending excused absence request", careplan.ErrExcusedRequestNotPending)
}

func (s *requestStore) DecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision, correction bool) (carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "decide an excused absence request")
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	now := time.Now()
	query := db.NewUpdate().Model((*excusedAbsenceRequestRow)(nil)).
		ModelTableExpr(`active.excused_absence_requests AS "excused_absence_request"`).
		Set("status = ?", value.Status).Set("decision_reason = ?", value.Reason).Set("updated_at = ?", now).
		Where(`"excused_absence_request".id = ?`, value.ID)
	if correction {
		query = query.Set("reviewed_by = ?", reviewerID(value.ReviewedBy)).Set("reviewed_at = ?", now).
			Where(`"excused_absence_request".status IN (?)`, bun.List([]string{requestApproved, requestRejected}))
	} else {
		query = query.Where(`"excused_absence_request".status = ?`, requestPending)
		if value.ReviewedBy != nil && *value.ReviewedBy > 0 {
			query = query.Set("reviewed_by = ?", *value.ReviewedBy).Set("reviewed_at = ?", now)
		}
	}
	if value.Applied {
		query = query.Set("applied_at = ?", now)
	} else if correction {
		query = query.Set("applied_at = NULL")
	}
	query = withTenant(query, "excused_absence_request", tenantID)
	if correction {
		return execRequest(ctx, query, "correct excused absence request", careplan.ErrExcusedRequestNotDecided)
	}
	return execRequest(ctx, query, "decide excused absence request", careplan.ErrExcusedRequestNotPending)
}

func reviewerID(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func (s *requestStore) FindCareScheduleRequest(ctx context.Context, id int64, lock bool) (careplan.CareScheduleChangeRequest, bool, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return careplan.CareScheduleChangeRequest{}, false, carePlanCompose.RequestStoreStats{}, err
	}
	row := new(careScheduleRequestRow)
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`).
		Where(`"care_schedule_change_request".id = ?`, id), "care_schedule_change_request", tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := requestStats(started, 1)
	if errors.Is(err, sql.ErrNoRows) {
		stats.Rows = 0
		return careplan.CareScheduleChangeRequest{}, false, stats, nil
	}
	if err != nil {
		stats.Rows = 0
		return careplan.CareScheduleChangeRequest{}, false, stats, requestDBError("find care schedule change request", err)
	}
	return careScheduleRequestToPublic(row), true, stats, nil
}

func (s *requestStore) FindPendingCareScheduleRequest(ctx context.Context, id int64) (careplan.CareScheduleChangeRequest, carePlanCompose.RequestStoreStats, error) {
	row, found, stats, err := s.FindCareScheduleRequest(ctx, id, true)
	if err != nil {
		return row, stats, err
	}
	if !found {
		return row, stats, careplan.ErrCareScheduleRequestNotFound
	}
	if row.Status != requestPending {
		return row, stats, careplan.ErrCareScheduleRequestNotPending
	}
	return row, stats, nil
}

func (s *requestStore) ListCareScheduleRequests(ctx context.Context, filter careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, carePlanCompose.RequestStoreStats{}, err
	}
	rows := []careScheduleRequestRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`), "care_schedule_change_request", tenantID)
	if filter.StudentID > 0 {
		query = query.Where(`"care_schedule_change_request".student_id = ?`, filter.StudentID)
	}
	if len(filter.RequestKinds) > 0 {
		query = query.Where(`"care_schedule_change_request".request_kind IN (?)`, bun.List(filter.RequestKinds))
	}
	if len(filter.Statuses) > 0 {
		query = query.Where(`"care_schedule_change_request".status IN (?)`, bun.List(filter.Statuses))
	}
	query, err = applyCareScheduleRequestOptions(query, filter)
	if err != nil {
		return nil, carePlanCompose.RequestStoreStats{}, err
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return nil, stats, requestDBError("list care schedule change requests", err)
	}
	return careScheduleRequestsToPublic(rows), stats, nil
}

func applyCareScheduleRequestOptions(query *bun.SelectQuery, filter careplan.CareScheduleRequestFilter) (*bun.SelectQuery, error) {
	if !filter.RecentSince.IsZero() {
		return query.Where(`("care_schedule_change_request".status = ? OR "care_schedule_change_request".updated_at >= ?)`, requestPending, filter.RecentSince).
			OrderExpr(`"care_schedule_change_request".created_at DESC`).OrderExpr(`"care_schedule_change_request".id DESC`), nil
	}
	if filter.Queue == nil {
		return query.OrderExpr(`"care_schedule_change_request".created_at DESC`).OrderExpr(`"care_schedule_change_request".id DESC`), nil
	}
	if len(filter.Statuses) == 1 && filter.Statuses[0] == requestPending {
		if filter.Queue.UrgentOnly == nil {
			return applyRequestQueueFilters(query, "care_schedule_change_request", "created_at", filter.Queue), nil
		}
		date, err := time.Parse(time.DateOnly, filter.Queue.UrgentDate)
		if err != nil {
			return nil, fmt.Errorf("parse care schedule request urgency date: %w", err)
		}
		weekday := int(date.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		weekdayJSON := fmt.Sprintf(`[{"weekday":%d}]`, weekday)
		query = applyRequestUrgency(query, filter.Queue, `
			("care_schedule_change_request".request_kind = 'pickup_change' AND "care_schedule_change_request".payload->>'date' = ?)
			OR ("care_schedule_change_request".request_kind <> 'pickup_change' AND "care_schedule_change_request".payload->'weekdays' @> ?::jsonb)
		`, filter.Queue.UrgentDate, weekdayJSON)
		return applyRequestQueueFilters(query, "care_schedule_change_request", "created_at", filter.Queue), nil
	}
	return applyRequestQueueFilters(query, "care_schedule_change_request", "updated_at", filter.Queue), nil
}

func (s *requestStore) CreateCareScheduleRequest(ctx context.Context, value careplan.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "create a care schedule request")
	if err != nil {
		return careplan.CareScheduleChangeRequest{}, carePlanCompose.RequestStoreStats{}, err
	}
	row := careScheduleRequestFromPublic(value)
	row.TenantID = tenantID
	started := time.Now()
	err = db.NewInsert().Model(row).ModelTableExpr("schedule.care_schedule_change_requests").Returning("*").Scan(ctx)
	stats := requestStats(started, 1)
	if err != nil {
		stats.Rows = 0
		if isUniqueViolation(err) {
			stats.Conflicts = 1
		}
		return careplan.CareScheduleChangeRequest{}, stats, requestDBError("create care schedule change request", err)
	}
	return careScheduleRequestToPublic(row), stats, nil
}

func (s *requestStore) UpdatePendingCareScheduleRequest(ctx context.Context, id int64, payload json.RawMessage) (carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "update a care schedule request")
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*careScheduleRequestRow)(nil)).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`).
		Set("payload = ?", payload).Set("updated_at = ?", time.Now()).
		Where(`"care_schedule_change_request".id = ?`, id).Where(`"care_schedule_change_request".status = ?`, requestPending), "care_schedule_change_request", tenantID)
	return execRequest(ctx, query, "update pending care schedule change request", careplan.ErrCareScheduleRequestNotPending)
}

func (s *requestStore) DecideCareScheduleRequest(ctx context.Context, value careplan.CareScheduleRequestDecision, correction bool) (carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "decide a care schedule request")
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	now := time.Now()
	query := db.NewUpdate().Model((*careScheduleRequestRow)(nil)).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`).
		Set("status = ?", value.Status).Set("decision_reason = ?", value.Reason).Set("updated_at = ?", now).
		Where(`"care_schedule_change_request".id = ?`, value.ID)
	if correction {
		query = query.Set("reviewed_by = ?", reviewerID(value.ReviewedBy)).Set("reviewed_at = ?", now).
			Where(`"care_schedule_change_request".status IN (?)`, bun.List([]string{requestApproved, requestRejected}))
	} else {
		query = query.Where(`"care_schedule_change_request".status = ?`, requestPending)
		if value.ReviewedBy != nil && *value.ReviewedBy > 0 {
			query = query.Set("reviewed_by = ?", *value.ReviewedBy).Set("reviewed_at = ?", now)
		}
	}
	if value.Applied {
		query = query.Set("applied_at = ?", now)
	} else if correction {
		query = query.Set("applied_at = NULL")
	}
	query = withTenant(query, "care_schedule_change_request", tenantID)
	if correction {
		return execRequest(ctx, query, "correct care schedule change request", careplan.ErrCareScheduleRequestNotDecided)
	}
	return execRequest(ctx, query, "decide care schedule change request", careplan.ErrCareScheduleRequestNotPending)
}

func (s *requestStore) UpdateCareScheduleRequestSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) (carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "update a care schedule request snapshot")
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*careScheduleRequestRow)(nil)).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`).
		Set("decision_snapshot = ?", snapshot).Set("updated_at = ?", time.Now()).
		Where(`"care_schedule_change_request".id = ?`, id), "care_schedule_change_request", tenantID)
	return execRequest(ctx, query, "update care request decision snapshot", careplan.ErrCareScheduleRequestNotFound)
}

func (s *requestStore) FindStudentDataRequest(ctx context.Context, id int64, lock bool) (careplan.StudentDataChangeRequest, bool, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return careplan.StudentDataChangeRequest{}, false, carePlanCompose.RequestStoreStats{}, err
	}
	row := new(studentDataRequestRow)
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`).
		Where(`"student_data_change_request".id = ?`, id), "student_data_change_request", tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := requestStats(started, 1)
	if errors.Is(err, sql.ErrNoRows) {
		stats.Rows = 0
		return careplan.StudentDataChangeRequest{}, false, stats, nil
	}
	if err != nil {
		stats.Rows = 0
		return careplan.StudentDataChangeRequest{}, false, stats, requestDBError("find student data change request", err)
	}
	return studentDataRequestToPublic(row), true, stats, nil
}

func (s *requestStore) FindPendingStudentDataRequest(ctx context.Context, id int64) (careplan.StudentDataChangeRequest, carePlanCompose.RequestStoreStats, error) {
	row, found, stats, err := s.FindStudentDataRequest(ctx, id, true)
	if err != nil {
		return row, stats, err
	}
	if !found {
		return row, stats, careplan.ErrStudentDataRequestNotFound
	}
	if row.Status != requestPending {
		return row, stats, careplan.ErrStudentDataRequestNotPending
	}
	return row, stats, nil
}

func (s *requestStore) ListStudentDataRequests(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, carePlanCompose.RequestStoreStats{}, err
	}
	rows := []studentDataRequestRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`), "student_data_change_request", tenantID)
	if filter.StudentID > 0 {
		query = query.Where(`"student_data_change_request".student_id = ?`, filter.StudentID)
	}
	if len(filter.Statuses) > 0 {
		query = query.Where(`"student_data_change_request".status IN (?)`, bun.List(filter.Statuses))
	}
	if filter.ParentVisible {
		query = query.Where(`"student_data_change_request".target IN (?)`, bun.List([]string{dataTargetPerson, dataTargetDeparture}))
	}
	if filter.Queue != nil {
		if len(filter.Statuses) == 1 && filter.Statuses[0] == requestPending {
			query = applyRequestUrgency(query, filter.Queue, "FALSE")
			query = applyRequestQueueFilters(query, "student_data_change_request", "created_at", filter.Queue)
		} else {
			query = applyRequestQueueFilters(query, "student_data_change_request", "updated_at", filter.Queue)
		}
	} else {
		query = query.OrderExpr(`"student_data_change_request".created_at DESC`).OrderExpr(`"student_data_change_request".id DESC`)
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return nil, stats, requestDBError("list student data change requests", err)
	}
	return studentDataRequestsToPublic(rows), stats, nil
}

func (s *requestStore) HasPendingStudentDataRequest(ctx context.Context, studentID int64, target, field string) (bool, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, carePlanCompose.RequestStoreStats{}, err
	}
	query := withTenant(db.NewSelect().Model((*studentDataRequestRow)(nil)).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`).
		Where(`"student_data_change_request".student_id = ?`, studentID).
		Where(`"student_data_change_request".target = ?`, target).
		Where(`"student_data_change_request".field_key = ?`, field).
		Where(`"student_data_change_request".status = ?`, requestPending), "student_data_change_request", tenantID)
	started := time.Now()
	exists, err := query.Exists(ctx)
	stats := requestStats(started, 0)
	if exists {
		stats.Rows = 1
	}
	if err != nil {
		return false, stats, requestDBError("check pending student data change request", err)
	}
	return exists, stats, nil
}

func (s *requestStore) CreateStudentDataRequest(ctx context.Context, value careplan.StudentDataChangeRequest) (careplan.StudentDataChangeRequest, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "create a student data request")
	if err != nil {
		return careplan.StudentDataChangeRequest{}, carePlanCompose.RequestStoreStats{}, err
	}
	row := studentDataRequestFromPublic(value)
	row.TenantID = tenantID
	started := time.Now()
	err = db.NewInsert().Model(row).ModelTableExpr("users.student_data_change_requests").Returning("*").Scan(ctx)
	stats := requestStats(started, 1)
	if err != nil {
		stats.Rows = 0
		if isUniqueViolation(err) {
			stats.Conflicts = 1
		}
		return careplan.StudentDataChangeRequest{}, stats, requestDBError("create student data change request", err)
	}
	return studentDataRequestToPublic(row), stats, nil
}

func (s *requestStore) UpdatePendingStudentDataRequest(ctx context.Context, id int64, value json.RawMessage) (carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "update a student data request")
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*studentDataRequestRow)(nil)).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`).
		Set("new_value = ?", value).Set("updated_at = ?", time.Now()).
		Where(`"student_data_change_request".id = ?`, id).Where(`"student_data_change_request".status = ?`, requestPending), "student_data_change_request", tenantID)
	return execRequest(ctx, query, "update pending student data change request", careplan.ErrStudentDataRequestNotPending)
}

func (s *requestStore) DecideStudentDataRequest(ctx context.Context, value careplan.StudentDataRequestDecision, correction bool) (carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "decide a student data request")
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	now := time.Now()
	query := db.NewUpdate().Model((*studentDataRequestRow)(nil)).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`).
		Set("status = ?", value.Status).Set("review_reason = ?", value.Reason).
		Set("reviewed_at = ?", now).Set("updated_at = ?", now).
		Where(`"student_data_change_request".id = ?`, value.ID)
	if value.ReviewedBy > 0 {
		query = query.Set("reviewed_by = ?", value.ReviewedBy)
	} else {
		query = query.Set("reviewed_by = NULL")
	}
	if correction {
		query = query.Where(`"student_data_change_request".status IN (?)`, bun.List([]string{requestApproved, requestRejected}))
	} else {
		query = query.Where(`"student_data_change_request".status = ?`, requestPending)
	}
	if value.Applied {
		query = query.Set("applied_at = ?", now)
	} else if correction {
		query = query.Set("applied_at = NULL")
	}
	query = withTenant(query, "student_data_change_request", tenantID)
	if correction {
		return execRequest(ctx, query, "correct student data change request", careplan.ErrStudentDataRequestNotDecided)
	}
	return execRequest(ctx, query, "decide student data change request", careplan.ErrStudentDataRequestNotPending)
}

func execRequest(ctx context.Context, query interface {
	Exec(context.Context, ...any) (sql.Result, error)
}, op string, noRows error) (carePlanCompose.RequestStoreStats, error) {
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := requestStats(started, 0)
	if err != nil {
		return stats, requestDBError(op, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, requestDBError(op, err)
	}
	stats.Rows = rows
	if rows == 0 {
		return stats, noRows
	}
	return stats, nil
}

func excusedRequestFromPublic(value careplan.ExcusedAbsenceRequest) *excusedAbsenceRequestRow {
	dates := make([]calendarDate, len(value.Dates))
	for i := range value.Dates {
		dates[i] = calendarDate(value.Dates[i])
	}
	row := &excusedAbsenceRequestRow{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Dates: dates, Note: value.Note, AbsenceStatus: value.AbsenceStatus, Status: value.Status, DecisionReason: value.DecisionReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func excusedRequestToPublic(row *excusedAbsenceRequestRow) careplan.ExcusedAbsenceRequest {
	dates := make([]careplan.Date, len(row.Dates))
	for i := range row.Dates {
		dates[i] = careplan.Date(row.Dates[i])
	}
	return careplan.ExcusedAbsenceRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Dates: dates, Note: row.Note, AbsenceStatus: row.AbsenceStatus, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}

func excusedRequestsToPublic(rows []excusedAbsenceRequestRow) []careplan.ExcusedAbsenceRequest {
	result := make([]careplan.ExcusedAbsenceRequest, 0, len(rows))
	for i := range rows {
		result = append(result, excusedRequestToPublic(&rows[i]))
	}
	return result
}

func careScheduleRequestFromPublic(value careplan.CareScheduleChangeRequest) *careScheduleRequestRow {
	row := &careScheduleRequestRow{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, RequestKind: value.RequestKind, Payload: value.Payload, Status: value.Status, DecisionReason: value.DecisionReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt, DecisionSnapshot: value.DecisionSnapshot}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func careScheduleRequestToPublic(row *careScheduleRequestRow) careplan.CareScheduleChangeRequest {
	return careplan.CareScheduleChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, RequestKind: row.RequestKind, Payload: row.Payload, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt, DecisionSnapshot: row.DecisionSnapshot}
}

func careScheduleRequestsToPublic(rows []careScheduleRequestRow) []careplan.CareScheduleChangeRequest {
	result := make([]careplan.CareScheduleChangeRequest, 0, len(rows))
	for i := range rows {
		result = append(result, careScheduleRequestToPublic(&rows[i]))
	}
	return result
}

func studentDataRequestFromPublic(value careplan.StudentDataChangeRequest) *studentDataRequestRow {
	row := &studentDataRequestRow{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Target: value.Target, TargetRefID: value.TargetRefID, FieldKey: value.FieldKey, OldValue: value.OldValue, NewValue: value.NewValue, Status: value.Status, ReviewReason: value.ReviewReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func studentDataRequestToPublic(row *studentDataRequestRow) careplan.StudentDataChangeRequest {
	return careplan.StudentDataChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Target: row.Target, TargetRefID: row.TargetRefID, FieldKey: row.FieldKey, OldValue: row.OldValue, NewValue: row.NewValue, Status: row.Status, ReviewReason: row.ReviewReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}

func studentDataRequestsToPublic(rows []studentDataRequestRow) []careplan.StudentDataChangeRequest {
	result := make([]careplan.StudentDataChangeRequest, 0, len(rows))
	for i := range rows {
		result = append(result, studentDataRequestToPublic(&rows[i]))
	}
	return result
}
