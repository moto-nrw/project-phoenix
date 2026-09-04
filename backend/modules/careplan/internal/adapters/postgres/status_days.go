package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type statusDayStore struct {
	db       *bun.DB
	students StatusStudentDirectory
	slots    scheduleModels.InstanceStudentRepository
}

type StatusStudentDirectory interface {
	ListEnrolledStudents(context.Context) ([]peopledirectory.Student, error)
	ListStudentsWithStatusFlag(context.Context, string) ([]peopledirectory.Student, error)
	ClearStudentStatusFlags(context.Context, []int64, string) (int64, error)
	LockStudent(context.Context, int64) error
}

func NewStatusDayStore(db *bun.DB, students StatusStudentDirectory) *statusDayStore {
	if db == nil || students == nil {
		panic("care plan status-day store: database and People Directory are required")
	}
	return &statusDayStore{db: db, students: students, slots: scheduleRepo.NewInstanceStudentRepository(db)}
}

func (s *statusDayStore) BindCarePlan(capability careplan.Capability) {
	if repository, ok := s.slots.(*scheduleRepo.InstanceStudentRepository); ok {
		repository.BindCarePlan(statusDayCarePlanDirectory{capability: capability})
	}
}

type statusDayCarePlanDirectory struct{ capability careplan.Capability }

func (d statusDayCarePlanDirectory) FindPickupException(ctx context.Context, id int64) (*scheduleRepo.PickupExceptionProjection, error) {
	value, err := d.capability.FindPickupException(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentStatusDayNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &scheduleRepo.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto}, nil
}

func (d statusDayCarePlanDirectory) ListPickupExceptions(ctx context.Context, filter scheduleRepo.PickupExceptionFilter) ([]scheduleRepo.PickupExceptionProjection, error) {
	ownerFilter := careplan.StudentScheduleFilter{IDs: filter.IDs, StudentIDs: filter.StudentIDs}
	if filter.Date != "" {
		ownerFilter.Date = careplan.Date(filter.Date)
	}
	if filter.From != "" {
		ownerFilter.From = careplan.Date(filter.From)
	}
	values, err := d.capability.ListPickupExceptions(ctx, ownerFilter)
	if err != nil {
		return nil, err
	}
	result := make([]scheduleRepo.PickupExceptionProjection, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleRepo.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto})
	}
	return result, nil
}

func (d statusDayCarePlanDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*scheduleRepo.StudentStatusDayProjection, error) {
	value, err := d.capability.FindStudentStatusDay(ctx, id, activeOnly)
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &scheduleRepo.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status}, nil
}

func (d statusDayCarePlanDirectory) ListStudentStatusDays(ctx context.Context, filter scheduleRepo.StudentStatusDayFilter) ([]scheduleRepo.StudentStatusDayProjection, error) {
	values, err := d.capability.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		IDs: filter.IDs, StudentIDs: filter.StudentIDs, Date: careplan.Date(filter.Date),
		From: careplan.Date(filter.From), ActiveOnly: filter.ActiveOnly, LatestOnly: filter.LatestOnly,
	})
	if err != nil {
		return nil, err
	}
	result := make([]scheduleRepo.StudentStatusDayProjection, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleRepo.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status})
	}
	return result, nil
}

func (s *statusDayStore) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (careplan.StudentStatusDay, bool, carePlanCompose.RequestStoreStats, error) {
	row := new(activeModels.StudentStatusDay)
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model(row).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Where(`"student_status_day".id = ?`, id), "student_status_day")
	if activeOnly {
		query = query.Where(`"student_status_day".cleared_at IS NULL`)
	}
	started := time.Now()
	err := query.Scan(ctx)
	stats := requestStats(started, 1)
	if errors.Is(err, sql.ErrNoRows) {
		stats.Rows = 0
		return careplan.StudentStatusDay{}, false, stats, nil
	}
	if err != nil {
		stats.Rows = 0
		return careplan.StudentStatusDay{}, false, stats, requestDBError("find student status day", err)
	}
	return statusDayToPublic(row), true, stats, nil
}

func (s *statusDayStore) ListStudentStatusDays(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, carePlanCompose.RequestStoreStats, error) {
	rows := []*activeModels.StudentStatusDay{}
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model(&rows).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).ColumnExpr(`"student_status_day".*`), "student_status_day")
	query = filterStatusDays(query, filter)
	query = orderStatusDays(query, filter)
	started := time.Now()
	err := query.Scan(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return nil, stats, requestDBError("list student status days", err)
	}
	return statusDaysToPublic(rows), stats, nil
}

