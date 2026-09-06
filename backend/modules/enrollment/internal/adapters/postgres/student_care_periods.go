package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (r *Store) StudentCarePeriods(ctx context.Context, studentID int64) ([]*enrollment.StudentCarePeriod, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student id must be positive")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		RequestChildID   int64           `bun:"request_child_id"`
		RequestID        int64           `bun:"request_id"`
		PhaseID          int64           `bun:"phase_id"`
		PhaseName        string          `bun:"phase_name"`
		ServiceStartDate enrollment.Date `bun:"service_start_date"`
		ServiceEndDate   enrollment.Date `bun:"service_end_date"`
	}
	err = db.NewSelect().TableExpr("enrollment.request_children AS child").
		Join("JOIN enrollment.requests AS request ON request.id = child.request_id AND request.tenant_id = child.tenant_id").
		Join("JOIN enrollment.phases AS phase ON phase.id = request.phase_id AND phase.tenant_id = request.tenant_id").
		ColumnExpr("child.id AS request_child_id, request.id AS request_id, phase.id AS phase_id, phase.name AS phase_name, phase.service_start_date, phase.service_end_date").
		Where("child.tenant_id = ? AND child.created_student_id = ? AND child.status = ?", tenantID, studentID, "approved").
		OrderExpr("phase.service_start_date DESC, child.id DESC").Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to list care periods by student: %w", err)
	}
	periods := make([]*enrollment.StudentCarePeriod, 0, len(rows))
	for _, row := range rows {
		periods = append(periods, &enrollment.StudentCarePeriod{
			RequestChildID: row.RequestChildID, RequestID: row.RequestID, PhaseID: row.PhaseID, PhaseName: row.PhaseName,
			ServiceStartDate: row.ServiceStartDate, ServiceEndDate: row.ServiceEndDate,
		})
	}
	return periods, nil
}
