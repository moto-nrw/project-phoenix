package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type statusDayStore struct {
	*Store
	students StatusStudentDirectory
	slots    StatusSlotDirectory
}

type StatusStudent struct {
	ID           int64
	TenantID     int64
	Status       string
	Sick         *bool
	SickSince    *time.Time
	Excused      *bool
	ExcusedSince *time.Time
}

type StatusStudentDirectory interface {
	ListEnrolledStudents(context.Context) ([]StatusStudent, error)
	ListStudentsWithStatusFlag(context.Context, string) ([]StatusStudent, error)
	ClearStudentStatusFlags(context.Context, []int64, string) (int64, error)
	LockStudent(context.Context, int64) error
}

type StatusSlotDirectory interface {
	ApplyStatusDay(context.Context, int64, careplan.Date, int64, string) (int, error)
	ReleaseStatusDay(context.Context, int64) (int, error)
}

func NewStatusDayStore(store *Store, students StatusStudentDirectory, slots StatusSlotDirectory) *statusDayStore {
	if store == nil || students == nil || slots == nil {
		panic("care plan status-day store: store, student directory, and slot directory are required")
	}
	return &statusDayStore{Store: store, students: students, slots: slots}
}

type studentStatusDayRow struct {
	bun.BaseModel     `bun:"table:student_status_days,alias:student_status_day"`
	ID                int64        `bun:"id,pk,autoincrement"`
	TenantID          int64        `bun:"tenant_id,notnull"`
	CreatedAt         time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID         int64        `bun:"student_id,notnull"`
	Date              calendarDate `bun:"date,notnull,type:date"`
	Status            string       `bun:"status,notnull"`
	ReportedAt        time.Time    `bun:"reported_at,notnull"`
	ClearedAt         *time.Time   `bun:"cleared_at"`
	Source            string       `bun:"source,notnull"`
	GuardianAccountID *int64       `bun:"guardian_account_id"`
	Note              *string      `bun:"note"`
}

const (
	statusSick          = "sick"
	statusExcused       = "excused"
	statusClassTrip     = "class_trip"
	statusSourceEndDay  = "end_of_day"
	studentStatusActive = "active"
	substatusSick       = "sick"
	substatusExcused    = "excused"
	substatusFieldTrip  = "field_trip"
	substatusOther      = "other"
)

func (s *Store) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (careplan.StudentStatusDay, bool, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return careplan.StudentStatusDay{}, false, carePlanCompose.RequestStoreStats{}, err
	}
	row := new(studentStatusDayRow)
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Where(`"student_status_day".id = ?`, id), "student_status_day", tenantID)
	if activeOnly {
		query = query.Where(`"student_status_day".cleared_at IS NULL`)
	}
	started := time.Now()
	err = query.Scan(ctx)
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

