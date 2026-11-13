package postgres

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
)

type VisitAdmissionReader struct{ database Database }

func NewVisitAdmissionReader(database Database) *VisitAdmissionReader {
	if database == nil {
		panic("student presence admission reader: database is required")
	}
	return &VisitAdmissionReader{database: database}
}

// ReadVisitAdmission combines two independent indexed reads after the caller
// acquired the session lock. Combining them with the locking statement would
// reuse its pre-wait snapshot and could miss a concurrent admission.
func (s *VisitAdmissionReader) ReadVisitAdmission(ctx context.Context, studentID, groupID int64) (ports.VisitAdmission, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.VisitAdmission{}, ports.Stats{}, err
	}
	started := time.Now()
	var result ports.VisitAdmission
	err = db.NewRaw(`
		SELECT EXISTS (
			SELECT 1 FROM active.visits
			WHERE tenant_id = ? AND student_id = ? AND exit_time IS NULL
		) AS has_open_visit, (
			SELECT count(*) FROM active.visits
			WHERE tenant_id = ? AND active_group_id = ? AND exit_time IS NULL
		) AS group_occupancy
	`, tenantID, studentID, tenantID, groupID).Scan(ctx, &result)
	return result, ports.Stats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}
