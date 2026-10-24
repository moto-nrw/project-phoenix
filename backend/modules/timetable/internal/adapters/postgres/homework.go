package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// classHomeworkRow is one row of education.class_homework (#4796).
type classHomeworkRow struct {
	bun.BaseModel    `bun:"table:education.class_homework,alias:class_homework"`
	ID               int64         `bun:"id,pk,autoincrement"`
	TenantID         int64         `bun:"tenant_id,notnull"`
	SchoolClass      string        `bun:"school_class,notnull"`
	Date             calendar.Date `bun:"date,notnull,type:date"`
	Subject          *string       `bun:"subject"`
	Description      string        `bun:"description,notnull"`
	CreatedByStaffID *int64        `bun:"created_by_staff_id"`
	CreatedAt        time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// homeworkFeedbackRow is one row of education.homework_feedback (#4796).
type homeworkFeedbackRow struct {
	bun.BaseModel `bun:"table:education.homework_feedback,alias:homework_feedback"`
	ID            int64         `bun:"id,pk,autoincrement"`
	TenantID      int64         `bun:"tenant_id,notnull"`
	StudentID     int64         `bun:"student_id,notnull"`
	Date          calendar.Date `bun:"date,notnull,type:date"`
	Status        string        `bun:"status,notnull"`
	Note          *string       `bun:"note"`
	CreatedAt     time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (row classHomeworkRow) entry() timetable.HomeworkEntry {
	return timetable.HomeworkEntry{
		ID: row.ID, SchoolClass: row.SchoolClass, Date: row.Date, Subject: row.Subject, Description: row.Description,
		CreatedBy: row.CreatedByStaffID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func (s *Store) ListClassHomework(ctx context.Context, classKeys []string, date calendar.Date) ([]timetable.HomeworkEntry, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []classHomeworkRow
	query := db.NewSelect().Model(&rows).
		Where("class_homework.tenant_id = ?", tenantID).
		Where("class_homework.date = ?", date).
		Where("LOWER(BTRIM(class_homework.school_class)) IN (?)", bun.List(classKeys)).
		OrderExpr("class_homework.created_at ASC, class_homework.id ASC")
	stats, err := scanAll(ctx, query, "list class homework")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	entries := make([]timetable.HomeworkEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, row.entry())
	}
	return entries, stats, nil
}

func (s *Store) InsertClassHomework(ctx context.Context, in timetable.HomeworkInput) (timetable.HomeworkEntry, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return timetable.HomeworkEntry{}, domain.OperationStats{}, err
	}
	staffID := in.StaffID
	row := classHomeworkRow{
		TenantID: tenantID, SchoolClass: in.SchoolClass, Date: in.Date, Subject: in.Subject,
		Description: in.Description, CreatedByStaffID: &staffID,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ExcludeColumn("id", "created_at", "updated_at").Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return timetable.HomeworkEntry{}, stats, fmt.Errorf("timetable postgres: insert class homework: %w", err)
	}
	stats.Rows = 1
	return row.entry(), stats, nil
}

// UpdateOwnClassHomework replaces subject and text of the author's entry of
// the class; another author's or another class's entry is not found.
func (s *Store) UpdateOwnClassHomework(ctx context.Context, in timetable.HomeworkChange, classKey string) (timetable.HomeworkEntry, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return timetable.HomeworkEntry{}, false, domain.OperationStats{}, err
	}
	row := classHomeworkRow{}
	query := db.NewUpdate().Model(&row).
		Set("subject = ?", in.Subject).
		Set("description = ?", in.Description).
		Where("class_homework.tenant_id = ?", tenantID).
		Where("class_homework.id = ?", in.ID).
		Where("LOWER(BTRIM(class_homework.school_class)) = ?", classKey).
		Where("class_homework.created_by_staff_id = ?", in.StaffID).
		Returning("*")
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return timetable.HomeworkEntry{}, false, stats, nil
	}
	if err != nil {
		return timetable.HomeworkEntry{}, false, stats, fmt.Errorf("timetable postgres: update class homework: %w", err)
	}
	stats.Rows = 1
	return row.entry(), true, stats, nil
}