func (s *Store) ListStudentStatusDays(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, carePlanCompose.RequestStoreStats{}, err
	}
	rows := []studentStatusDayRow{}
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).ColumnExpr(`"student_status_day".*`), "student_status_day", tenantID)
	if filter.LatestOnly {
		// PostgreSQL requires DISTINCT ON expressions to lead the ORDER BY list.
		query = orderStatusDays(query, filter)
	}
	query = filterStatusDays(query, filter)
	if !filter.LatestOnly {
		query = orderStatusDays(query, filter)
	}
	started := time.Now()
	err = query.Scan(ctx)
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
		query = query.Where(`"student_status_day".date = ?`, calendarDate(filter.Date))
	}
	if filter.From != "" {
		query = query.Where(`"student_status_day".date >= ?`, calendarDate(filter.From))
	}
	if filter.To != "" {
		query = query.Where(`"student_status_day".date <= ?`, calendarDate(filter.To))
	}
	if filter.ActiveOnly {
		query = query.Where(`"student_status_day".cleared_at IS NULL`)
	} else if filter.IncludeEndOfDay {
		query = query.Where(`("student_status_day".cleared_at IS NULL OR "student_status_day".source = ?)`, statusSourceEndDay)
	}
	if filter.LatestOnly {
		query = query.DistinctOn(`"student_status_day".student_id, "student_status_day".date`)
	}
	query = applyStudentScheduleOptions(query, filter.Options, "student_status_day")
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
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, carePlanCompose.RequestStoreStats{}, err
	}
	query := withTenant(db.NewSelect().Model((*studentStatusDayRow)(nil)).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`), "student_status_day", tenantID)
	if options != nil {
		query = applyStudentScheduleFilter(query, options.Filter, "student_status_day")
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
		case statusSick:
			value.sick = true
		case statusExcused:
			value.excused = true
		case statusClassTrip:
			value.classTrip = true
		}
		byStudent[row.StudentID] = value
	}
	counts := careplan.StudentStatusCounts{}
	for _, student := range students {
		if student.Status != studentStatusActive {
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
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, carePlanCompose.RequestStoreStats{}, err
	}
	var result []int64
	query := withTenant(db.NewSelect().TableExpr(`active.student_status_days AS "student_status_day"`).
		ColumnExpr(`"student_status_day".id`).Where(`"student_status_day".id IN (?)`, bun.List(ids)), "student_status_day", tenantID)
	started := time.Now()
	err = query.Scan(ctx, &result)
	stats := requestStats(started, int64(len(result)))
	if err != nil {
		return nil, stats, requestDBError("list existing student status day ids", err)
	}
	return result, stats, nil
}

func (s *statusDayStore) CountCarePlanDeletionRecords(ctx context.Context, studentID int64) (careplan.CarePlanDeletionCounts, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return careplan.CarePlanDeletionCounts{}, carePlanCompose.RequestStoreStats{}, err
	}
	counts := careplan.CarePlanDeletionCounts{}
	query := db.NewRaw(`SELECT
		(SELECT COUNT(*) FROM active.student_status_days WHERE tenant_id = ? AND student_id = ?)::int AS status_days,
		(SELECT COUNT(*) FROM active.excused_absence_requests WHERE tenant_id = ? AND student_id = ?)::int AS excused_requests,
		(SELECT COUNT(*) FROM schedule.care_schedule_change_requests WHERE tenant_id = ? AND student_id = ?)::int AS care_requests,
		(SELECT COUNT(*) FROM users.student_data_change_requests WHERE tenant_id = ? AND student_id = ?)::int AS data_requests`,
		tenantID, studentID, tenantID, studentID, tenantID, studentID, tenantID, studentID)
	started := time.Now()
	err = query.Scan(ctx, &counts)
	stats := requestStats(started, 1)
	if err != nil {
		return counts, stats, requestDBError("count care plan deletion records", err)
	}
	return counts, stats, nil
}

func (s *statusDayStore) UpsertStudentStatusDay(ctx context.Context, value careplan.StudentStatusDay) (careplan.StudentStatusDay, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "upsert a student status day")
	if err != nil {
		return careplan.StudentStatusDay{}, carePlanCompose.RequestStoreStats{}, err
	}
	row := statusDayFromPublic(value)
	row.TenantID = tenantID
	if row.Date == "" {
		return careplan.StudentStatusDay{}, carePlanCompose.RequestStoreStats{}, errors.New("student status day date is required")
	}
	if err := s.lockStudentDates(ctx, db, tenantID, row.StudentID, []calendarDate{row.Date}); err != nil {
		return careplan.StudentStatusDay{}, carePlanCompose.RequestStoreStats{}, err
	}
	query := db.NewInsert().Model(row).ModelTableExpr("active.student_status_days").
		On("CONFLICT (tenant_id, student_id, date, status) DO UPDATE").
		Set("reported_at = EXCLUDED.reported_at").Set("cleared_at = NULL").Set("source = EXCLUDED.source").
		Set(`note = CASE WHEN student_status_days.cleared_at IS NULL THEN COALESCE(EXCLUDED.note, student_status_days.note) ELSE EXCLUDED.note END`).
		Set(`guardian_account_id = CASE WHEN student_status_days.cleared_at IS NULL AND EXCLUDED.note IS NULL THEN student_status_days.guardian_account_id ELSE EXCLUDED.guardian_account_id END`).
		Returning("*")
	started := time.Now()
	err = query.Scan(ctx)
	stats := requestStats(started, 1)
	if err != nil {
		stats.Rows = 0
		return careplan.StudentStatusDay{}, stats, requestDBError("upsert student status day", err)
	}
	_, err = s.slots.ApplyStatusDay(ctx, row.StudentID, careplan.Date(row.Date), row.ID, attendanceSubstatus(row.Status))
	if err != nil {
		return careplan.StudentStatusDay{}, stats, err
	}
	return statusDayToPublic(row), stats, nil
}

func attendanceSubstatus(status string) string {
	switch status {
	case statusSick:
		return substatusSick
	case statusExcused:
		return substatusExcused
	case statusClassTrip:
		return substatusFieldTrip
	default:
		return substatusOther
	}
}

func (s *statusDayStore) ClearStudentStatusDays(ctx context.Context, studentID int64, status string, dates []careplan.Date, clearedAt time.Time, source string) (carePlanCompose.RequestStoreStats, error) {
	if len(dates) == 0 {
		return carePlanCompose.RequestStoreStats{}, nil
	}
	rowDates := make([]calendarDate, 0, len(dates))
	seen := make(map[calendarDate]struct{}, len(dates))
	for _, value := range dates {
		date := calendarDate(value)
		if _, ok := seen[date]; !ok {
			seen[date] = struct{}{}
			rowDates = append(rowDates, date)
		}
	}
	db, tenantID, err := s.databaseForWrite(ctx, "clear student status days")
	if err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	if err := s.lockStudentDates(ctx, db, tenantID, studentID, rowDates); err != nil {
		return carePlanCompose.RequestStoreStats{}, err
	}
	ids, stats, err := s.activeStatusDayIDs(ctx, studentID, status, rowDates)
	if err != nil {
		return stats, err
	}
	query := withTenant(db.NewUpdate().Model((*studentStatusDayRow)(nil)).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).Set("cleared_at = ?", clearedAt).Set("source = ?", source).
		Where(`"student_status_day".student_id = ?`, studentID).Where(`"student_status_day".status = ?`, status).
		Where(`"student_status_day".date IN (?)`, bun.List(rowDates)).Where(`"student_status_day".cleared_at IS NULL`), "student_status_day", tenantID)
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

func (s *statusDayStore) activeStatusDayIDs(ctx context.Context, studentID int64, status string, dates []calendarDate) ([]int64, carePlanCompose.RequestStoreStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, carePlanCompose.RequestStoreStats{}, err
	}
	var ids []int64
	query := withTenant(db.NewSelect().TableExpr(`active.student_status_days AS "student_status_day"`).
		ColumnExpr(`"student_status_day".id`).Where(`"student_status_day".student_id = ?`, studentID).
		Where(`"student_status_day".status = ?`, status).Where(`"student_status_day".date IN (?)`, bun.List(dates)).
		Where(`"student_status_day".cleared_at IS NULL`), "student_status_day", tenantID)
	started := time.Now()
	err = query.Scan(ctx, &ids)
	stats := requestStats(started, int64(len(ids)))
	if err != nil {
		return nil, stats, requestDBError("find active student status day ids", err)
	}
	return ids, stats, nil
}

func (s *statusDayStore) ClearStudentStatusDayByID(ctx context.Context, id int64, clearedAt time.Time, source string) (carePlanCompose.RequestStoreStats, error) {
	row, found, stats, err := s.FindStudentStatusDay(ctx, id, false)
	if err != nil || !found {
		return stats, err
	}
	db, tenantID, err := s.databaseForWrite(ctx, "clear a student status day")
	if err != nil {
		return stats, err
	}
	if err := s.lockStudentDates(ctx, db, tenantID, row.StudentID, []calendarDate{calendarDate(row.Date)}); err != nil {
		return stats, err
	}
	query := withTenant(db.NewUpdate().Model((*studentStatusDayRow)(nil)).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).Set("cleared_at = ?", clearedAt).Set("source = ?", source).
		Where(`"student_status_day".id = ?`, id).Where(`"student_status_day".cleared_at IS NULL`), "student_status_day", tenantID)
	started := time.Now()
	result, err := query.Exec(ctx)
	writeStats := requestStats(started, 0)
	if err != nil {
		return stats, requestDBError("mark student status day cleared by id", err)
	}
	writeStats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, requestDBError("mark student status day cleared by id", err)
	}
	stats.Queries += writeStats.Queries
	stats.Rows += writeStats.Rows
	stats.StatementDuration += writeStats.StatementDuration
	if _, err := s.slots.ReleaseStatusDay(ctx, id); err != nil {
		return stats, err
	}
	return stats, nil
}

func (s *statusDayStore) lockStudentDates(ctx context.Context, db bun.IDB, tenantID, studentID int64, dates []calendarDate) error {
	if err := s.students.LockStudent(ctx, studentID); err != nil {
		return err
	}
	sortedDates := append([]calendarDate(nil), dates...)
	slices.Sort(sortedDates)
	for _, date := range slices.Compact(sortedDates) {
		if err := LockExceptionDay(ctx, db, tenantID, studentID, string(date)); err != nil {
			return fmt.Errorf("lock student status day: %w", err)
		}
	}
	return nil
}

func (s *statusDayStore) ArchiveStudentStatusFlags(ctx context.Context, value careplan.StatusFlagArchive) (int64, carePlanCompose.RequestStoreStats, error) {
	if (value.Status == statusSick && (value.FlagColumn != "sick" || value.SinceColumn != "sick_since")) ||
		(value.Status == statusExcused && (value.FlagColumn != "excused" || value.SinceColumn != "excused_since")) {
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
	students = slices.DeleteFunc(students, func(student StatusStudent) bool { _, ok := locked[student.ID]; return !ok })
	rows := make([]studentStatusDayRow, 0, len(students))
	ids = ids[:0]
	for _, student := range students {
		reportedAt := value.ReportedFallback
		if value.Status == statusSick && student.SickSince != nil {
			reportedAt = *student.SickSince
		}
		if value.Status == statusExcused && student.ExcusedSince != nil {
			reportedAt = *student.ExcusedSince
		}
		row := studentStatusDayRow{TenantID: student.TenantID, StudentID: student.ID, Date: calendarDate(value.Date), Status: value.Status, ReportedAt: reportedAt, ClearedAt: &value.ReportedFallback, Source: value.Source}
		rows = append(rows, row)
		ids = append(ids, student.ID)
	}
	if len(rows) == 0 {
		return 0, carePlanCompose.RequestStoreStats{}, nil
	}
	db, _, err := s.databaseForWrite(ctx, "archive student status flags")
	if err != nil {
		return 0, carePlanCompose.RequestStoreStats{}, err
	}
	started := time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr("active.student_status_days").
		On("CONFLICT (tenant_id, student_id, date, status) DO UPDATE").Set("reported_at = EXCLUDED.reported_at").
		Set("cleared_at = EXCLUDED.cleared_at").Set("source = EXCLUDED.source").Exec(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return 0, stats, requestDBError("archive status flag", err)
	}
	affected, err := s.students.ClearStudentStatusFlags(ctx, ids, value.Status)
	return affected, stats, err
}

func statusDayFromPublic(value careplan.StudentStatusDay) *studentStatusDayRow {
	row := &studentStatusDayRow{StudentID: value.StudentID, Date: calendarDate(value.Date), Status: value.Status, ReportedAt: value.ReportedAt, ClearedAt: value.ClearedAt, Source: value.Source, GuardianAccountID: value.GuardianAccountID, Note: value.Note}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func statusDayToPublic(row *studentStatusDayRow) careplan.StudentStatusDay {
	return careplan.StudentStatusDay{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, Date: careplan.Date(row.Date), Status: row.Status, ReportedAt: row.ReportedAt, ClearedAt: row.ClearedAt, Source: row.Source, GuardianAccountID: row.GuardianAccountID, Note: row.Note}
}

func statusDaysToPublic(rows []studentStatusDayRow) []careplan.StudentStatusDay {
	result := make([]careplan.StudentStatusDay, 0, len(rows))
	for i := range rows {
		result = append(result, statusDayToPublic(&rows[i]))
	}
	return result
}
