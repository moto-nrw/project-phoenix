package presenceprojection

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// ManualPlanningOccurrence is one planned care block of a child that a
// manual plan, not a course enrollment, put on the roster.
type ManualPlanningOccurrence struct {
	ActivityGroupID   int64
	ActivityGroupName string
	InstanceID        int64
	Date              string
}

// ListManualPlanningOccurrences lists the manually planned care blocks of a
// child in the date range that no session has started yet and that the
// attendance does not mark as a walk-in or as not scheduled.
func ListManualPlanningOccurrences(ctx context.Context, db bun.IDB, tenantID, studentID int64, from, to timezone.Date) ([]ManualPlanningOccurrence, error) {
	if tenantID <= 0 {
		return nil, ErrInvalidTenantID
	}
	var rows []struct {
		ActivityGroupID   int64
		ActivityGroupName string
		InstanceID        int64
		Date              timezone.Date
	}
	err := db.NewRaw(`
		SELECT activity_group.id AS activity_group_id, activity_group.name AS activity_group_name,
		       activity_instance.id AS instance_id, activity_instance.date
		FROM schedule.instance_students AS instance_student
		LEFT JOIN active.activity_session_attendance AS attendance
		  ON attendance.tenant_id = instance_student.tenant_id AND attendance.instance_student_id = instance_student.id
		JOIN schedule.activity_instances AS activity_instance
		  ON activity_instance.id = instance_student.instance_id AND activity_instance.tenant_id = instance_student.tenant_id
		JOIN activities.groups AS activity_group
		  ON activity_group.id = activity_instance.activity_group_id AND activity_group.tenant_id = activity_instance.tenant_id
		WHERE instance_student.tenant_id = ? AND instance_student.student_id = ?
		  AND COALESCE(attendance.is_unplanned, FALSE) = FALSE AND COALESCE(attendance.not_scheduled, FALSE) = FALSE
		  AND NOT EXISTS (SELECT 1 FROM active.activity_sessions AS session
		    WHERE session.tenant_id = activity_instance.tenant_id AND session.schedule_instance_id = activity_instance.id)
		  AND activity_instance.date BETWEEN ? AND ? AND activity_instance.status = ?
		  AND activity_instance.calendar_period_id IS NOT NULL AND activity_instance.is_spontaneous = FALSE
		  AND activity_group.is_template = TRUE AND activity_group.type = ?
		  AND COALESCE(jsonb_array_length(activity_group.source_care_offering_ids), 0) = 0
		ORDER BY activity_group.name, activity_group.id, activity_instance.date, activity_instance.id`,
		tenantID, studentID, from, to, scheduleModels.InstanceStatusPlanned, activitiesModels.GroupTypeCare).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("presence projection: list manual planning occurrences: %w", err)
	}
	result := make([]ManualPlanningOccurrence, 0, len(rows))
	for _, row := range rows {
		result = append(result, ManualPlanningOccurrence{ActivityGroupID: row.ActivityGroupID, ActivityGroupName: row.ActivityGroupName, InstanceID: row.InstanceID, Date: row.Date.String()})
	}
	return result, nil
}
