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

// classHandoverAbsenceRow is one row of active.class_handover_absences.
type classHandoverAbsenceRow struct {
	bun.BaseModel `bun:"table:active.class_handover_absences,alias:class_handover_absence"`
	ID            int64 `bun:"id,pk,autoincrement"`
	TenantID      int64 `bun:"tenant_id,notnull"`
	HandoverID    int64 `bun:"handover_id,notnull"`
	StudentID     int64 `bun:"student_id,notnull"`
}

// classHandoverClassMatch is the predicate the unique index matches a class
// with: trimmed and lowercased, the normalized class bound to it.
const classHandoverClassMatch = "LOWER(BTRIM(class_handover.school_class)) = ?"

func (s *Store) ListClassHandovers(ctx context.Context, date ports.Date) ([]ports.ClassHandoverRecord, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var rows []classHandoverRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).
		Where("class_handover.tenant_id = ?", tenantID).
		Where("class_handover.date = ?", date).
		OrderExpr("class_handover.id ASC").
		Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("list class handovers: %w", err)
	}
	records := make([]ports.ClassHandoverRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, ports.ClassHandoverRecord{ID: row.ID, ClassHandover: studentpresence.ClassHandover{
			SchoolClass: row.SchoolClass, Date: row.Date, HandedOverAt: row.HandedOverAt, HandedOverByStaffID: row.HandedOverByStaffID,
		}})
	}
	return records, stats, nil
}

func (s *Store) ListClassHandoverAbsences(ctx context.Context, handoverIDs []int64) (map[int64][]int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var rows []classHandoverAbsenceRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).
		Where("class_handover_absence.tenant_id = ?", tenantID).
		Where("class_handover_absence.handover_id IN (?)", bun.List(handoverIDs)).
		OrderExpr("class_handover_absence.student_id ASC").
		Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("list class handover absences: %w", err)
	}
	absent := make(map[int64][]int64, len(handoverIDs))
	for _, row := range rows {
		absent[row.HandoverID] = append(absent[row.HandoverID], row.StudentID)
	}
	return absent, stats, nil
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
	if len(in.AbsentStudentIDs) == 0 {
		return replaced, stats, nil
	}
	absences := make([]classHandoverAbsenceRow, 0, len(in.AbsentStudentIDs))
	for _, studentID := range in.AbsentStudentIDs {
		absences = append(absences, classHandoverAbsenceRow{TenantID: tenantID, HandoverID: row.ID, StudentID: studentID})
	}
	started = time.Now()
	_, err = db.NewInsert().Model(&absences).ExcludeColumn("id").Returning("NULL").Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("insert class handover absences: %w", err)
	}
	stats.Rows += int64(len(absences))
	return replaced, stats, nil
}

// DeleteClassHandover deletes the handover; its absences go with it through
// ON DELETE CASCADE.
func (s *Store) DeleteClassHandover(ctx context.Context, normalizedClass string, date ports.Date) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Model((*classHandoverRow)(nil)).
		Where("class_handover.tenant_id = ?", tenantID).
		Where(classHandoverClassMatch, normalizedClass).
		Where("class_handover.date = ?", date).
		Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("delete class handover: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

// DeleteClassHandoversBefore deletes the tenant's handovers dated before
// cutoff; their absences go with them through ON DELETE CASCADE.
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