func filterStatusDays(query *bun.SelectQuery, filter careplan.StudentStatusDayFilter) *bun.SelectQuery {
	if len(filter.IDs) > 0 {
		query = query.Where(`"student_status_day".id IN (?)`, bun.List(filter.IDs))
	}
	if len(filter.StudentIDs) > 0 {
		query = query.Where(`"student_status_day".student_id IN (?)`, bun.List(filter.StudentIDs))
	}
	if filter.Date != "" {
		query = query.Where(`"student_status_day".date = ?`, timezone.Date(filter.Date))
	}
	if filter.From != "" {
		query = query.Where(`"student_status_day".date >= ?`, timezone.Date(filter.From))
	}
	if filter.To != "" {
		query = query.Where(`"student_status_day".date <= ?`, timezone.Date(filter.To))
	}
	if filter.ActiveOnly {
		query = query.Where(`"student_status_day".cleared_at IS NULL`)
	} else if filter.IncludeEndOfDay {
		query = query.Where(`("student_status_day".cleared_at IS NULL OR "student_status_day".source = ?)`, activeModels.StudentStatusSourceEndOfDay)
	}
	if filter.LatestOnly {
		query = query.DistinctOn(`"student_status_day".student_id, "student_status_day".date`)
	}
	if options := legacyQueryOptions(filter.Options); options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("student_status_day")
			query = base.ApplyFilter(query, options.Filter)
		}
		if options.Pagination != nil {
			query = base.ApplyPagination(query, *options.Pagination)
		}
		if options.Sorting != nil {
			query = base.ApplySorting(query, *options.Sorting)
		}
	}
	return query
}

func orderStatusDays(query *bun.SelectQuery, filter careplan.StudentStatusDayFilter) *bun.SelectQuery {
	if filter.LatestOnly {
		return query.OrderExpr(`"student_status_day".student_id ASC`).OrderExpr(`"student_status_day".date ASC`).OrderExpr(`"student_status_day".reported_at DESC`).OrderExpr(`"student_status_day".id DESC`)
	}
	if filter.Overview && len(filter.OrderedStudentIDs) == 0 {
		return query.OrderExpr(`"student_status_day".date ASC`).OrderExpr(`"student_status_day".student_id ASC`).OrderExpr(`"student_status_day".reported_at DESC`).OrderExpr(`"student_status_day".id ASC`)
	}
	if len(filter.OrderedStudentIDs) > 0 {
		return query.OrderExpr(`"student_status_day".date ASC`).
			OrderExpr(`array_position(?::bigint[], "student_status_day".student_id) ASC`, pgdialect.Array(filter.OrderedStudentIDs)).
			OrderExpr(`"student_status_day".reported_at DESC`).OrderExpr(`"student_status_day".id ASC`)
	}
	return query.OrderExpr(`"student_status_day".date DESC`).OrderExpr(`"student_status_day".reported_at DESC`).OrderExpr(`"student_status_day".id DESC`)
}

