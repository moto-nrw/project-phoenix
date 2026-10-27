package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

// morningAttendanceRow is one row of active.morning_attendance (#4790).
// Exactly one of StudentID and ClassListEntryID is set.
type morningAttendanceRow struct {
	bun.BaseModel     `bun:"table:active.morning_attendance,alias:morning_attendance"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	Date              ports.Date `bun:"date,notnull,type:date"`
	StudentID         int64      `bun:"student_id,nullzero"`
	ClassListEntryID  int64      `bun:"class_list_entry_id,nullzero"`
	Status            string     `bun:"status,notnull"`
	RecordedByStaffID int64      `bun:"recorded_by_staff_id,nullzero"`
	CreatedAt         time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (s *Store) ListMorningAttendance(ctx context.Context, date ports.Date, studentIDs, classListEntryIDs []int64) ([]studentpresence.MorningAttendanceEntry, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var rows []morningAttendanceRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).
		Where("morning_attendance.tenant_id = ?", tenantID).
		Where("morning_attendance.date = ?", date).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			if len(studentIDs) > 0 {
				q = q.WhereOr("morning_attendance.student_id IN (?)", bun.List(studentIDs))
			}
			if len(classListEntryIDs) > 0 {
				q = q.WhereOr("morning_attendance.class_list_entry_id IN (?)", bun.List(classListEntryIDs))
			}
			return q
		}).
		OrderExpr("morning_attendance.id ASC").
		Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("list morning attendance: %w", err)
	}
	entries := make([]studentpresence.MorningAttendanceEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, studentpresence.MorningAttendanceEntry{
			Child:  studentpresence.MorningAttendanceChild{StudentID: row.StudentID, ClassListEntryID: row.ClassListEntryID},
			Status: row.Status, RecordedAt: row.UpdatedAt,
		})
	}
	return entries, stats, nil
}

// UpsertMorningAttendance writes the students and the class-list entries in
// one statement each: the two kinds of child have separate partial unique
// indexes, and ON CONFLICT names one of them.
func (s *Store) UpsertMorningAttendance(ctx context.Context, date ports.Date, recordedBy int64, marks []studentpresence.MorningAttendanceMark) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	var students, entries []morningAttendanceRow
	for _, mark := range marks {
		row := morningAttendanceRow{
			TenantID: tenantID, Date: date, StudentID: mark.Child.StudentID, ClassListEntryID: mark.Child.ClassListEntryID,
			Status: mark.Status, RecordedByStaffID: recordedBy,
		}
		if row.StudentID > 0 {
			students = append(students, row)
		} else {
			entries = append(entries, row)
		}
	}
	var stats ports.Stats
	if len(students) > 0 {
		query := db.NewInsert().Model(&students).ExcludeColumn("id", "created_at", "updated_at").
			On("CONFLICT (tenant_id, student_id, date) WHERE student_id IS NOT NULL DO UPDATE")
		if err := execMorningAttendanceUpsert(ctx, query, &stats); err != nil {
			return stats, err
		}
	}
	if len(entries) > 0 {
		query := db.NewInsert().Model(&entries).ExcludeColumn("id", "created_at", "updated_at").
			On("CONFLICT (tenant_id, class_list_entry_id, date) WHERE class_list_entry_id IS NOT NULL DO UPDATE")
		if err := execMorningAttendanceUpsert(ctx, query, &stats); err != nil {
			return stats, err
		}
	}
	return stats, nil
}

// execMorningAttendanceUpsert replaces the status and the recorder of a
// conflicting row and adds the statement to stats.
func execMorningAttendanceUpsert(ctx context.Context, query *bun.InsertQuery, stats *ports.Stats) error {
	started := time.Now()
	result, err := query.Set("status = EXCLUDED.status").
		Set("recorded_by_staff_id = EXCLUDED.recorded_by_staff_id").
		Returning("NULL").
		Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return fmt.Errorf("upsert morning attendance: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("upsert morning attendance: %w", err)
	}
	stats.Rows += affected
	return nil
}

// DeleteMorningAttendanceBefore deletes the tenant's marks dated before
// cutoff (#4790). Nothing references a mark, so the rows go alone.
func (s *Store) DeleteMorningAttendanceBefore(ctx context.Context, cutoff ports.Date) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.morning_attendance").Where("tenant_id = ?", tenantID).Where("date < ?", cutoff).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("delete expired morning attendance: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

// LockMorningAttendance serializes a school report with corrections made by
// the teacher. A missing row cannot justify an absence report.
func (s *Store) LockMorningAttendance(ctx context.Context, studentID int64, date ports.Date) (string, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return "", ports.Stats{}, err
	}
	var row morningAttendanceRow
	started := time.Now()
	err = db.NewSelect().Model(&row).Column("status").
		Where("tenant_id = ?", tenantID).Where("student_id = ?", studentID).
		Where("date = ?", date).For("UPDATE").Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return "", stats, nil
	}
	return row.Status, stats, err
}
