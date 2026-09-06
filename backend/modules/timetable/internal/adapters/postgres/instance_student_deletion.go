package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Store) CountStudentAssignments(ctx context.Context, studentID int64) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	count, err := db.NewSelect().Table("schedule.instance_students").
		Where("tenant_id = ?", tenantID).Where("student_id = ?", studentID).Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("timetable postgres: count student assignments: %w", err)
	}
	return count, stats, nil
}

func (s *Store) DeleteStudentAssignments(ctx context.Context, studentID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().Table("schedule.instance_students").
		Where("tenant_id = ?", tenantID).Where("student_id = ?", studentID), "delete student assignments")
}
