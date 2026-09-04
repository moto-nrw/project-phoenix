package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

const excusedRequestLockClass int32 = 0x65786162

type requestStore struct{ db *bun.DB }

func NewRequestStore(db *bun.DB) *requestStore {
	if db == nil {
		panic("care plan request store: database is required")
	}
	return &requestStore{db: db}
}

func (s *requestStore) CountOpenCareRequests(ctx context.Context, studentIDs []int64) (map[int64]int, carePlanCompose.RequestStoreStats, error) {
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
		tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().TableExpr(`users.student_data_change_requests AS "request"`).ColumnExpr(`"request".student_id, COUNT(*)::int AS total`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, userModels.DataChangeStatusPending).GroupExpr(`"request".student_id`), "request"),
		tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().TableExpr(`active.excused_absence_requests AS "request"`).ColumnExpr(`"request".student_id, COUNT(*)::int AS total`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, activeModels.ExcusedRequestStatusPending).GroupExpr(`"request".student_id`), "request"),
		tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().TableExpr(`schedule.care_schedule_change_requests AS "request"`).ColumnExpr(`"request".student_id, COUNT(*)::int AS total`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, scheduleModels.CareRequestStatusPending).GroupExpr(`"request".student_id`), "request"),
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
		tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().TableExpr(`users.student_data_change_requests AS "request"`).ColumnExpr(`"request".id`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, userModels.DataChangeStatusPending).OrderExpr(`"request".id ASC`).For("UPDATE"), "request"),
		tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().TableExpr(`active.excused_absence_requests AS "request"`).ColumnExpr(`"request".id`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, activeModels.ExcusedRequestStatusPending).OrderExpr(`"request".id ASC`).For("UPDATE"), "request"),
		tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().TableExpr(`schedule.care_schedule_change_requests AS "request"`).ColumnExpr(`"request".id`).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, scheduleModels.CareRequestStatusPending).OrderExpr(`"request".id ASC`).For("UPDATE"), "request"),
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
		tenantQuery(ctx, base.GetDB(ctx, s.db).NewUpdate().TableExpr(`users.student_data_change_requests AS "request"`).Set(`status = ?`, userModels.DataChangeStatusCareEnded).Set(`review_reason = ?`, reason).Set(`reviewed_by = ?`, reviewedBy).Set(`reviewed_at = ?`, at).Set(`updated_at = ?`, at).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, userModels.DataChangeStatusPending), "request"),
		tenantQuery(ctx, base.GetDB(ctx, s.db).NewUpdate().TableExpr(`active.excused_absence_requests AS "request"`).Set(`status = ?`, activeModels.ExcusedRequestStatusCareEnded).Set(`decision_reason = ?`, reason).Set(`reviewed_by = ?`, reviewedBy).Set(`reviewed_at = ?`, at).Set(`updated_at = ?`, at).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, activeModels.ExcusedRequestStatusPending), "request"),
		tenantQuery(ctx, base.GetDB(ctx, s.db).NewUpdate().TableExpr(`schedule.care_schedule_change_requests AS "request"`).Set(`status = ?`, scheduleModels.CareRequestStatusCareEnded).Set(`decision_reason = ?`, reason).Set(`reviewed_by = ?`, reviewedBy).Set(`reviewed_at = ?`, at).Set(`updated_at = ?`, at).Where(`"request".student_id IN (?)`, bun.List(studentIDs)).Where(`"request".status = ?`, scheduleModels.CareRequestStatusPending), "request"),
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
	return &modelBase.DatabaseError{Op: op, Err: base.TranslateNotFound(err)}
}

func tenantQuery[Q interface{ Where(string, ...any) Q }](ctx context.Context, query Q, alias string) Q {
	if where, value, ok := base.TenantWhere(ctx, alias); ok {
		return query.Where(where, value)
	}
	return query
}

func queueFilter(value *careplan.RequestQueueFilter) modelBase.RequestQueueFilters {
	if value == nil {
		return modelBase.RequestQueueFilters{}
	}
	return modelBase.RequestQueueFilters{
		UrgentOnly: value.UrgentOnly, UrgentDate: value.UrgentDate,
		StudentIDs: value.StudentIDs, StudentID: value.StudentID, Search: value.Search,
		BeforeInstant: value.BeforeInstant, BeforeID: value.BeforeID, Limit: value.Limit,
	}
}