// DeleteOwnClassHomework deletes the author's entry of the class.
func (s *Store) DeleteOwnClassHomework(ctx context.Context, ref timetable.HomeworkRef, classKey string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewDelete().TableExpr("education.class_homework").
		Where("tenant_id = ?", tenantID).
		Where("id = ?", ref.ID).
		Where("LOWER(BTRIM(school_class)) = ?", classKey).
		Where("created_by_staff_id = ?", ref.StaffID).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("timetable postgres: delete class homework: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats.Rows > 0, stats, err
}

func (s *Store) ListHomeworkFeedback(ctx context.Context, date calendar.Date, studentIDs []int64) ([]timetable.HomeworkFeedback, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []homeworkFeedbackRow
	query := db.NewSelect().Model(&rows).
		Where("homework_feedback.tenant_id = ?", tenantID).
		Where("homework_feedback.date = ?", date).
		Where("homework_feedback.student_id IN (?)", bun.List(studentIDs)).
		OrderExpr("homework_feedback.student_id ASC")
	stats, err := scanAll(ctx, query, "list homework feedback")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	feedback := make([]timetable.HomeworkFeedback, 0, len(rows))
	for _, row := range rows {
		feedback = append(feedback, row.feedback())
	}
	return feedback, stats, nil
}

func (row homeworkFeedbackRow) feedback() timetable.HomeworkFeedback {
	return timetable.HomeworkFeedback{StudentID: row.StudentID, Date: row.Date, Status: row.Status, Note: row.Note, UpdatedAt: row.UpdatedAt}
}

// UpsertHomeworkFeedback stores the child's feedback of the day; a second
// feedback of the same day replaces the first.
func (s *Store) UpsertHomeworkFeedback(ctx context.Context, in timetable.HomeworkFeedbackInput) (timetable.HomeworkFeedback, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return timetable.HomeworkFeedback{}, domain.OperationStats{}, err
	}
	row := homeworkFeedbackRow{TenantID: tenantID, StudentID: in.StudentID, Date: in.Date, Status: in.Status, Note: in.Note}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ExcludeColumn("id", "created_at", "updated_at").
		On("CONFLICT (tenant_id, student_id, date) DO UPDATE").
		Set("status = EXCLUDED.status").
		Set("note = EXCLUDED.note").
		Returning("*").
		Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return timetable.HomeworkFeedback{}, stats, fmt.Errorf("timetable postgres: upsert homework feedback: %w", err)
	}
	stats.Rows = 1
	return row.feedback(), stats, nil
}

// DeleteHomeworkBefore deletes the tenant's feedback and entries dated
// before cutoff. Nothing references either table.
func (s *Store) DeleteHomeworkBefore(ctx context.Context, cutoff calendar.Date) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	feedback, err := execHomeworkDelete(ctx, db.NewDelete().TableExpr("education.homework_feedback").
		Where("tenant_id = ?", tenantID).Where("date < ?", cutoff), &stats)
	if err != nil {
		return 0, stats, fmt.Errorf("timetable postgres: delete expired homework feedback: %w", err)
	}
	entries, err := execHomeworkDelete(ctx, db.NewDelete().TableExpr("education.class_homework").
		Where("tenant_id = ?", tenantID).Where("date < ?", cutoff), &stats)
	if err != nil {
		return int(feedback), stats, fmt.Errorf("timetable postgres: delete expired class homework: %w", err)
	}
	stats.Rows = feedback + entries
	return int(stats.Rows), stats, nil
}

// execHomeworkDelete runs one delete and adds it to stats.
func execHomeworkDelete(ctx context.Context, query *bun.DeleteQuery, stats *domain.OperationStats) (int64, error) {
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
