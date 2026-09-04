package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

type arrivalScheduleRow struct {
	bun.BaseModel   `bun:"table:student_arrival_schedules,alias:student_arrival_schedule"`
	ID              int64     `bun:"id,pk,autoincrement"`
	TenantID        int64     `bun:"tenant_id,notnull"`
	CreatedAt       time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID       int64     `bun:"student_id,notnull"`
	Weekday         int       `bun:"weekday,notnull"`
	ExpectedArrival time.Time `bun:"expected_arrival,nullzero"`
	Notes           *string   `bun:"notes"`
	CreatedBy       int64     `bun:"created_by,notnull"`
}

type arrivalExceptionRow struct {
	bun.BaseModel     `bun:"table:student_arrival_exceptions,alias:student_arrival_exception"`
	ID                int64        `bun:"id,pk,autoincrement"`
	TenantID          int64        `bun:"tenant_id,notnull"`
	CreatedAt         time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID         int64        `bun:"student_id,notnull"`
	ExceptionDate     calendarDate `bun:"exception_date,notnull,type:date"`
	ExpectedArrival   *time.Time   `bun:"expected_arrival"`
	Reason            *string      `bun:"reason"`
	Source            string       `bun:"source,nullzero,notnull,default:'staff'"`
	CreatedBy         int64        `bun:"created_by,nullzero"`
	CreatedByGuardian *int64       `bun:"created_by_guardian,nullzero"`
}

type arrivalNoteRow struct {
	bun.BaseModel `bun:"table:student_arrival_notes,alias:student_arrival_note"`
	ID            int64        `bun:"id,pk,autoincrement"`
	TenantID      int64        `bun:"tenant_id,notnull"`
	CreatedAt     time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID     int64        `bun:"student_id,notnull"`
	NoteDate      calendarDate `bun:"note_date,notnull,type:date"`
	Content       string       `bun:"content,notnull"`
	CreatedBy     int64        `bun:"created_by,notnull"`
}

type pickupScheduleRow struct {
	bun.BaseModel  `bun:"table:student_pickup_schedules,alias:student_pickup_schedule"`
	ID             int64     `bun:"id,pk,autoincrement"`
	TenantID       int64     `bun:"tenant_id,notnull"`
	CreatedAt      time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID      int64     `bun:"student_id,notnull"`
	Weekday        int       `bun:"weekday,notnull"`
	PickupTime     time.Time `bun:"pickup_time,notnull"`
	Notes          *string   `bun:"notes"`
	CreatedBy      int64     `bun:"created_by,nullzero"`
	Source         string    `bun:"source,nullzero,notnull,default:'staff'"`
	CareOfferingID *int64    `bun:"care_offering_id,nullzero"`
}

type pickupExceptionRow struct {
	bun.BaseModel         `bun:"table:student_pickup_exceptions,alias:student_pickup_exception"`
	ID                    int64        `bun:"id,pk,autoincrement"`
	TenantID              int64        `bun:"tenant_id,notnull"`
	CreatedAt             time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID             int64        `bun:"student_id,notnull"`
	ExceptionDate         calendarDate `bun:"exception_date,notnull,type:date"`
	PickupTime            *time.Time   `bun:"pickup_time"`
	Reason                *string      `bun:"reason"`
	ExcusedFrom           *time.Time   `bun:"excused_from"`
	ExcusedReason         *string      `bun:"excused_reason"`
	ExcusedCreatedBy      *int64       `bun:"excused_created_by"`
	ExcusedOwnsPickupTime bool         `bun:"excused_owns_pickup_time,notnull,default:false"`
	ExcusedAuto           bool         `bun:"excused_auto,notnull,default:false"`
	Source                string       `bun:"source,nullzero,notnull,default:'staff'"`
	CreatedBy             int64        `bun:"created_by,nullzero"`
	CreatedByGuardian     *int64       `bun:"created_by_guardian,nullzero"`
}

type pickupNoteRow struct {
	bun.BaseModel `bun:"table:student_pickup_notes,alias:student_pickup_note"`
	ID            int64        `bun:"id,pk,autoincrement"`
	TenantID      int64        `bun:"tenant_id,notnull"`
	CreatedAt     time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID     int64        `bun:"student_id,notnull"`
	NoteDate      calendarDate `bun:"note_date,notnull,type:date"`
	Content       string       `bun:"content,notnull"`
	CreatedBy     int64        `bun:"created_by,notnull"`
}

func finishStudentScheduleList[T any](ctx context.Context, query *bun.SelectQuery, rows *[]T, operation string) (domain.OperationStats, error) {
	stats, err := scanAll(ctx, query, operation)
	if err == nil {
		stats.Rows = int64(len(*rows))
	}
	return stats, err
}

func findStudentScheduleRow(ctx context.Context, query *bun.SelectQuery, lock bool, operation string) (bool, domain.OperationStats, error) {
	if lock {
		query = query.For("UPDATE")
	}
	return scanOne(ctx, query, operation)
}

func createStudentScheduleRow(ctx context.Context, query *bun.InsertQuery, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("care plan postgres: %s: %w", operation, err)
	}
	stats.Rows = 1
	return stats, nil
}