func legacyQueryOptions(value *careplan.StudentScheduleQueryOptions) *modelBase.QueryOptions {
	if value == nil {
		return nil
	}
	result := &modelBase.QueryOptions{}
	if value.Filter != nil {
		result.Filter = legacyQueryFilter(value.Filter)
	}
	if value.Limit > 0 {
		page := value.Offset/value.Limit + 1
		result.Pagination = &modelBase.Pagination{Page: page, PageSize: value.Limit}
	}
	if len(value.Sorting) > 0 {
		result.Sorting = &modelBase.Sorting{Fields: make([]modelBase.SortField, 0, len(value.Sorting))}
		for _, field := range value.Sorting {
			direction := modelBase.SortAsc
			if field.Descending {
				direction = modelBase.SortDesc
			}
			result.Sorting.Fields = append(result.Sorting.Fields, modelBase.SortField{Field: field.Field, Direction: direction})
		}
	}
	return result
}

func legacyQueryFilter(value *careplan.StudentScheduleQueryFilter) *modelBase.Filter {
	result := modelBase.NewFilter()
	for _, condition := range value.Conditions {
		result.Where(condition.Field, modelBase.Operator(condition.Operator), condition.Value)
	}
	for i := range value.Or {
		result.Or(*legacyQueryFilter(&value.Or[i]))
	}
	for i := range value.And {
		result.And(*legacyQueryFilter(&value.And[i]))
	}
	return result
}

func (s *requestStore) FindExcusedAbsenceRequest(ctx context.Context, id int64, lock bool) (careplan.ExcusedAbsenceRequest, bool, carePlanCompose.RequestStoreStats, error) {
	row := new(activeModels.ExcusedAbsenceRequest)
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model(row).
		ModelTableExpr(`active.excused_absence_requests AS "excused_absence_request"`).
		Where(`"excused_absence_request".id = ?`, id), "excused_absence_request")
	if lock {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err := query.Scan(ctx)
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
	if row.Status != activeModels.ExcusedRequestStatusPending {
		return row, stats, careplan.ErrExcusedRequestNotPending
	}
	return row, stats, nil
}

func (s *requestStore) ListExcusedAbsenceRequests(ctx context.Context, filter careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, carePlanCompose.RequestStoreStats, error) {
	rows := []*activeModels.ExcusedAbsenceRequest{}
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model(&rows).
		ModelTableExpr(`active.excused_absence_requests AS "excused_absence_request"`), "excused_absence_request")
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
	err := query.Scan(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return nil, stats, requestDBError("list excused absence requests", err)
	}
	return excusedRequestsToPublic(rows), stats, nil
}

func applyExcusedRequestOptions(query *bun.SelectQuery, filter careplan.ExcusedAbsenceRequestFilter) *bun.SelectQuery {
	if filter.Queue != nil {
		queue := queueFilter(filter.Queue)
		if len(filter.Statuses) == 1 && filter.Statuses[0] == activeModels.ExcusedRequestStatusPending {
			query = base.ApplyRequestUrgency(query, queue, `"excused_absence_request".dates @> ?::jsonb`, `["`+queue.UrgentDate+`"]`)
			return base.ApplyRequestQueueFilters(query, "excused_absence_request", "created_at", queue)
		}
		return base.ApplyRequestQueueFilters(query, "excused_absence_request", "updated_at", queue)
	}
	if options := legacyQueryOptions(filter.Options); options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("excused_absence_request")
			query = base.ApplyFilter(query, options.Filter)
		}
		if options.Pagination != nil {
			query = base.ApplyPagination(query, *options.Pagination)
		}
		if options.Sorting != nil {
			return base.ApplySorting(query, *options.Sorting)
		}
	}
	return query.OrderExpr(`"excused_absence_request".created_at DESC`).OrderExpr(`"excused_absence_request".id DESC`)
}

