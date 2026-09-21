package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Store) ListClassArrivalExceptionRecords(ctx context.Context, classes []string, from, to string) ([]domain.ClassArrivalException, domain.OperationStats, error) {
	rows := make([]*schedule.ClassArrivalException, 0)
	query, err := s.classArrivalExceptionQuery(ctx, classes, schedule.Date(from), schedule.Date(to), &rows)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	stats, err := scanAll(ctx, query, "list class arrival exceptions")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.ClassArrivalException, len(rows))
	for i, row := range rows {
		result[i] = classArrivalExceptionValue(row)
	}
	stats.Rows = int64(len(rows))
	return result, stats, nil
}

func classArrivalExceptionValue(row *schedule.ClassArrivalException) domain.ClassArrivalException {
	return domain.ClassArrivalException{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		SchoolClass: row.SchoolClass, Date: string(row.Date), ArrivalTime: row.ArrivalTime, Reason: row.Reason, CreatedBy: row.CreatedBy, Origin: row.Origin}
}

func (s *Store) SaveClassArrivalException(ctx context.Context, input domain.ClassArrivalException) (domain.ClassArrivalException, domain.OperationStats, error) {
	row := &schedule.ClassArrivalException{SchoolClass: input.SchoolClass, Date: schedule.Date(input.Date),
		ArrivalTime: input.ArrivalTime, Reason: input.Reason, CreatedBy: input.CreatedBy, Origin: input.Origin}
	query, err := s.classArrivalExceptionUpsertQuery(ctx, row)
	if err != nil {
		return domain.ClassArrivalException{}, domain.OperationStats{}, err
	}
	// Bind a clock string so bun cannot convert an arbitrary date/location
	// through UTC before PostgreSQL stores the TIME value.
	query = query.Value("arrival_time", "?::time", input.ArrivalTime.Format("15:04:05.999999999"))
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.ClassArrivalException{}, stats, classifyWriteError("upsert class arrival exception", err, &stats)
	}
	stats.Rows = 1
	return classArrivalExceptionValue(row), stats, nil
}

func (s *Store) RemoveClassArrivalException(ctx context.Context, class, date string) (bool, domain.OperationStats, error) {
	query, err := s.classArrivalExceptionDeleteQuery(ctx, class, schedule.Date(date))
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, classifyWriteError("delete class arrival exception", err, &stats)
	}
	stats.Rows, err = result.RowsAffected()
	return stats.Rows > 0, stats, err
}