func (s *Store) FindArrivalSchedule(ctx context.Context, id int64) (careplan.ArrivalSchedule, bool, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return careplan.ArrivalSchedule{}, false, domain.OperationStats{}, err
	}
	row := new(arrivalScheduleRow)
	query := db.NewSelect().Model(row).
		ModelTableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).
		Where(`"student_arrival_schedule".tenant_id = ?`, tid).
		Where(`"student_arrival_schedule".id = ?`, id)
	found, stats, err := findStudentScheduleRow(ctx, query, false, "find arrival schedule")
	return arrivalScheduleToPublic(*row), found, stats, err
}
func (s *Store) ListArrivalSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []arrivalScheduleRow{}
	if f.IDs != nil && len(f.IDs) == 0 || f.StudentIDs != nil && len(f.StudentIDs) == 0 {
		return []careplan.ArrivalSchedule{}, domain.OperationStats{}, nil
	}
	query := db.NewSelect().Model(&rows).
		ModelTableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).
		Where(`"student_arrival_schedule".tenant_id = ?`, tid)
	if f.IDs != nil {
		query = query.Where(`"student_arrival_schedule".id IN (?)`, bun.List(f.IDs))
	}
	if f.StudentIDs != nil {
		query = query.Where(`"student_arrival_schedule".student_id IN (?)`, bun.List(f.StudentIDs))
	}
	if f.Weekday > 0 {
		query = query.Where(`"student_arrival_schedule".weekday = ?`, f.Weekday)
	}
	query = query.OrderExpr(`"student_arrival_schedule".student_id ASC, "student_arrival_schedule".weekday ASC`)
	if f.LockForUpdate {
		query = query.For("UPDATE")
	}
	stats, err := finishStudentScheduleList(ctx, query, &rows, "list arrival schedules")
	return mapRows(rows, arrivalScheduleToPublic), stats, err
}
func (s *Store) CreateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (careplan.ArrivalSchedule, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "create arrival schedule")
	if err != nil {
		return careplan.ArrivalSchedule{}, domain.OperationStats{}, err
	}
	row := arrivalScheduleFromPublic(v)
	row.TenantID = tid
	stats, err := createStudentScheduleRow(ctx, db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_arrival_schedules`), "create arrival schedule")
	return arrivalScheduleToPublic(row), stats, err
}
func (s *Store) UpdateArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "update arrival schedule")
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := arrivalScheduleFromPublic(v)
	row.TenantID = tid
	return execGuarded(ctx, db.NewUpdate().Model(&row).
		ModelTableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).
		Where(`"student_arrival_schedule".id = ?`, row.ID).
		Where(`"student_arrival_schedule".tenant_id = ?`, tid), "update arrival schedule", careplan.ErrStudentScheduleNotFound)
}
func (s *Store) UpsertArrivalSchedule(ctx context.Context, v careplan.ArrivalSchedule) (careplan.ArrivalSchedule, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "upsert arrival schedule")
	if err != nil {
		return careplan.ArrivalSchedule{}, domain.OperationStats{}, err
	}
	row := arrivalScheduleFromPublic(v)
	row.TenantID = tid
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_arrival_schedules`).On("CONFLICT (tenant_id, student_id, weekday) DO UPDATE").Set("expected_arrival = EXCLUDED.expected_arrival").Set("notes = EXCLUDED.notes").Set("updated_at = NOW()").Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return careplan.ArrivalSchedule{}, stats, fmt.Errorf("care plan postgres: upsert arrival schedule: %w", err)
	}
	stats.Rows = 1
	return arrivalScheduleToPublic(row), stats, nil
}
func (s *Store) DeleteArrivalSchedule(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete arrival schedule")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).Where(`"student_arrival_schedule".id = ?`, id).Where(`"student_arrival_schedule".tenant_id = ?`, tid), "delete arrival schedule")
}
func (s *Store) DeleteArrivalSchedulesByStudent(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete arrival schedules by student")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_arrival_schedules AS "student_arrival_schedule"`).Where(`"student_arrival_schedule".tenant_id = ?`, tid).Where(`"student_arrival_schedule".student_id = ?`, id), "delete arrival schedules by student")
}

