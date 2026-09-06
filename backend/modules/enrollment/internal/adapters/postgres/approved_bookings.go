package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (r *Store) ApprovedBookings(ctx context.Context) ([]enrollment.ApprovedBooking, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	rows := []enrollment.ApprovedBooking{}
	err = db.NewRaw(`SELECT child.id AS request_child_id,
	 COALESCE(child.created_student_id, child.matched_student_id) AS student_id,
	 phase.id AS phase_id, child.tenant_id, phase.service_start_date,
	 phase.service_end_date, phase.care_offering_selection_mode
	 FROM enrollment.request_children AS child
	 JOIN enrollment.requests AS request ON request.id = child.request_id AND request.tenant_id = child.tenant_id
	 JOIN enrollment.phases AS phase ON phase.id = request.phase_id AND phase.tenant_id = request.tenant_id
	 WHERE child.tenant_id = ? AND child.status = 'approved'
	 ORDER BY child.id`, tenantID).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list approved enrollment bookings: %w", err)
	}
	return rows, nil
}
