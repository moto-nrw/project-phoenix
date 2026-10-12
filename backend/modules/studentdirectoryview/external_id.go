package studentdirectoryview

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ExternalIDHolder is the child that holds a key another system knows it by.
type ExternalIDHolder struct {
	ID     int64
	Status string
}

// FindExternalIDHolder returns the child, graduates included, that holds the
// key another system, such as IServ, knows it by (#1446).
func (s *Projection) FindExternalIDHolder(ctx context.Context, key string) (ExternalIDHolder, bool, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ExternalIDHolder{}, false, OperationStats{}, err
	}
	var holder ExternalIDHolder
	query := withStudentTenant(db.NewSelect().TableExpr(studentSource), tenantID).
		ColumnExpr(`"student".id, "student".status`).
		Where(`"student".external_id = ?`, key)
	stats := OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &holder.ID, &holder.Status)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return ExternalIDHolder{}, false, stats, nil
	}
	if err != nil {
		return ExternalIDHolder{}, false, stats, fmt.Errorf("student directory projection: find external id holder: %w", err)
	}
	stats.Rows = 1
	return holder, true, stats, nil
}