func (s *Store) FindArrivalException(ctx context.Context, id int64, lock bool) (careplan.ArrivalException, bool, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return careplan.ArrivalException{}, false, domain.OperationStats{}, err
	}
	row := new(arrivalExceptionRow)
	query := db.NewSelect().Model(row).ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).Where(`"student_arrival_exception".tenant_id = ?`, tid).Where(`"student_arrival_exception".id = ?`, id)
	found, stats, err := findStudentScheduleRow(ctx, query, lock, "find arrival exception")
	return arrivalExceptionToPublic(*row), found, stats, err
}
func (s *Store) ListArrivalExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalException, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []arrivalExceptionRow{}
	if f.IDs != nil && len(f.IDs) == 0 || f.StudentIDs != nil && len(f.StudentIDs) == 0 {
		return []careplan.ArrivalException{}, domain.OperationStats{}, nil
	}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).Where(`"student_arrival_exception".tenant_id = ?`, tid)
	if f.IDs != nil {
		query = query.Where(`"student_arrival_exception".id IN (?)`, bun.List(f.IDs))
	}
	if f.StudentIDs != nil {
		query = query.Where(`"student_arrival_exception".student_id IN (?)`, bun.List(f.StudentIDs))
	}
	if !f.Date.IsZero() {
		query = query.Where(`"student_arrival_exception".exception_date = ?`, calendarDate(f.Date))
	}
	if !f.From.IsZero() {
		query = query.Where(`"student_arrival_exception".exception_date >= ?`, calendarDate(f.From))
	}
	if !f.To.IsZero() {
		query = query.Where(`"student_arrival_exception".exception_date <= ?`, calendarDate(f.To))
	}
	if !f.UpcomingFrom.IsZero() {
		query = query.Where(`"student_arrival_exception".exception_date >= ?`, calendarDate(f.UpcomingFrom))
	}
	query = query.OrderExpr(`"student_arrival_exception".exception_date ASC`)
	if f.LockForUpdate {
		query = query.For("UPDATE")
	}
	stats, err := finishStudentScheduleList(ctx, query, &rows, "list arrival exceptions")
	return mapRows(rows, arrivalExceptionToPublic), stats, err
}
func (s *Store) CreateArrivalException(ctx context.Context, v careplan.ArrivalException) (careplan.ArrivalException, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "create arrival exception")
	if err != nil {
		return careplan.ArrivalException{}, domain.OperationStats{}, err
	}
	row := arrivalExceptionFromPublic(v)
	row.TenantID = tid
	stats, err := createStudentScheduleRow(ctx, db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_arrival_exceptions`), "create arrival exception")
	return arrivalExceptionToPublic(row), stats, err
}
func (s *Store) UpdateArrivalException(ctx context.Context, v careplan.ArrivalException) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "update arrival exception")
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := arrivalExceptionFromPublic(v)
	row.TenantID = tid
	return execGuarded(ctx, db.NewUpdate().Model(&row).ModelTableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).Where(`"student_arrival_exception".id = ?`, row.ID).Where(`"student_arrival_exception".tenant_id = ?`, tid), "update arrival exception", careplan.ErrStudentScheduleNotFound)
}
func (s *Store) DeleteArrivalException(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete arrival exception")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).Where(`"student_arrival_exception".id = ?`, id).Where(`"student_arrival_exception".tenant_id = ?`, tid), "delete arrival exception")
}
func (s *Store) DeleteArrivalExceptionsByStudent(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete arrival exceptions by student")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).Where(`"student_arrival_exception".tenant_id = ?`, tid).Where(`"student_arrival_exception".student_id = ?`, id), "delete arrival exceptions by student")
}
func (s *Store) DeleteArrivalExceptionsBefore(ctx context.Context, d careplan.Date) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete old arrival exceptions")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_arrival_exceptions AS "student_arrival_exception"`).Where(`"student_arrival_exception".tenant_id = ?`, tid).Where(`"student_arrival_exception".exception_date < ?`, calendarDate(d)), "delete past arrival exceptions")
}

func (s *Store) FindArrivalNote(ctx context.Context, id int64) (careplan.ArrivalNote, bool, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return careplan.ArrivalNote{}, false, domain.OperationStats{}, err
	}
	row := new(arrivalNoteRow)
	query := db.NewSelect().Model(row).ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).Where(`"student_arrival_note".tenant_id = ?`, tid).Where(`"student_arrival_note".id = ?`, id)
	found, stats, err := findStudentScheduleRow(ctx, query, false, "find arrival note")
	return arrivalNoteToPublic(*row), found, stats, err
}
func (s *Store) ListArrivalNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalNote, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []arrivalNoteRow{}
	if f.IDs != nil && len(f.IDs) == 0 || f.StudentIDs != nil && len(f.StudentIDs) == 0 {
		return []careplan.ArrivalNote{}, domain.OperationStats{}, nil
	}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).Where(`"student_arrival_note".tenant_id = ?`, tid)
	if f.IDs != nil {
		query = query.Where(`"student_arrival_note".id IN (?)`, bun.List(f.IDs))
	}
	if f.StudentIDs != nil {
		query = query.Where(`"student_arrival_note".student_id IN (?)`, bun.List(f.StudentIDs))
	}
	if !f.Date.IsZero() {
		query = query.Where(`"student_arrival_note".note_date = ?`, calendarDate(f.Date))
	}
	if !f.From.IsZero() {
		query = query.Where(`"student_arrival_note".note_date >= ?`, calendarDate(f.From))
	}
	if !f.To.IsZero() {
		query = query.Where(`"student_arrival_note".note_date <= ?`, calendarDate(f.To))
	}
	if !f.UpcomingFrom.IsZero() {
		query = query.Where(`"student_arrival_note".note_date >= ?`, calendarDate(f.UpcomingFrom))
	}
	query = query.OrderExpr(`"student_arrival_note".note_date ASC, "student_arrival_note".created_at ASC`)
	if f.LockForUpdate {
		query = query.For("UPDATE")
	}
	stats, err := finishStudentScheduleList(ctx, query, &rows, "list arrival notes")
	return mapRows(rows, arrivalNoteToPublic), stats, err
}
func (s *Store) CreateArrivalNote(ctx context.Context, v careplan.ArrivalNote) (careplan.ArrivalNote, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "create arrival note")
	if err != nil {
		return careplan.ArrivalNote{}, domain.OperationStats{}, err
	}
	row := arrivalNoteFromPublic(v)
	row.TenantID = tid
	stats, err := createStudentScheduleRow(ctx, db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_arrival_notes`), "create arrival note")
	return arrivalNoteToPublic(row), stats, err
}
func (s *Store) UpdateArrivalNote(ctx context.Context, v careplan.ArrivalNote) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "update arrival note")
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := arrivalNoteFromPublic(v)
	row.TenantID = tid
	return execGuarded(ctx, db.NewUpdate().Model(&row).ModelTableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).Where(`"student_arrival_note".id = ?`, row.ID).Where(`"student_arrival_note".tenant_id = ?`, tid), "update arrival note", careplan.ErrStudentScheduleNotFound)
}
func (s *Store) DeleteArrivalNote(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete arrival note")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).Where(`"student_arrival_note".id = ?`, id).Where(`"student_arrival_note".tenant_id = ?`, tid), "delete arrival note")
}
func (s *Store) DeleteArrivalNotesByStudent(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete arrival notes by student")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).Where(`"student_arrival_note".tenant_id = ?`, tid).Where(`"student_arrival_note".student_id = ?`, id), "delete arrival notes by student")
}
func (s *Store) DeleteArrivalNotesBefore(ctx context.Context, d careplan.Date) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete old arrival notes")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_arrival_notes AS "student_arrival_note"`).Where(`"student_arrival_note".tenant_id = ?`, tid).Where(`"student_arrival_note".note_date < ?`, calendarDate(d)), "delete past arrival notes")
}

