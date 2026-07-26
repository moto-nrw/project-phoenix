package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	sourcedEnrollmentExclusiveEndVersion     = "1.15.186"
	sourcedEnrollmentExclusiveEndDescription = "Repair enrollment-sourced roster end dates to use exclusive phase bounds"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     sourcedEnrollmentExclusiveEndVersion,
		Description: sourcedEnrollmentExclusiveEndDescription,
		DependsOn:   []string{groupCalendarPeriodTenantFKVersion},
	})
	Migrations.MustRegister(sourcedEnrollmentExclusiveEndUp, sourcedEnrollmentExclusiveEndDown)
}

func sourcedEnrollmentExclusiveEndUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.186: Repairing sourced enrollment exclusive end dates...")
	if err := repairSourcedEnrollmentExclusiveEnds(ctx, db); err != nil {
		return err
	}
	fmt.Println("Migration 1.15.186: Completed successfully")
	return nil
}

func repairSourcedEnrollmentExclusiveEnds(ctx context.Context, db bun.IDB) error {
	_, err := db.NewRaw(`
		UPDATE activities.student_enrollments AS se
		SET valid_until = p.service_end_date + 1
		FROM enrollment.request_children AS rc
		INNER JOIN enrollment.requests AS r
			ON r.id = rc.request_id
			AND r.tenant_id = rc.tenant_id
		INNER JOIN enrollment.phases AS p
			ON p.id = r.phase_id
			AND p.tenant_id = r.tenant_id
		WHERE se.enrollment_request_child_id = rc.id
			AND se.tenant_id = rc.tenant_id
			AND se.student_id = rc.created_student_id
			AND se.valid_until = p.service_end_date
			AND se.valid_from <= p.service_end_date;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed repairing sourced enrollment exclusive end dates: %w", err)
	}
	return nil
}

func sourcedEnrollmentExclusiveEndDown(_ context.Context, _ *bun.DB) error {
	// Intentionally irreversible: after Up, correctly-created +1 rows are
	// indistinguishable from repaired rows. Reverting all of them would
	// reintroduce final-day child loss.
	return nil
}
