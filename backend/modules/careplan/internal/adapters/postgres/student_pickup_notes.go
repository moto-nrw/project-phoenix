package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

// pickupNoteRow is either dated (NoteDate) or recurring on a weekday
// (Weekday, #3369); chk_pickup_note_date_or_weekday enforces exactly one.
type pickupNoteRow struct {
	bun.BaseModel `bun:"table:student_pickup_notes,alias:student_pickup_note"`
	ID            int64         `bun:"id,pk,autoincrement"`
	TenantID      int64         `bun:"tenant_id,notnull"`
	CreatedAt     time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID     int64         `bun:"student_id,notnull"`
	NoteDate      *calendarDate `bun:"note_date,type:date"`
	Weekday       *int          `bun:"weekday"`
	Content       string        `bun:"content,notnull"`
	CreatedBy     int64         `bun:"created_by,notnull"`
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
		// A recurring weekday note (#3369) belongs to every date of that
		// weekday, so the day's notes stay one statement.
		isoWeekday := (int(f.Date.Weekday())+6)%7 + 1 // Monday = 1
		query = query.Where(`("student_pickup_note".note_date = ? OR "student_pickup_note".weekday = ?)`, calendarDate(f.Date), isoWeekday)
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
	query = applyStudentScheduleOptions(query, f.Options, "student_pickup_note")
	if f.Options == nil || len(f.Options.Sorting) == 0 {
		query = query.OrderExpr(`"student_pickup_note".note_date ASC NULLS FIRST, "student_pickup_note".weekday ASC, "student_pickup_note".created_at ASC`)
	}
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

func pickupNoteFromPublic(v careplan.PickupNote) pickupNoteRow {
	row := pickupNoteRow{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Content: v.Content, CreatedBy: v.CreatedBy}
	if v.Weekday != 0 {
		weekday := v.Weekday
		row.Weekday = &weekday
		return row
	}
	noteDate := calendarDate(v.NoteDate)
	row.NoteDate = &noteDate
	return row
}

func pickupNoteToPublic(v pickupNoteRow) careplan.PickupNote {
	note := careplan.PickupNote{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, Content: v.Content, CreatedBy: v.CreatedBy}
	if v.NoteDate != nil {
		note.NoteDate = careplan.Date(*v.NoteDate)
	}
	if v.Weekday != nil {
		note.Weekday = *v.Weekday
	}
	return note
}