func (s *requestStore) CreateExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceRequest) (careplan.ExcusedAbsenceRequest, carePlanCompose.RequestStoreStats, error) {
	row := excusedRequestFromPublic(value)
	base.EnsureTenantID(ctx, row)
	started := time.Now()
	err := base.GetDB(ctx, s.db).NewInsert().Model(row).ModelTableExpr("active.excused_absence_requests").Returning("*").Scan(ctx)
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
	started := time.Now()
	_, err := base.GetDB(ctx, s.db).NewRaw("SELECT pg_advisory_xact_lock(?, ?)", excusedRequestLockClass, int32(studentID)).Exec(ctx)
	stats := requestStats(started, 0)
	if err != nil {
		return stats, requestDBError("lock student excused requests", err)
	}
	return stats, nil
}

func (s *requestStore) UpdatePendingExcusedAbsenceRequest(ctx context.Context, id int64, dates []careplan.Date, note, absenceStatus string) (carePlanCompose.RequestStoreStats, error) {
	legacyDates := make([]timezone.Date, len(dates))
	for i := range dates {
		legacyDates[i] = timezone.Date(dates[i])
	}
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewUpdate().Model((*activeModels.ExcusedAbsenceRequest)(nil)).
		ModelTableExpr(`active.excused_absence_requests AS "excused_absence_request"`).
		Set("dates = ?", legacyDates).Set("note = ?", note).Set("absence_status = ?", absenceStatus).Set("updated_at = ?", time.Now()).
		Where(`"excused_absence_request".id = ?`, id).Where(`"excused_absence_request".status = ?`, activeModels.ExcusedRequestStatusPending), "excused_absence_request")
	return execRequest(ctx, query, "update pending excused absence request", careplan.ErrExcusedRequestNotPending)
}

func (s *requestStore) DecideExcusedAbsenceRequest(ctx context.Context, value careplan.ExcusedAbsenceDecision, correction bool) (carePlanCompose.RequestStoreStats, error) {
	now := time.Now()
	query := base.GetDB(ctx, s.db).NewUpdate().Model((*activeModels.ExcusedAbsenceRequest)(nil)).
		ModelTableExpr(`active.excused_absence_requests AS "excused_absence_request"`).
		Set("status = ?", value.Status).Set("decision_reason = ?", value.Reason).Set("updated_at = ?", now).
		Where(`"excused_absence_request".id = ?`, value.ID)
	if correction {
		query = query.Set("reviewed_by = ?", reviewerID(value.ReviewedBy)).Set("reviewed_at = ?", now).
			Where(`"excused_absence_request".status IN (?)`, bun.List([]string{activeModels.ExcusedRequestStatusApproved, activeModels.ExcusedRequestStatusRejected}))
	} else {
		query = query.Where(`"excused_absence_request".status = ?`, activeModels.ExcusedRequestStatusPending)
		if value.ReviewedBy != nil && *value.ReviewedBy > 0 {
			query = query.Set("reviewed_by = ?", *value.ReviewedBy).Set("reviewed_at = ?", now)
		}
	}
	if value.Applied {
		query = query.Set("applied_at = ?", now)
	} else if correction {
		query = query.Set("applied_at = NULL")
	}
	query = tenantQuery(ctx, query, "excused_absence_request")
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
	row := new(scheduleModels.CareScheduleChangeRequest)
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model(row).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`).
		Where(`"care_schedule_change_request".id = ?`, id), "care_schedule_change_request")
	if lock {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err := query.Scan(ctx)
	stats := requestStats(started, 1)
	if errors.Is(err, sql.ErrNoRows) {
		stats.Rows = 0
		return careplan.CareScheduleChangeRequest{}, false, stats, nil
	}
	if err != nil {
		stats.Rows = 0
		return careplan.CareScheduleChangeRequest{}, false, stats, requestDBError("find care schedule change request", err)
	}
	value, err := careScheduleRequestToPublic(row)
	if err != nil {
		return careplan.CareScheduleChangeRequest{}, false, stats, err
	}
	return value, true, stats, nil
}

func (s *requestStore) FindPendingCareScheduleRequest(ctx context.Context, id int64) (careplan.CareScheduleChangeRequest, carePlanCompose.RequestStoreStats, error) {
	row, found, stats, err := s.FindCareScheduleRequest(ctx, id, true)
	if err != nil {
		return row, stats, err
	}
	if !found {
		return row, stats, careplan.ErrCareScheduleRequestNotFound
	}
	if row.Status != scheduleModels.CareRequestStatusPending {
		return row, stats, careplan.ErrCareScheduleRequestNotPending
	}
	return row, stats, nil
}

func (s *requestStore) ListCareScheduleRequests(ctx context.Context, filter careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, carePlanCompose.RequestStoreStats, error) {
	rows := []*scheduleModels.CareScheduleChangeRequest{}
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model(&rows).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`), "care_schedule_change_request")
	if filter.StudentID > 0 {
		query = query.Where(`"care_schedule_change_request".student_id = ?`, filter.StudentID)
	}
	if len(filter.RequestKinds) > 0 {
		query = query.Where(`"care_schedule_change_request".request_kind IN (?)`, bun.List(filter.RequestKinds))
	}
	if len(filter.Statuses) > 0 {
		query = query.Where(`"care_schedule_change_request".status IN (?)`, bun.List(filter.Statuses))
	}
	query, err := applyCareScheduleRequestOptions(query, filter)
	if err != nil {
		return nil, carePlanCompose.RequestStoreStats{}, err
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return nil, stats, requestDBError("list care schedule change requests", err)
	}
	values, err := careScheduleRequestsToPublic(rows)
	if err != nil {
		return nil, stats, err
	}
	return values, stats, nil
}

