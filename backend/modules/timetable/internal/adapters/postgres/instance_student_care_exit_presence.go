package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) ListOpenStudentAssignments(ctx context.Context, studentIDs []int64) ([]int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	ids := []int64{}
	query := db.NewSelect().Table("schedule.instance_students").
		ColumnExpr("DISTINCT student_id").Where("tenant_id = ?", tenantID).
		Where("student_id IN (?)", bun.List(studentIDs)).
		Where("checked_in_at IS NOT NULL").Where("checked_out_at IS NULL").OrderExpr("student_id")
	stats, err := scanAllInto(ctx, query, &ids, "list open student assignments")
	return ids, stats, err
}

func (s *Store) LatestStudentAssignmentAttendanceDate(ctx context.Context, studentID int64) (*string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var day *string
	started := time.Now()
	err = db.NewRaw(`
 SELECT MAX(instance.date)::text
 FROM schedule.instance_students AS roster
 JOIN schedule.activity_instances AS instance
 ON instance.tenant_id = roster.tenant_id AND instance.id = roster.instance_id
 WHERE roster.tenant_id = ? AND roster.student_id = ? AND roster.checked_in_at IS NOT NULL
 `, tenantID, studentID).Scan(ctx, &day)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: latest student assignment attendance date: %w", err)
	}
	return day, stats, nil
}

func (s *Store) CloseOpenStudentAssignments(ctx context.Context, studentIDs []int64, at time.Time) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, db.NewUpdate().Table("schedule.instance_students").
		Set("checked_out_at = ?", at).Set("updated_at = ?", at).
		Where("tenant_id = ?", tenantID).Where("student_id IN (?)", bun.List(studentIDs)).
		Where("checked_in_at IS NOT NULL").Where("checked_out_at IS NULL"), "close open student assignments")
	return stats.Rows, stats, err
}
