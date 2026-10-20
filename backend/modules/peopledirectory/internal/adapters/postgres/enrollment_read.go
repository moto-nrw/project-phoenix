package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentStore) LockEnrollmentClassWrites(ctx context.Context) (domain.OperationStats, error) {
	started := time.Now()
	err := s.classGate(ctx, false)
	return domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}, err
}

func (s *StudentStore) ReadEnrollment(ctx context.Context, id int64, lock string) (peopledirectory.EnrollmentRecord, domain.OperationStats, error) {
	var stats domain.OperationStats
	if lock != "" {
		gateStats, err := s.LockEnrollmentClassWrites(ctx)
		stats.Add(gateStats)
		if err != nil {
			return peopledirectory.EnrollmentRecord{}, stats, err
		}
	}
	row, queryStats, err := s.reads.ReadEnrollment(ctx, id, lock)
	stats.Add(domain.OperationStats(queryStats))
	return peopledirectory.EnrollmentRecord(row), stats, projectionError(err)
}