func (s *Store) FindPickupSchedule(ctx context.Context, id int64) (careplan.PickupSchedule, bool, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return careplan.PickupSchedule{}, false, domain.OperationStats{}, err
	}
	row := new(pickupScheduleRow)
	query := db.NewSelect().Model(row).ModelTableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).Where(`"student_pickup_schedule".tenant_id = ?`, tid).Where(`"student_pickup_schedule".id = ?`, id)
	found, stats, err := findStudentScheduleRow(ctx, query, false, "find pickup schedule")
	return pickupScheduleToPublic(*row), found, stats, err
}
func (s *Store) ListPickupSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []pickupScheduleRow{}
	if f.IDs != nil && len(f.IDs) == 0 || f.StudentIDs != nil && len(f.StudentIDs) == 0 {
		return []careplan.PickupSchedule{}, domain.OperationStats{}, nil
	}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).Where(`"student_pickup_schedule".tenant_id = ?`, tid)
	if f.IDs != nil {
		query = query.Where(`"student_pickup_schedule".id IN (?)`, bun.List(f.IDs))
	}
	if f.StudentIDs != nil {
		query = query.Where(`"student_pickup_schedule".student_id IN (?)`, bun.List(f.StudentIDs))
	}
	if f.Weekday > 0 {
		query = query.Where(`"student_pickup_schedule".weekday = ?`, f.Weekday)
	}
	query = query.OrderExpr(`"student_pickup_schedule".student_id ASC, "student_pickup_schedule".weekday ASC`)
	if f.LockForUpdate {
		query = query.For("UPDATE")
	}
	stats, err := finishStudentScheduleList(ctx, query, &rows, "list pickup schedules")
	return mapRows(rows, pickupScheduleToPublic), stats, err
}
func (s *Store) CreatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) (careplan.PickupSchedule, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "create pickup schedule")
	if err != nil {
		return careplan.PickupSchedule{}, domain.OperationStats{}, err
	}
	row := pickupScheduleFromPublic(v)
	row.TenantID = tid
	stats, err := createStudentScheduleRow(ctx, db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_pickup_schedules`), "create pickup schedule")
	return pickupScheduleToPublic(row), stats, err
}
func (s *Store) UpdatePickupSchedule(ctx context.Context, v careplan.PickupSchedule) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "update pickup schedule")
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := pickupScheduleFromPublic(v)
	row.TenantID = tid
	return execGuarded(ctx, db.NewUpdate().Model(&row).ModelTableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).Where(`"student_pickup_schedule".id = ?`, row.ID).Where(`"student_pickup_schedule".tenant_id = ?`, tid), "update pickup schedule", careplan.ErrStudentScheduleNotFound)
}
func (s *Store) UpsertPickupSchedule(ctx context.Context, v careplan.PickupSchedule) (careplan.PickupSchedule, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "upsert pickup schedule")
	if err != nil {
		return careplan.PickupSchedule{}, domain.OperationStats{}, err
	}
	row := pickupScheduleFromPublic(v)
	row.TenantID = tid
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_pickup_schedules`).On("CONFLICT (tenant_id, student_id, weekday) DO UPDATE").Set("pickup_time = EXCLUDED.pickup_time").Set("notes = EXCLUDED.notes").Set("source = EXCLUDED.source").Set("care_offering_id = EXCLUDED.care_offering_id").Set("updated_at = NOW()").Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return careplan.PickupSchedule{}, stats, fmt.Errorf("care plan postgres: upsert pickup schedule: %w", err)
	}
	stats.Rows = 1
	return pickupScheduleToPublic(row), stats, nil
}
func (s *Store) DeletePickupSchedule(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete pickup schedule")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).Where(`"student_pickup_schedule".id = ?`, id).Where(`"student_pickup_schedule".tenant_id = ?`, tid), "delete pickup schedule")
}
func (s *Store) DeletePickupSchedulesByStudent(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete pickup schedules by student")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_schedules AS "student_pickup_schedule"`).Where(`"student_pickup_schedule".tenant_id = ?`, tid).Where(`"student_pickup_schedule".student_id = ?`, id), "delete pickup schedules by student")
}

