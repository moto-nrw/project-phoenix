package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

// classHandoverRow is one row of active.class_handovers (#4794).
type classHandoverRow struct {
	bun.BaseModel       `bun:"table:active.class_handovers,alias:class_handover"`
	ID                  int64      `bun:"id,pk,autoincrement"`
	TenantID            int64      `bun:"tenant_id,notnull"`
	SchoolClass         string     `bun:"school_class,notnull"`
	Date                ports.Date `bun:"date,notnull,type:date"`
	HandedOverAt        time.Time  `bun:"handed_over_at,notnull"`
	HandedOverByStaffID int64      `bun:"handed_over_by_staff_id,nullzero"`
}

// classHandoverChildRow is one row of active.class_handover_children.
type classHandoverChildRow struct {
	bun.BaseModel `bun:"table:active.class_handover_children,alias:class_handover_child"`
	ID            int64 `bun:"id,pk,autoincrement"`
	TenantID      int64 `bun:"tenant_id,notnull"`
	HandoverID    int64 `bun:"handover_id,notnull"`
	StudentID     int64 `bun:"student_id,notnull"`
	Absent        bool  `bun:"absent,notnull"`
}

// classHandoverClassMatch is the predicate the unique index matches a class
// with: trimmed and lowercased, the normalized class bound to it.
const classHandoverClassMatch = "LOWER(BTRIM(class_handover.school_class)) = ?"

// classHandoverReadRow includes children loaded in the parent's statement snapshot.
type classHandoverReadRow struct {
	classHandoverRow
	PresentStudentIDs []int64 `bun:"present_student_ids,array"`
	AbsentStudentIDs  []int64 `bun:"absent_student_ids,array"`
}

func (s *Store) ListClassHandovers(ctx context.Context, date ports.Date) ([]studentpresence.ClassHandover, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var rows []classHandoverReadRow
	started := time.Now()
	// A single statement prevents replacement/withdrawal between reading the
	// handover and its children, including under READ COMMITTED isolation.
	err = db.NewSelect().TableExpr(`active.class_handovers AS "class_handover"`).
		ColumnExpr("class_handover.id, class_handover.tenant_id, class_handover.school_class, class_handover.date, class_handover.handed_over_at, class_handover.handed_over_by_staff_id").
		ColumnExpr("COALESCE(array_agg(child.student_id ORDER BY child.student_id) FILTER (WHERE NOT child.absent), '{}'::bigint[]) AS present_student_ids").
		ColumnExpr("COALESCE(array_agg(child.student_id ORDER BY child.student_id) FILTER (WHERE child.absent), '{}'::bigint[]) AS absent_student_ids").
		Join("LEFT JOIN active.class_handover_children AS child ON child.handover_id = class_handover.id AND child.tenant_id = class_handover.tenant_id").
		Where("class_handover.tenant_id = ?", tenantID).
		Where("class_handover.date = ?", date).
		GroupExpr("class_handover.id").
		OrderExpr("class_handover.id ASC").
		Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("list class handovers: %w", err)
	}
	handovers := make([]studentpresence.ClassHandover, 0, len(rows))
	for _, row := range rows {
		handovers = append(handovers, studentpresence.ClassHandover{
			SchoolClass: row.SchoolClass, Date: row.Date, HandedOverAt: row.HandedOverAt, HandedOverByStaffID: row.HandedOverByStaffID,
			PresentStudentIDs: row.PresentStudentIDs,
			AbsentStudentIDs:  row.AbsentStudentIDs,
		})
	}
	return handovers, stats, nil
}

func (s *Store) ReplaceClassHandover(ctx context.Context, in studentpresence.ClassHandoverChange, normalizedClass string, at time.Time) (bool, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, ports.Stats{}, err
	}
	stats, err := s.DeleteClassHandover(ctx, normalizedClass, in.Date)
	if err != nil {
		return false, stats, err
	}
	replaced := stats.Rows > 0
	row := classHandoverRow{
		TenantID: tenantID, SchoolClass: in.SchoolClass, Date: in.Date, HandedOverAt: at, HandedOverByStaffID: in.HandedOverBy,
	}
	started := time.Now()
	_, err = db.NewInsert().Model(&row).Returning("id").Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("insert class handover: %w", err)
	}
	stats.Rows++
	children := make([]classHandoverChildRow, 0, len(in.PresentStudentIDs)+len(in.AbsentStudentIDs))
	for _, studentID := range in.PresentStudentIDs {
		children = append(children, classHandoverChildRow{TenantID: tenantID, HandoverID: row.ID, StudentID: studentID})
	}
	for _, studentID := range in.AbsentStudentIDs {
		children = append(children, classHandoverChildRow{TenantID: tenantID, HandoverID: row.ID, StudentID: studentID, Absent: true})
	}
	if len(children) == 0 {
		return replaced, stats, nil
	}
	started = time.Now()
	_, err = db.NewInsert().Model(&children).ExcludeColumn("id").Returning("NULL").Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("insert class handover children: %w", err)
	}
	stats.Rows += int64(len(children))
	return replaced, stats, nil
}

// DeleteClassHandover deletes the handover; its children go with it through
// ON DELETE CASCADE.
func (s *Store) DeleteClassHandover(ctx context.Context, normalizedClass string, date ports.Date) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	// Lock before DELETE even when no row exists yet. Replacements call this
	// method too, so both replacement and withdrawal hold the same lock until
	// their surrounding tenant transaction commits or rolls back.
	key := fmt.Sprintf("class-handover:%d:%s:%s", tenantID, date, normalizedClass)
	started := time.Now()
	_, err = db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("lock class handover: %w", err)
	}
	started = time.Now()
	result, err := db.NewDelete().Model((*classHandoverRow)(nil)).
		Where("class_handover.tenant_id = ?", tenantID).
		Where(classHandoverClassMatch, normalizedClass).
		Where("class_handover.date = ?", date).
		Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("delete class handover: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

// DeleteClassHandoversBefore deletes the tenant's handovers dated before
// cutoff; their children go with them through ON DELETE CASCADE.
func (s *Store) DeleteClassHandoversBefore(ctx context.Context, cutoff ports.Date) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.class_handovers").Where("tenant_id = ?", tenantID).Where("date < ?", cutoff).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("delete expired class handovers: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}
