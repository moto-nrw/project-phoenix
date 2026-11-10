package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (r *Store) UpdateChildActivationPlan(ctx context.Context, id int64, mode string, on *enrollment.Date) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	result, err := db.NewUpdate().TableExpr("enrollment.request_children").
		Set("activation_mode = ?", mode).Set("activate_on = ?", on).
		Set("updated_at = ?", time.Now()).Where("tenant_id = ? AND id = ?", tenantID, id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child activation plan: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("request child %d not found", id)
	}
	return nil
}

// PhaseActivationPlans lists the open activation plans of every student an
// approved request child of the phase plans to activate on: those plans of
// the phase and the students' plans in other phases, ordered by student and
// request child (#6505). An activation plan is open while its student is
// pending; the caller checks that under the student's row lock.
func (r *Store) PhaseActivationPlans(ctx context.Context, phaseID int64, on enrollment.Date) ([]enrollment.ActivationPlan, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	plans := []enrollment.ActivationPlan{}
	err = db.NewRaw(`SELECT child.id AS request_child_id, child.created_student_id AS student_id,
	 request.phase_id, child.activate_on AS "on", phase.service_end_date AS phase_end
	 FROM enrollment.request_children AS child
	 JOIN enrollment.requests AS request ON request.id = child.request_id AND request.tenant_id = child.tenant_id
	 JOIN enrollment.phases AS phase ON phase.id = request.phase_id AND phase.tenant_id = request.tenant_id
	 WHERE child.tenant_id = ?0 AND child.status = ?1 AND child.activation_mode = ?2
	   AND child.activate_on IS NOT NULL
	   AND child.created_student_id IN (
	     SELECT planned.created_student_id
	     FROM enrollment.request_children AS planned
	     JOIN enrollment.requests AS planned_request ON planned_request.id = planned.request_id AND planned_request.tenant_id = planned.tenant_id
	     WHERE planned.tenant_id = ?0 AND planned_request.phase_id = ?3 AND planned.status = ?1
	       AND planned.activation_mode = ?2 AND planned.activate_on = ?4::date
	       AND planned.created_student_id IS NOT NULL)
	 ORDER BY child.created_student_id, child.id`,
		tenantID, enrollment.ChildStatusApproved, enrollment.ChildActivationScheduled, phaseID, on).Scan(ctx, &plans)
	if err != nil {
		return nil, fmt.Errorf("list phase activation plans: %w", err)
	}
	return plans, nil
}