func (s *Store) FindPickupException(ctx context.Context, id int64, lock bool) (careplan.PickupException, bool, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return careplan.PickupException{}, false, domain.OperationStats{}, err
	}
	row := new(pickupExceptionRow)
	query := db.NewSelect().Model(row).ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".tenant_id = ?`, tid).Where(`"student_pickup_exception".id = ?`, id)
	found, stats, err := findStudentScheduleRow(ctx, query, lock, "find pickup exception")
	return pickupExceptionToPublic(*row), found, stats, err
}
func (s *Store) ListPickupExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupException, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []pickupExceptionRow{}
	if f.IDs != nil && len(f.IDs) == 0 || f.StudentIDs != nil && len(f.StudentIDs) == 0 {
		return []careplan.PickupException{}, domain.OperationStats{}, nil
	}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".tenant_id = ?`, tid)
	if f.IDs != nil {
		query = query.Where(`"student_pickup_exception".id IN (?)`, bun.List(f.IDs))
	}
	if f.StudentIDs != nil {
		query = query.Where(`"student_pickup_exception".student_id IN (?)`, bun.List(f.StudentIDs))
	}
	if !f.Date.IsZero() {
		query = query.Where(`"student_pickup_exception".exception_date = ?`, calendarDate(f.Date))
	}
	if !f.From.IsZero() {
		query = query.Where(`"student_pickup_exception".exception_date >= ?`, calendarDate(f.From))
	}
	if !f.To.IsZero() {
		query = query.Where(`"student_pickup_exception".exception_date <= ?`, calendarDate(f.To))
	}
	if !f.UpcomingFrom.IsZero() {
		query = query.Where(`"student_pickup_exception".exception_date >= ?`, calendarDate(f.UpcomingFrom))
	}
	query = query.OrderExpr(`"student_pickup_exception".exception_date ASC`)
	if f.LockForUpdate {
		query = query.For("UPDATE")
	}
	stats, err := finishStudentScheduleList(ctx, query, &rows, "list pickup exceptions")
	return mapRows(rows, pickupExceptionToPublic), stats, err
}
func (s *Store) CreatePickupException(ctx context.Context, v careplan.PickupException) (careplan.PickupException, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "create pickup exception")
	if err != nil {
		return careplan.PickupException{}, domain.OperationStats{}, err
	}
	row := pickupExceptionFromPublic(v)
	row.TenantID = tid
	stats, err := createStudentScheduleRow(ctx, db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_pickup_exceptions`), "create pickup exception")
	return pickupExceptionToPublic(row), stats, err
}
func (s *Store) UpdatePickupException(ctx context.Context, v careplan.PickupException) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "update pickup exception")
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := pickupExceptionFromPublic(v)
	row.TenantID = tid
	return execGuarded(ctx, db.NewUpdate().Model(&row).ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".id = ?`, row.ID).Where(`"student_pickup_exception".tenant_id = ?`, tid), "update pickup exception", careplan.ErrStudentScheduleNotFound)
}
func (s *Store) DeletePickupException(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete pickup exception")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".id = ?`, id).Where(`"student_pickup_exception".tenant_id = ?`, tid), "delete pickup exception")
}
func (s *Store) DeletePickupExceptionsByStudent(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete pickup exceptions by student")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".tenant_id = ?`, tid).Where(`"student_pickup_exception".student_id = ?`, id), "delete pickup exceptions by student")
}
func (s *Store) DeletePickupExceptionsBefore(ctx context.Context, d careplan.Date) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete old pickup exceptions")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".tenant_id = ?`, tid).Where(`"student_pickup_exception".exception_date < ?`, calendarDate(d)), "delete past pickup exceptions")
}

func (s *Store) FindPickupNote(ctx context.Context, id int64) (careplan.PickupNote, bool, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return careplan.PickupNote{}, false, domain.OperationStats{}, err
	}
	row := new(pickupNoteRow)
	query := db.NewSelect().Model(row).ModelTableExpr(`schedule.student_pickup_notes AS "student_pickup_note"`).Where(`"student_pickup_note".tenant_id = ?`, tid).Where(`"student_pickup_note".id = ?`, id)
	found, stats, err := findStudentScheduleRow(ctx, query, false, "find pickup note")
	return pickupNoteToPublic(*row), found, stats, err
}
func (s *Store) ListPickupNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupNote, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []pickupNoteRow{}
	if f.IDs != nil && len(f.IDs) == 0 || f.StudentIDs != nil && len(f.StudentIDs) == 0 {
		return []careplan.PickupNote{}, domain.OperationStats{}, nil
	}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`schedule.student_pickup_notes AS "student_pickup_note"`).Where(`"student_pickup_note".tenant_id = ?`, tid)
	if f.IDs != nil {
		query = query.Where(`"student_pickup_note".id IN (?)`, bun.List(f.IDs))
	}
	if f.StudentIDs != nil {
		query = query.Where(`"student_pickup_note".student_id IN (?)`, bun.List(f.StudentIDs))
	}
	if !f.Date.IsZero() {
		query = query.Where(`"student_pickup_note".note_date = ?`, calendarDate(f.Date))
	}
	if !f.From.IsZero() {
		query = query.Where(`"student_pickup_note".note_date >= ?`, calendarDate(f.From))
	}
	if !f.To.IsZero() {
		query = query.Where(`"student_pickup_note".note_date <= ?`, calendarDate(f.To))
	}
	if !f.UpcomingFrom.IsZero() {
		query = query.Where(`"student_pickup_note".note_date >= ?`, calendarDate(f.UpcomingFrom))
	}
	query = query.OrderExpr(`"student_pickup_note".note_date ASC, "student_pickup_note".created_at ASC`)
	if f.LockForUpdate {
		query = query.For("UPDATE")
	}
	stats, err := finishStudentScheduleList(ctx, query, &rows, "list pickup notes")
	return mapRows(rows, pickupNoteToPublic), stats, err
}
func (s *Store) CreatePickupNote(ctx context.Context, v careplan.PickupNote) (careplan.PickupNote, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "create pickup note")
	if err != nil {
		return careplan.PickupNote{}, domain.OperationStats{}, err
	}
	row := pickupNoteFromPublic(v)
	row.TenantID = tid
	stats, err := createStudentScheduleRow(ctx, db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_pickup_notes`), "create pickup note")
	return pickupNoteToPublic(row), stats, err
}
func (s *Store) UpdatePickupNote(ctx context.Context, v careplan.PickupNote) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "update pickup note")
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := pickupNoteFromPublic(v)
	row.TenantID = tid
	return execGuarded(ctx, db.NewUpdate().Model(&row).ModelTableExpr(`schedule.student_pickup_notes AS "student_pickup_note"`).Where(`"student_pickup_note".id = ?`, row.ID).Where(`"student_pickup_note".tenant_id = ?`, tid), "update pickup note", careplan.ErrStudentScheduleNotFound)
}
func (s *Store) DeletePickupNote(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete pickup note")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_notes AS "student_pickup_note"`).Where(`"student_pickup_note".id = ?`, id).Where(`"student_pickup_note".tenant_id = ?`, tid), "delete pickup note")
}
func (s *Store) DeletePickupNotesByStudent(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete pickup notes by student")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_notes AS "student_pickup_note"`).Where(`"student_pickup_note".tenant_id = ?`, tid).Where(`"student_pickup_note".student_id = ?`, id), "delete pickup notes by student")
}
func (s *Store) DeletePickupNotesBefore(ctx context.Context, d careplan.Date) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete old pickup notes")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_notes AS "student_pickup_note"`).Where(`"student_pickup_note".tenant_id = ?`, tid).Where(`"student_pickup_note".note_date < ?`, calendarDate(d)), "delete past pickup notes")
}

