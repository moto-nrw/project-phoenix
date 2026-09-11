// Package timetableenrollmentprovenance is the narrow read projection that validates
// links from timetable enrollments back to their enrollment request source.
package timetableenrollmentprovenance

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func EligibleEnrollmentIDs(ctx context.Context, db bun.IDB, tenantID, studentID, requestChildID int64, groupIDs []int64) ([]int64, error) {
	var ids []int64
	err := db.NewSelect().
		TableExpr("activities.student_enrollments AS student_enrollment").
		ColumnExpr("student_enrollment.id").
		Join(`JOIN enrollment.request_children AS request_child
		  ON request_child.tenant_id = student_enrollment.tenant_id
		 AND request_child.created_student_id = student_enrollment.student_id`).
		Join(`JOIN enrollment.requests AS request
		  ON request.tenant_id = request_child.tenant_id AND request.id = request_child.request_id`).
		Join(`JOIN enrollment.phases AS phase
		  ON phase.tenant_id = request_child.tenant_id AND phase.id = request.phase_id`).
		Where("student_enrollment.tenant_id = ?", tenantID).
		Where("student_enrollment.student_id = ?", studentID).
		Where("student_enrollment.enrollment_request_child_id IS NULL").
		Where("request_child.id = ? AND request_child.status = 'approved'", requestChildID).
		Where("request_child.reviewed_at IS NOT NULL").
		Where("student_enrollment.created_at BETWEEN request_child.reviewed_at - INTERVAL '5 minutes' AND request_child.reviewed_at + INTERVAL '5 minutes'").
		Where(`student_enrollment.activity_group_id IN (?) OR (
			student_enrollment.valid_from = phase.service_start_date AND (
			student_enrollment.valid_until = phase.service_end_date OR
			(student_enrollment.valid_until IS NULL AND phase.service_end_date IS NULL) OR
			student_enrollment.valid_until = (phase.service_end_date + 1)))`, bun.List(groupIDs)).
		For("UPDATE OF student_enrollment").
		Scan(ctx, &ids)
	if err != nil {
		return nil, fmt.Errorf("enrollment provenance: list eligible timetable enrollments: %w", err)
	}
	return ids, nil
}

func ExistingRequestChildIDs(ctx context.Context, db bun.IDB, tenantID int64, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}
	var existing []int64
	err := db.NewRaw(`SELECT request_child.id
		FROM enrollment.request_children AS request_child
		WHERE request_child.tenant_id = ? AND request_child.id IN (?)`,
		tenantID, bun.List(ids)).Scan(ctx, &existing)
	if err != nil {
		return nil, fmt.Errorf("enrollment provenance: list existing request children: %w", err)
	}
	return existing, nil
}