func applyCareScheduleRequestOptions(query *bun.SelectQuery, filter careplan.CareScheduleRequestFilter) (*bun.SelectQuery, error) {
	if !filter.RecentSince.IsZero() {
		return query.Where(`("care_schedule_change_request".status = ? OR "care_schedule_change_request".updated_at >= ?)`, scheduleModels.CareRequestStatusPending, filter.RecentSince).
			OrderExpr(`"care_schedule_change_request".created_at DESC`).OrderExpr(`"care_schedule_change_request".id DESC`), nil
	}
	if filter.Queue == nil {
		return query.OrderExpr(`"care_schedule_change_request".created_at DESC`).OrderExpr(`"care_schedule_change_request".id DESC`), nil
	}
	queue := queueFilter(filter.Queue)
	if len(filter.Statuses) == 1 && filter.Statuses[0] == scheduleModels.CareRequestStatusPending {
		if queue.UrgentOnly == nil {
			return base.ApplyRequestQueueFilters(query, "care_schedule_change_request", "created_at", queue), nil
		}
		date, err := time.Parse(time.DateOnly, queue.UrgentDate)
		if err != nil {
			return nil, fmt.Errorf("parse care schedule request urgency date: %w", err)
		}
		weekday := int(date.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		weekdayJSON := fmt.Sprintf(`[{"weekday":%d}]`, weekday)
		query = base.ApplyRequestUrgency(query, queue, `
			("care_schedule_change_request".request_kind = 'pickup_change' AND "care_schedule_change_request".payload->>'date' = ?)
			OR ("care_schedule_change_request".request_kind <> 'pickup_change' AND "care_schedule_change_request".payload->'weekdays' @> ?::jsonb)
		`, queue.UrgentDate, weekdayJSON)
		return base.ApplyRequestQueueFilters(query, "care_schedule_change_request", "created_at", queue), nil
	}
	return base.ApplyRequestQueueFilters(query, "care_schedule_change_request", "updated_at", queue), nil
}

func (s *requestStore) CreateCareScheduleRequest(ctx context.Context, value careplan.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, carePlanCompose.RequestStoreStats, error) {
	row, err := careScheduleRequestFromPublic(value)
	if err != nil {
		return careplan.CareScheduleChangeRequest{}, carePlanCompose.RequestStoreStats{}, err
	}
	base.EnsureTenantID(ctx, row)
	started := time.Now()
	err = base.GetDB(ctx, s.db).NewInsert().Model(row).ModelTableExpr("schedule.care_schedule_change_requests").Returning("*").Scan(ctx)
	stats := requestStats(started, 1)
	if err != nil {
		stats.Rows = 0
		if isUniqueViolation(err) {
			stats.Conflicts = 1
		}
		return careplan.CareScheduleChangeRequest{}, stats, requestDBError("create care schedule change request", err)
	}
	created, err := careScheduleRequestToPublic(row)
	if err != nil {
		return careplan.CareScheduleChangeRequest{}, stats, err
	}
	return created, stats, nil
}

func (s *requestStore) UpdatePendingCareScheduleRequest(ctx context.Context, id int64, payload json.RawMessage) (carePlanCompose.RequestStoreStats, error) {
	decoded, err := decodeCareSchedulePayload(payload)
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewUpdate().Model((*scheduleModels.CareScheduleChangeRequest)(nil)).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`).
		Set("payload = ?", decoded).Set("updated_at = ?", time.Now()).
		Where(`"care_schedule_change_request".id = ?`, id).Where(`"care_schedule_change_request".status = ?`, scheduleModels.CareRequestStatusPending), "care_schedule_change_request")
	return execRequest(ctx, query, "update pending care schedule change request", careplan.ErrCareScheduleRequestNotPending)
}

func (s *requestStore) DecideCareScheduleRequest(ctx context.Context, value careplan.CareScheduleRequestDecision, correction bool) (carePlanCompose.RequestStoreStats, error) {
	now := time.Now()
	query := base.GetDB(ctx, s.db).NewUpdate().Model((*scheduleModels.CareScheduleChangeRequest)(nil)).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`).
		Set("status = ?", value.Status).Set("decision_reason = ?", value.Reason).Set("updated_at = ?", now).
		Where(`"care_schedule_change_request".id = ?`, value.ID)
	if correction {
		query = query.Set("reviewed_by = ?", reviewerID(value.ReviewedBy)).Set("reviewed_at = ?", now).
			Where(`"care_schedule_change_request".status IN (?)`, bun.List([]string{scheduleModels.CareRequestStatusApproved, scheduleModels.CareRequestStatusRejected}))
	} else {
		query = query.Where(`"care_schedule_change_request".status = ?`, scheduleModels.CareRequestStatusPending)
		if value.ReviewedBy != nil && *value.ReviewedBy > 0 {
			query = query.Set("reviewed_by = ?", *value.ReviewedBy).Set("reviewed_at = ?", now)
		}
	}
	if value.Applied {
		query = query.Set("applied_at = ?", now)
	} else if correction {
		query = query.Set("applied_at = NULL")
	}
	query = tenantQuery(ctx, query, "care_schedule_change_request")
	if correction {
		return execRequest(ctx, query, "correct care schedule change request", careplan.ErrCareScheduleRequestNotDecided)
	}
	return execRequest(ctx, query, "decide care schedule change request", careplan.ErrCareScheduleRequestNotPending)
}