func (s *Store) CountStudentScheduleRows(ctx context.Context, studentID int64) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	var count int
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT (
		(SELECT COUNT(*) FROM schedule.student_arrival_schedules WHERE tenant_id = ? AND student_id = ?) +
		(SELECT COUNT(*) FROM schedule.student_arrival_exceptions WHERE tenant_id = ? AND student_id = ?) +
		(SELECT COUNT(*) FROM schedule.student_arrival_notes WHERE tenant_id = ? AND student_id = ?) +
		(SELECT COUNT(*) FROM schedule.student_pickup_schedules WHERE tenant_id = ? AND student_id = ?) +
		(SELECT COUNT(*) FROM schedule.student_pickup_exceptions WHERE tenant_id = ? AND student_id = ?) +
		(SELECT COUNT(*) FROM schedule.student_pickup_notes WHERE tenant_id = ? AND student_id = ?)
	)::int`, tenantID, studentID, tenantID, studentID, tenantID, studentID, tenantID, studentID, tenantID, studentID, tenantID, studentID).Scan(ctx, &count)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("care plan postgres: count student schedule rows: %w", err)
	}
	stats.Rows = int64(count)
	return count, stats, nil
}

func addOperationCost(total *domain.OperationStats, operation domain.OperationStats) {
	total.Queries += operation.Queries
	total.Conflicts += operation.Conflicts
	total.StatementDuration += operation.StatementDuration
}

func (s *Store) archiveAndDeleteStudentSchedules(ctx context.Context, snapshot *bun.RawQuery, deletion executable, kind string, stats *domain.OperationStats) (int64, error) {
	removals := make([]domain.CareExitSourceRemoval, 0)
	started := time.Now()
	err := snapshot.Scan(ctx, &removals)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return 0, fmt.Errorf("care plan postgres: snapshot %s before care exit: %w", kind, err)
	}
	recorded, err := s.RecordCareExitSourceRemovals(ctx, removals)
	addOperationCost(stats, recorded)
	if err != nil {
		return 0, err
	}
	removed, err := execAny(ctx, deletion, "delete "+kind+" after care exit")
	addOperationCost(stats, removed)
	return removed.Rows, err
}

func (s *Store) EndStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64, validUntil careplan.Date) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "end student schedules for care exit")
	if err != nil {
		return domain.OperationStats{}, err
	}
	var stats domain.OperationStats
	operations := []struct {
		snapshot *bun.RawQuery
		deletion executable
		kind     string
	}{
		{db.NewRaw(`SELECT tenant_id, student_id, 'pickup_schedule'::text AS kind, id AS source_row_id, TRUE AS was_deleted, to_jsonb(plan) AS snapshot FROM schedule.student_pickup_schedules AS plan WHERE tenant_id = ? AND student_id IN (?)`, tenantID, bun.List(studentIDs)), db.NewDelete().TableExpr(`schedule.student_pickup_schedules AS "plan"`).Where(`"plan".tenant_id = ?`, tenantID).Where(`"plan".student_id IN (?)`, bun.List(studentIDs)), careplan.CareExitSourcePickupSchedule},
		{db.NewRaw(`SELECT tenant_id, student_id, 'arrival_schedule'::text AS kind, id AS source_row_id, TRUE AS was_deleted, to_jsonb(plan) AS snapshot FROM schedule.student_arrival_schedules AS plan WHERE tenant_id = ? AND student_id IN (?)`, tenantID, bun.List(studentIDs)), db.NewDelete().TableExpr(`schedule.student_arrival_schedules AS "plan"`).Where(`"plan".tenant_id = ?`, tenantID).Where(`"plan".student_id IN (?)`, bun.List(studentIDs)), careplan.CareExitSourceArrivalSchedule},
		{db.NewRaw(`SELECT tenant_id, student_id, 'pickup_exception'::text AS kind, id AS source_row_id, TRUE AS was_deleted, to_jsonb(plan) AS snapshot FROM schedule.student_pickup_exceptions AS plan WHERE tenant_id = ? AND student_id IN (?) AND exception_date >= ?`, tenantID, bun.List(studentIDs), calendarDate(validUntil)), db.NewDelete().TableExpr(`schedule.student_pickup_exceptions AS "plan"`).Where(`"plan".tenant_id = ?`, tenantID).Where(`"plan".student_id IN (?)`, bun.List(studentIDs)).Where(`"plan".exception_date >= ?`, calendarDate(validUntil)), careplan.CareExitSourcePickupException},
		{db.NewRaw(`SELECT tenant_id, student_id, 'arrival_exception'::text AS kind, id AS source_row_id, TRUE AS was_deleted, to_jsonb(plan) AS snapshot FROM schedule.student_arrival_exceptions AS plan WHERE tenant_id = ? AND student_id IN (?) AND exception_date >= ?`, tenantID, bun.List(studentIDs), calendarDate(validUntil)), db.NewDelete().TableExpr(`schedule.student_arrival_exceptions AS "plan"`).Where(`"plan".tenant_id = ?`, tenantID).Where(`"plan".student_id IN (?)`, bun.List(studentIDs)).Where(`"plan".exception_date >= ?`, calendarDate(validUntil)), careplan.CareExitSourceArrivalException},
	}
	var deletedRows int64
	for _, operation := range operations {
		rows, err := s.archiveAndDeleteStudentSchedules(ctx, operation.snapshot, operation.deletion, operation.kind, &stats)
		deletedRows += rows
		if err != nil {
			return stats, err
		}
	}
	stats.Rows = deletedRows
	return stats, nil
}

func (s *Store) RestoreStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "restore student schedules for care exit")
	if err != nil {
		return domain.OperationStats{}, err
	}
	var stats domain.OperationStats
	operations := []struct {
		query *bun.RawQuery
		kind  string
	}{
		{db.NewRaw(`INSERT INTO schedule.student_pickup_schedules SELECT (jsonb_populate_record(NULL::schedule.student_pickup_schedules, removal.snapshot)).* FROM users.student_care_exit_source_removals AS removal WHERE removal.kind = 'pickup_schedule' AND removal.was_deleted = TRUE AND removal.tenant_id = ? AND removal.student_id IN (?) ON CONFLICT DO NOTHING`, tenantID, bun.List(studentIDs)), careplan.CareExitSourcePickupSchedule},
		{db.NewRaw(`INSERT INTO schedule.student_arrival_schedules SELECT (jsonb_populate_record(NULL::schedule.student_arrival_schedules, removal.snapshot)).* FROM users.student_care_exit_source_removals AS removal WHERE removal.kind = 'arrival_schedule' AND removal.was_deleted = TRUE AND removal.tenant_id = ? AND removal.student_id IN (?) ON CONFLICT DO NOTHING`, tenantID, bun.List(studentIDs)), careplan.CareExitSourceArrivalSchedule},
		{db.NewRaw(`INSERT INTO schedule.student_pickup_exceptions SELECT (jsonb_populate_record(NULL::schedule.student_pickup_exceptions, removal.snapshot)).* FROM users.student_care_exit_source_removals AS removal WHERE removal.kind = 'pickup_exception' AND removal.was_deleted = TRUE AND removal.tenant_id = ? AND removal.student_id IN (?) ON CONFLICT DO NOTHING`, tenantID, bun.List(studentIDs)), careplan.CareExitSourcePickupException},
		{db.NewRaw(`INSERT INTO schedule.student_arrival_exceptions SELECT (jsonb_populate_record(NULL::schedule.student_arrival_exceptions, removal.snapshot)).* FROM users.student_care_exit_source_removals AS removal WHERE removal.kind = 'arrival_exception' AND removal.was_deleted = TRUE AND removal.tenant_id = ? AND removal.student_id IN (?) ON CONFLICT DO NOTHING`, tenantID, bun.List(studentIDs)), careplan.CareExitSourceArrivalException},
	}
	var restoredRows int64
	for _, operation := range operations {
		result, err := execAny(ctx, operation.query, "restore "+operation.kind+" after care exit")
		addOperationCost(&stats, result)
		restoredRows += result.Rows
		if err != nil {
			return stats, err
		}
	}
	stats.Rows = restoredRows
	return stats, nil
}

func mapRows[T any, R any](rows []T, convert func(T) R) []R {
	result := make([]R, 0, len(rows))
	for _, row := range rows {
		result = append(result, convert(row))
	}
	return result
}

func arrivalScheduleFromPublic(v careplan.ArrivalSchedule) arrivalScheduleRow {
	return arrivalScheduleRow{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, ExpectedArrival: v.ExpectedArrival, Notes: v.Notes, CreatedBy: v.CreatedBy}
}
func arrivalScheduleToPublic(v arrivalScheduleRow) careplan.ArrivalSchedule {
	return careplan.ArrivalSchedule{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, ExpectedArrival: v.ExpectedArrival, Notes: v.Notes, CreatedBy: v.CreatedBy}
}
func arrivalExceptionFromPublic(v careplan.ArrivalException) arrivalExceptionRow {
	return arrivalExceptionRow{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: calendarDate(v.ExceptionDate), ExpectedArrival: v.ExpectedArrival, Reason: v.Reason, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func arrivalExceptionToPublic(v arrivalExceptionRow) careplan.ArrivalException {
	return careplan.ArrivalException{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: careplan.Date(v.ExceptionDate), ExpectedArrival: v.ExpectedArrival, Reason: v.Reason, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func arrivalNoteFromPublic(v careplan.ArrivalNote) arrivalNoteRow {
	return arrivalNoteRow{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: calendarDate(v.NoteDate), Content: v.Content, CreatedBy: v.CreatedBy}
}
func arrivalNoteToPublic(v arrivalNoteRow) careplan.ArrivalNote {
	return careplan.ArrivalNote{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: careplan.Date(v.NoteDate), Content: v.Content, CreatedBy: v.CreatedBy}
}
func pickupScheduleFromPublic(v careplan.PickupSchedule) pickupScheduleRow {
	return pickupScheduleRow{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, PickupTime: v.PickupTime, Notes: v.Notes, CreatedBy: v.CreatedBy, Source: v.Source, CareOfferingID: v.CareOfferingID}
}
func pickupScheduleToPublic(v pickupScheduleRow) careplan.PickupSchedule {
	return careplan.PickupSchedule{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Weekday: v.Weekday, PickupTime: v.PickupTime, Notes: v.Notes, CreatedBy: v.CreatedBy, Source: v.Source, CareOfferingID: v.CareOfferingID}
}
func pickupExceptionFromPublic(v careplan.PickupException) pickupExceptionRow {
	return pickupExceptionRow{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: calendarDate(v.ExceptionDate), PickupTime: v.PickupTime, Reason: v.Reason, ExcusedFrom: v.ExcusedFrom, ExcusedReason: v.ExcusedReason, ExcusedCreatedBy: v.ExcusedCreatedBy, ExcusedOwnsPickupTime: v.ExcusedOwnsPickupTime, ExcusedAuto: v.ExcusedAuto, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func pickupExceptionToPublic(v pickupExceptionRow) careplan.PickupException {
	return careplan.PickupException{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: careplan.Date(v.ExceptionDate), PickupTime: v.PickupTime, Reason: v.Reason, ExcusedFrom: v.ExcusedFrom, ExcusedReason: v.ExcusedReason, ExcusedCreatedBy: v.ExcusedCreatedBy, ExcusedOwnsPickupTime: v.ExcusedOwnsPickupTime, ExcusedAuto: v.ExcusedAuto, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func pickupNoteFromPublic(v careplan.PickupNote) pickupNoteRow {
	return pickupNoteRow{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: calendarDate(v.NoteDate), Content: v.Content, CreatedBy: v.CreatedBy}
}
func pickupNoteToPublic(v pickupNoteRow) careplan.PickupNote {
	return careplan.PickupNote{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, NoteDate: careplan.Date(v.NoteDate), Content: v.Content, CreatedBy: v.CreatedBy}
}