func (s *statusDayStore) CountStudentStatusDays(ctx context.Context, options *careplan.StudentScheduleQueryOptions) (int, carePlanCompose.RequestStoreStats, error) {
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().Model((*activeModels.StudentStatusDay)(nil)).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`), "student_status_day")
	if legacy := legacyQueryOptions(options); legacy != nil && legacy.Filter != nil {
		legacy.Filter.WithTableAlias("student_status_day")
		query = base.ApplyFilter(query, legacy.Filter)
	}
	started := time.Now()
	count, err := query.Count(ctx)
	stats := requestStats(started, int64(count))
	if err != nil {
		return 0, stats, requestDBError("count student status days", err)
	}
	return count, stats, nil
}

func (s *statusDayStore) CountEffectiveStudentAbsences(ctx context.Context, date careplan.Date) (careplan.StudentStatusCounts, carePlanCompose.RequestStoreStats, error) {
	students, err := s.students.ListEnrolledStudents(ctx)
	if err != nil {
		return careplan.StudentStatusCounts{}, carePlanCompose.RequestStoreStats{}, err
	}
	rows, stats, err := s.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{Date: date, ActiveOnly: true})
	if err != nil {
		return careplan.StudentStatusCounts{}, stats, err
	}
	type flags struct{ sick, excused, classTrip bool }
	byStudent := make(map[int64]flags, len(rows))
	for _, row := range rows {
		value := byStudent[row.StudentID]
		switch row.Status {
		case activeModels.StudentStatusDaySick:
			value.sick = true
		case activeModels.StudentStatusDayExcused:
			value.excused = true
		case activeModels.StudentStatusDayClassTrip:
			value.classTrip = true
		}
		byStudent[row.StudentID] = value
	}
	counts := careplan.StudentStatusCounts{}
	for _, student := range students {
		if student.Status != string(userModels.StudentStatusActive) {
			continue
		}
		counts.Total++
		day := byStudent[student.ID]
		if (student.Sick != nil && *student.Sick) || day.sick {
			counts.Sick++
		} else if (student.Excused != nil && *student.Excused) || day.excused || day.classTrip {
			counts.Excused++
		}
	}
	return counts, stats, nil
}

func (s *statusDayStore) ListStatusDaySummaries(ctx context.Context, from, to careplan.Date) ([]careplan.StatusDaySummary, carePlanCompose.RequestStoreStats, error) {
	rows, stats, err := s.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{From: from, To: to, IncludeEndOfDay: true})
	if err != nil {
		return nil, stats, err
	}
	result := make([]careplan.StatusDaySummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, careplan.StatusDaySummary{StudentID: row.StudentID, Date: row.Date, Status: row.Status})
	}
	return result, stats, nil
}

func (s *statusDayStore) ExistingStudentStatusDayIDs(ctx context.Context, ids []int64) ([]int64, carePlanCompose.RequestStoreStats, error) {
	if len(ids) == 0 {
		return []int64{}, carePlanCompose.RequestStoreStats{}, nil
	}
	var result []int64
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().TableExpr(`active.student_status_days AS "student_status_day"`).
		ColumnExpr(`"student_status_day".id`).Where(`"student_status_day".id IN (?)`, bun.List(ids)), "student_status_day")
	started := time.Now()
	err := query.Scan(ctx, &result)
	stats := requestStats(started, int64(len(result)))
	if err != nil {
		return nil, stats, requestDBError("list existing student status day ids", err)
	}
	return result, stats, nil
}

func (s *statusDayStore) CountCarePlanDeletionRecords(ctx context.Context, studentID int64) (careplan.CarePlanDeletionCounts, carePlanCompose.RequestStoreStats, error) {
	counts := careplan.CarePlanDeletionCounts{}
	query := base.GetDB(ctx, s.db).NewRaw(`SELECT
		(SELECT COUNT(*) FROM active.student_status_days WHERE tenant_id = ? AND student_id = ?)::int AS status_days,
		(SELECT COUNT(*) FROM active.excused_absence_requests WHERE tenant_id = ? AND student_id = ?)::int AS excused_requests,
		(SELECT COUNT(*) FROM schedule.care_schedule_change_requests WHERE tenant_id = ? AND student_id = ?)::int AS care_requests,
		(SELECT COUNT(*) FROM users.student_data_change_requests WHERE tenant_id = ? AND student_id = ?)::int AS data_requests`,
		tenant.FromContext(ctx), studentID, tenant.FromContext(ctx), studentID,
		tenant.FromContext(ctx), studentID, tenant.FromContext(ctx), studentID)
	started := time.Now()
	err := query.Scan(ctx, &counts)
	stats := requestStats(started, 1)
	if err != nil {
		return counts, stats, requestDBError("count care plan deletion records", err)
	}
	return counts, stats, nil
}

func (s *statusDayStore) UpsertStudentStatusDay(ctx context.Context, value careplan.StudentStatusDay) (careplan.StudentStatusDay, carePlanCompose.RequestStoreStats, error) {
	row := statusDayFromPublic(value)
	base.EnsureTenantID(ctx, row)
	if row.Date.IsZero() {
		return careplan.StudentStatusDay{}, carePlanCompose.RequestStoreStats{}, errors.New("student status day date is required")
	}
	query := base.GetDB(ctx, s.db).NewInsert().Model(row).ModelTableExpr("active.student_status_days").
		On("CONFLICT (tenant_id, student_id, date, status) DO UPDATE").
		Set("reported_at = EXCLUDED.reported_at").Set("cleared_at = NULL").Set("source = EXCLUDED.source").
		Set(`note = CASE WHEN student_status_days.cleared_at IS NULL THEN COALESCE(EXCLUDED.note, student_status_days.note) ELSE EXCLUDED.note END`).
		Set(`guardian_account_id = CASE WHEN student_status_days.cleared_at IS NULL AND EXCLUDED.note IS NULL THEN student_status_days.guardian_account_id ELSE EXCLUDED.guardian_account_id END`).
		Returning("*")
	started := time.Now()
	err := query.Scan(ctx)
	stats := requestStats(started, 1)
	if err != nil {
		stats.Rows = 0
		return careplan.StudentStatusDay{}, stats, requestDBError("upsert student status day", err)
	}
	_, err = s.slots.ApplyStatusDay(ctx, row.StudentID, row.Date, row.ID, attendanceSubstatus(row.Status))
	if err != nil {
		return careplan.StudentStatusDay{}, stats, err
	}
	return statusDayToPublic(row), stats, nil
}

func attendanceSubstatus(status string) string {
	switch status {
	case activeModels.StudentStatusDaySick:
		return scheduleModels.AttendanceSubstatusSick
	case activeModels.StudentStatusDayExcused:
		return scheduleModels.AttendanceSubstatusExcused
	case activeModels.StudentStatusDayClassTrip:
		return scheduleModels.AttendanceSubstatusFieldTrip
	default:
		return scheduleModels.AttendanceSubstatusOther
	}
}

func (s *statusDayStore) ClearStudentStatusDays(ctx context.Context, studentID int64, status string, dates []careplan.Date, clearedAt time.Time, source string) (carePlanCompose.RequestStoreStats, error) {
	if len(dates) == 0 {
		return carePlanCompose.RequestStoreStats{}, nil
	}
	legacyDates := make([]timezone.Date, 0, len(dates))
	seen := make(map[timezone.Date]struct{}, len(dates))
	for _, value := range dates {
		date := timezone.Date(value)
		if _, ok := seen[date]; !ok {
			seen[date] = struct{}{}
			legacyDates = append(legacyDates, date)
		}
	}
	ids, stats, err := s.activeStatusDayIDs(ctx, studentID, status, legacyDates)
	if err != nil {
		return stats, err
	}
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewUpdate().Model((*activeModels.StudentStatusDay)(nil)).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).Set("cleared_at = ?", clearedAt).Set("source = ?", source).
		Where(`"student_status_day".student_id = ?`, studentID).Where(`"student_status_day".status = ?`, status).
		Where(`"student_status_day".date IN (?)`, bun.List(legacyDates)).Where(`"student_status_day".cleared_at IS NULL`), "student_status_day")
	writeStats, err := execRequest(ctx, query, "mark student status days cleared", errors.New("student status day was already cleared"))
	stats.Queries += writeStats.Queries
	stats.Rows += writeStats.Rows
	stats.StatementDuration += writeStats.StatementDuration
	if err != nil && writeStats.Rows == 0 {
		// Clearing an already-clear/nonexistent row was historically a no-op.
		err = nil
	}
	if err != nil {
		return stats, err
	}
	for _, id := range ids {
		if _, err := s.slots.ReleaseStatusDay(ctx, id); err != nil {
			return stats, err
		}
	}
	return stats, nil
}

func (s *statusDayStore) activeStatusDayIDs(ctx context.Context, studentID int64, status string, dates []timezone.Date) ([]int64, carePlanCompose.RequestStoreStats, error) {
	var ids []int64
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewSelect().TableExpr(`active.student_status_days AS "student_status_day"`).
		ColumnExpr(`"student_status_day".id`).Where(`"student_status_day".student_id = ?`, studentID).
		Where(`"student_status_day".status = ?`, status).Where(`"student_status_day".date IN (?)`, bun.List(dates)).
		Where(`"student_status_day".cleared_at IS NULL`), "student_status_day")
	started := time.Now()
	err := query.Scan(ctx, &ids)
	stats := requestStats(started, int64(len(ids)))
	if err != nil {
		return nil, stats, requestDBError("find active student status day ids", err)
	}
	return ids, stats, nil
}

func (s *statusDayStore) ClearStudentStatusDayByID(ctx context.Context, id int64, clearedAt time.Time, source string) (carePlanCompose.RequestStoreStats, error) {
	query := tenantQuery(ctx, base.GetDB(ctx, s.db).NewUpdate().Model((*activeModels.StudentStatusDay)(nil)).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).Set("cleared_at = ?", clearedAt).Set("source = ?", source).
		Where(`"student_status_day".id = ?`, id).Where(`"student_status_day".cleared_at IS NULL`), "student_status_day")
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := requestStats(started, 0)
	if err != nil {
		return stats, requestDBError("mark student status day cleared by id", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, requestDBError("mark student status day cleared by id", err)
	}
	if _, err := s.slots.ReleaseStatusDay(ctx, id); err != nil {
		return stats, err
	}
	return stats, nil
}

func (s *statusDayStore) ArchiveStudentStatusFlags(ctx context.Context, value careplan.StatusFlagArchive) (int64, carePlanCompose.RequestStoreStats, error) {
	if (value.Status == activeModels.StudentStatusDaySick && (value.FlagColumn != "sick" || value.SinceColumn != "sick_since")) ||
		(value.Status == activeModels.StudentStatusDayExcused && (value.FlagColumn != "excused" || value.SinceColumn != "excused_since")) {
		return 0, carePlanCompose.RequestStoreStats{}, fmt.Errorf("status flag columns do not match status %q", value.Status)
	}
	candidates, err := s.students.ListStudentsWithStatusFlag(ctx, value.Status)
	if err != nil || len(candidates) == 0 {
		return 0, carePlanCompose.RequestStoreStats{}, err
	}
	ids := make([]int64, 0, len(candidates))
	locked := make(map[int64]struct{}, len(candidates))
	for _, student := range candidates {
		ids = append(ids, student.ID)
		locked[student.ID] = struct{}{}
	}
	slices.Sort(ids)
	for _, studentID := range slices.Compact(ids) {
		if err := s.students.LockStudent(ctx, studentID); err != nil {
			return 0, carePlanCompose.RequestStoreStats{}, err
		}
	}
	students, err := s.students.ListStudentsWithStatusFlag(ctx, value.Status)
	if err != nil {
		return 0, carePlanCompose.RequestStoreStats{}, err
	}
	students = slices.DeleteFunc(students, func(student peopledirectory.Student) bool { _, ok := locked[student.ID]; return !ok })
	rows := make([]*activeModels.StudentStatusDay, 0, len(students))
	ids = ids[:0]
	for _, student := range students {
		reportedAt := value.ReportedFallback
		if value.Status == activeModels.StudentStatusDaySick && student.SickSince != nil {
			reportedAt = *student.SickSince
		}
		if value.Status == activeModels.StudentStatusDayExcused && student.ExcusedSince != nil {
			reportedAt = *student.ExcusedSince
		}
		row := &activeModels.StudentStatusDay{StudentID: student.ID, Date: timezone.Date(value.Date), Status: value.Status, ReportedAt: reportedAt, ClearedAt: &value.ReportedFallback, Source: value.Source}
		row.TenantID = student.TenantID
		rows = append(rows, row)
		ids = append(ids, student.ID)
	}
	if len(rows) == 0 {
		return 0, carePlanCompose.RequestStoreStats{}, nil
	}
	started := time.Now()
	_, err = base.GetDB(ctx, s.db).NewInsert().Model(&rows).ModelTableExpr("active.student_status_days").
		On("CONFLICT (tenant_id, student_id, date, status) DO UPDATE").Set("reported_at = EXCLUDED.reported_at").
		Set("cleared_at = EXCLUDED.cleared_at").Set("source = EXCLUDED.source").Exec(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return 0, stats, requestDBError("archive status flag", err)
	}
	affected, err := s.students.ClearStudentStatusFlags(ctx, ids, value.Status)
	return affected, stats, err
}

func statusDayFromPublic(value careplan.StudentStatusDay) *activeModels.StudentStatusDay {
	row := &activeModels.StudentStatusDay{StudentID: value.StudentID, Date: timezone.Date(value.Date), Status: value.Status, ReportedAt: value.ReportedAt, ClearedAt: value.ClearedAt, Source: value.Source, GuardianAccountID: value.GuardianAccountID, Note: value.Note}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func statusDayToPublic(row *activeModels.StudentStatusDay) careplan.StudentStatusDay {
	return careplan.StudentStatusDay{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, Date: careplan.Date(row.Date), Status: row.Status, ReportedAt: row.ReportedAt, ClearedAt: row.ClearedAt, Source: row.Source, GuardianAccountID: row.GuardianAccountID, Note: row.Note}
}

func statusDaysToPublic(rows []*activeModels.StudentStatusDay) []careplan.StudentStatusDay {
	result := make([]careplan.StudentStatusDay, 0, len(rows))
	for _, row := range rows {
		result = append(result, statusDayToPublic(row))
	}
	return result
}