func (s *requestStore) UpdateCareScheduleRequestSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) (carePlanCompose.RequestStoreStats, error) {
	var legacySnapshot *scheduleModels.CareRequestDecisionSnapshot
	if len(snapshot) > 0 && string(snapshot) != "null" {
		legacySnapshot = new(scheduleModels.CareRequestDecisionSnapshot)
		if err := json.Unmarshal(snapshot, legacySnapshot); err != nil {
			return carePlanCompose.RequestStoreStats{}, err
		}
	}
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewUpdate().Model((*scheduleModels.CareScheduleChangeRequest)(nil)).
		ModelTableExpr(`schedule.care_schedule_change_requests AS "care_schedule_change_request"`).
		Set("decision_snapshot = ?", legacySnapshot).Set("updated_at = ?", time.Now()).
		Where(`"care_schedule_change_request".id = ?`, id), "care_schedule_change_request")
	return execRequest(ctx, query, "update care request decision snapshot", careplan.ErrCareScheduleRequestNotFound)
}

func (s *requestStore) FindStudentDataRequest(ctx context.Context, id int64, lock bool) (careplan.StudentDataChangeRequest, bool, carePlanCompose.RequestStoreStats, error) {
	row := new(userModels.StudentDataChangeRequest)
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model(row).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`).
		Where(`"student_data_change_request".id = ?`, id), "student_data_change_request")
	if lock {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err := query.Scan(ctx)
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
	if row.Status != userModels.DataChangeStatusPending {
		return row, stats, careplan.ErrStudentDataRequestNotPending
	}
	return row, stats, nil
}

func (s *requestStore) ListStudentDataRequests(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]careplan.StudentDataChangeRequest, carePlanCompose.RequestStoreStats, error) {
	rows := []*userModels.StudentDataChangeRequest{}
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model(&rows).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`), "student_data_change_request")
	if filter.StudentID > 0 {
		query = query.Where(`"student_data_change_request".student_id = ?`, filter.StudentID)
	}
	if len(filter.Statuses) > 0 {
		query = query.Where(`"student_data_change_request".status IN (?)`, bun.List(filter.Statuses))
	}
	if filter.ParentVisible {
		query = query.Where(`"student_data_change_request".target IN (?)`, bun.List([]string{userModels.DataChangeTargetPerson, userModels.DataChangeTargetDeparture}))
	}
	if filter.Queue != nil {
		queue := queueFilter(filter.Queue)
		if len(filter.Statuses) == 1 && filter.Statuses[0] == userModels.DataChangeStatusPending {
			query = base.ApplyRequestUrgency(query, queue, "FALSE")
			query = base.ApplyRequestQueueFilters(query, "student_data_change_request", "created_at", queue)
		} else {
			query = base.ApplyRequestQueueFilters(query, "student_data_change_request", "updated_at", queue)
		}
	} else {
		query = query.OrderExpr(`"student_data_change_request".created_at DESC`).OrderExpr(`"student_data_change_request".id DESC`)
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
	}
	started := time.Now()
	err := query.Scan(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return nil, stats, requestDBError("list student data change requests", err)
	}
	return studentDataRequestsToPublic(rows), stats, nil
}

