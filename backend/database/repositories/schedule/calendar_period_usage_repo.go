package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CalendarPeriodUsageRepository counts how many planning objects reference a
// calendar period. The periods themselves belong to the School Calendar
// owner (#2666); this repository only reads the referencing planning tables,
// so a period without references is simply absent from the result.
type CalendarPeriodUsageRepository struct {
	db         *bun.DB
	enrollment EnrollmentPhaseQueries
}

type EnrollmentPhaseQueries interface {
	PhaseCountsByCalendarPeriod(context.Context) (map[int64]int, error)
}

// NewCalendarPeriodUsageRepository creates a new CalendarPeriodUsageRepository.
func NewCalendarPeriodUsageRepository(db *bun.DB, enrollment EnrollmentPhaseQueries) *CalendarPeriodUsageRepository {
	if enrollment == nil {
		panic("calendar period usage: enrollment queries are required")
	}
	return &CalendarPeriodUsageRepository{db: db, enrollment: enrollment}
}

// UsageCounts returns, per calendar period of the current tenant, how many
// rows reference it through nullable calendar_period_id FKs. Periods without
// references are omitted from the map. Enrollment supplies its own phase
// references; the remaining planning references share one SQL query.
func (r *CalendarPeriodUsageRepository) UsageCounts(ctx context.Context) (map[int64]schedule.CalendarPeriodUsage, error) {
	phaseCounts, err := r.enrollment.PhaseCountsByCalendarPeriod(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "usage counts", Err: err}
	}
	var rows []struct {
		Source           string `bun:"source"`
		CalendarPeriodID int64  `bun:"calendar_period_id"`
		Count            int    `bun:"count"`
	}

	var tenantID *int64
	if id := modelBase.RepositoryTenantID(ctx); id > 0 {
		tenantID = &id
	} else if id := tenant.FromContext(ctx); id > 0 {
		tenantID = &id
	}
	err = base.GetDB(ctx, r.db).NewRaw(`
		SELECT 'activity_group' AS source, g.calendar_period_id, COUNT(*)::int AS count
		FROM activities.groups AS g
		WHERE g.calendar_period_id IS NOT NULL AND (?::BIGINT IS NULL OR g.tenant_id = ?)
		GROUP BY g.calendar_period_id
		UNION ALL
		SELECT 'schedule' AS source, s.calendar_period_id, COUNT(*)::int AS count
		FROM activities.schedules AS s
		WHERE s.calendar_period_id IS NOT NULL AND (?::BIGINT IS NULL OR s.tenant_id = ?)
		GROUP BY s.calendar_period_id
		UNION ALL
		SELECT 'student_enrollment' AS source, se.calendar_period_id, COUNT(*)::int AS count
		FROM activities.student_enrollments AS se
		WHERE se.calendar_period_id IS NOT NULL AND (?::BIGINT IS NULL OR se.tenant_id = ?)
		GROUP BY se.calendar_period_id
		UNION ALL
		SELECT 'supervisor' AS source, sp.calendar_period_id, COUNT(*)::int AS count
		FROM activities.supervisors AS sp
		WHERE sp.calendar_period_id IS NOT NULL AND (?::BIGINT IS NULL OR sp.tenant_id = ?)
		GROUP BY sp.calendar_period_id
		UNION ALL
		SELECT 'activity_instance' AS source, ai.calendar_period_id, COUNT(*)::int AS count
		FROM schedule.activity_instances AS ai
		WHERE ai.calendar_period_id IS NOT NULL AND (?::BIGINT IS NULL OR ai.tenant_id = ?)
		GROUP BY ai.calendar_period_id
	`, tenantID, tenantID, tenantID, tenantID,
		tenantID, tenantID, tenantID, tenantID, tenantID, tenantID,
	).Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "usage counts",
			Err: base.TranslateNotFound(err),
		}
	}

	usage := make(map[int64]schedule.CalendarPeriodUsage, len(rows))
	for id, count := range phaseCounts {
		usage[id] = schedule.CalendarPeriodUsage{EnrollmentPhases: count}
	}
	for _, row := range rows {
		entry := usage[row.CalendarPeriodID]
		switch row.Source {
		case "activity_group":
			entry.ActivityGroups += row.Count
		case "schedule":
			entry.Schedules += row.Count
		case "student_enrollment":
			entry.StudentEnrollments += row.Count
		case "supervisor":
			entry.Supervisors += row.Count
		case "activity_instance":
			entry.ActivityInstances += row.Count
		}
		usage[row.CalendarPeriodID] = entry
	}
	return usage, nil
}