func (s *requestStore) HasPendingStudentDataRequest(ctx context.Context, studentID int64, target, field string) (bool, carePlanCompose.RequestStoreStats, error) {
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model((*userModels.StudentDataChangeRequest)(nil)).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`).
		Where(`"student_data_change_request".student_id = ?`, studentID).
		Where(`"student_data_change_request".target = ?`, target).
		Where(`"student_data_change_request".field_key = ?`, field).
		Where(`"student_data_change_request".status = ?`, userModels.DataChangeStatusPending), "student_data_change_request")
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
	row := studentDataRequestFromPublic(value)
	base.EnsureTenantID(ctx, row)
	started := time.Now()
	err := base.GetDB(ctx, s.db).NewInsert().Model(row).ModelTableExpr("users.student_data_change_requests").Returning("*").Scan(ctx)
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
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewUpdate().Model((*userModels.StudentDataChangeRequest)(nil)).
		ModelTableExpr(`users.student_data_change_requests AS "student_data_change_request"`).
		Set("new_value = ?", value).Set("updated_at = ?", time.Now()).
		Where(`"student_data_change_request".id = ?`, id).Where(`"student_data_change_request".status = ?`, userModels.DataChangeStatusPending), "student_data_change_request")
	return execRequest(ctx, query, "update pending student data change request", careplan.ErrStudentDataRequestNotPending)
}

func (s *requestStore) DecideStudentDataRequest(ctx context.Context, value careplan.StudentDataRequestDecision, correction bool) (carePlanCompose.RequestStoreStats, error) {
	now := time.Now()
	query := base.GetDB(ctx, s.db).NewUpdate().Model((*userModels.StudentDataChangeRequest)(nil)).
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
		query = query.Where(`"student_data_change_request".status IN (?)`, bun.List([]string{userModels.DataChangeStatusApproved, userModels.DataChangeStatusRejected}))
	} else {
		query = query.Where(`"student_data_change_request".status = ?`, userModels.DataChangeStatusPending)
	}
	if value.Applied {
		query = query.Set("applied_at = ?", now)
	} else if correction {
		query = query.Set("applied_at = NULL")
	}
	query = tenantQuery(ctx, query, "student_data_change_request")
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

func excusedRequestFromPublic(value careplan.ExcusedAbsenceRequest) *activeModels.ExcusedAbsenceRequest {
	dates := make([]timezone.Date, len(value.Dates))
	for i := range value.Dates {
		dates[i] = timezone.Date(value.Dates[i])
	}
	row := &activeModels.ExcusedAbsenceRequest{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Dates: dates, Note: value.Note, AbsenceStatus: value.AbsenceStatus, Status: value.Status, DecisionReason: value.DecisionReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func excusedRequestToPublic(row *activeModels.ExcusedAbsenceRequest) careplan.ExcusedAbsenceRequest {
	dates := make([]careplan.Date, len(row.Dates))
	for i := range row.Dates {
		dates[i] = careplan.Date(row.Dates[i])
	}
	return careplan.ExcusedAbsenceRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Dates: dates, Note: row.Note, AbsenceStatus: row.AbsenceStatus, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}

func excusedRequestsToPublic(rows []*activeModels.ExcusedAbsenceRequest) []careplan.ExcusedAbsenceRequest {
	result := make([]careplan.ExcusedAbsenceRequest, 0, len(rows))
	for _, row := range rows {
		result = append(result, excusedRequestToPublic(row))
	}
	return result
}

func careScheduleRequestFromPublic(value careplan.CareScheduleChangeRequest) (*scheduleModels.CareScheduleChangeRequest, error) {
	payload, err := decodeCareSchedulePayload(value.Payload)
	if err != nil {
		return nil, err
	}
	var snapshot *scheduleModels.CareRequestDecisionSnapshot
	if len(value.DecisionSnapshot) > 0 && string(value.DecisionSnapshot) != "null" {
		snapshot = new(scheduleModels.CareRequestDecisionSnapshot)
		if err := json.Unmarshal(value.DecisionSnapshot, snapshot); err != nil {
			return nil, err
		}
	}
	row := &scheduleModels.CareScheduleChangeRequest{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, RequestKind: value.RequestKind, Payload: payload, Status: value.Status, DecisionReason: value.DecisionReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt, DecisionSnapshot: snapshot}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row, nil
}

func careScheduleRequestToPublic(row *scheduleModels.CareScheduleChangeRequest) (careplan.CareScheduleChangeRequest, error) {
	var snapshot json.RawMessage
	if row.DecisionSnapshot != nil {
		var err error
		snapshot, err = json.Marshal(row.DecisionSnapshot)
		if err != nil {
			return careplan.CareScheduleChangeRequest{}, fmt.Errorf("encode care schedule request decision snapshot: %w", err)
		}
	}
	payload, err := json.Marshal(row.Payload)
	if err != nil {
		return careplan.CareScheduleChangeRequest{}, fmt.Errorf("encode care schedule request payload: %w", err)
	}
	return careplan.CareScheduleChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, RequestKind: row.RequestKind, Payload: payload, Status: row.Status, DecisionReason: row.DecisionReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt, DecisionSnapshot: snapshot}, nil
}

func decodeCareSchedulePayload(payload json.RawMessage) (map[string]any, error) {
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("decode care schedule request payload: %w", err)
	}
	return decoded, nil
}

func careScheduleRequestsToPublic(rows []*scheduleModels.CareScheduleChangeRequest) ([]careplan.CareScheduleChangeRequest, error) {
	result := make([]careplan.CareScheduleChangeRequest, 0, len(rows))
	for _, row := range rows {
		value, err := careScheduleRequestToPublic(row)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func studentDataRequestFromPublic(value careplan.StudentDataChangeRequest) *userModels.StudentDataChangeRequest {
	row := &userModels.StudentDataChangeRequest{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Target: value.Target, TargetRefID: value.TargetRefID, FieldKey: value.FieldKey, OldValue: value.OldValue, NewValue: value.NewValue, Status: value.Status, ReviewReason: value.ReviewReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func studentDataRequestToPublic(row *userModels.StudentDataChangeRequest) careplan.StudentDataChangeRequest {
	return careplan.StudentDataChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Target: row.Target, TargetRefID: row.TargetRefID, FieldKey: row.FieldKey, OldValue: row.OldValue, NewValue: row.NewValue, Status: row.Status, ReviewReason: row.ReviewReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}

func studentDataRequestsToPublic(rows []*userModels.StudentDataChangeRequest) []careplan.StudentDataChangeRequest {
	result := make([]careplan.StudentDataChangeRequest, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentDataRequestToPublic(row))
	}
	return result
}
