package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
	"github.com/uptrace/bun"
)

// ListRetentionCandidates returns the appointments of the school that start
// before cutoff, are neither cancelled nor deleted and are no endless series, each with its recurrence rule and
// the last day an occurrence override moves one of its occurrences to
// (#4045).
func (s *Store) ListRetentionCandidates(ctx context.Context, cutoff appointments.Date) ([]appointments.RetentionCandidate, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "list appointment retention candidates")
	if err != nil {
		return nil, err
	}
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(`"appointment".start_date < ?`, cutoff.String()).
		// Cancelled and deleted appointments keep their 90-day feed window
		// (DeleteFeedTombstonesBefore), however old the appointment is.
		Where(`"appointment".cancelled_at IS NULL AND "appointment".deleted_at IS NULL`).
		Where(`NOT EXISTS (
			SELECT 1 FROM calendar.recurrence_rules AS endless
			WHERE endless.tenant_id = "appointment".tenant_id AND endless.appointment_id = "appointment".id
			  AND endless.ends_on IS NULL AND endless.occurrence_count IS NULL)`).
		OrderExpr(`"appointment".id`)
	if _, err := scanAll(ctx, withTenant(query, "appointment", tenantID), "list appointment retention candidates"); err != nil || len(rows) == 0 {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	rules := []recurrenceRuleRow{}
	ruleQuery := db.NewSelect().Model(&rules).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id IN (?)`, bun.List(ids))
	if _, err := scanAll(ctx, withTenant(ruleQuery, "recurrence_rule", tenantID), "list retention recurrence rules"); err != nil {
		return nil, err
	}
	overrides, err := latestOverrideDays(ctx, db, tenantID, ids)
	if err != nil {
		return nil, err
	}
	rulesByAppointment := make(map[int64]*appointments.RecurrenceRule, len(rules))
	for _, rule := range rules {
		rulesByAppointment[rule.AppointmentID] = &appointments.RecurrenceRule{
			AppointmentID: rule.AppointmentID, Frequency: rule.Frequency, IntervalCount: rule.IntervalCount,
			Weekdays: rule.Weekdays, MonthDays: rule.MonthDays, EndsOn: publicDate(rule.EndsOn), OccurrenceCount: rule.OccurrenceCount,
		}
	}
	candidates := make([]appointments.RetentionCandidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, appointments.RetentionCandidate{
			Appointment: &appointments.Appointment{
				ID: row.ID, TenantID: row.TenantID,
				StartDate: appointments.Date(row.StartDate), EndDate: appointments.Date(row.EndDate),
			},
			Rule:           rulesByAppointment[row.ID],
			LatestOverride: overrides[row.ID],
		})
	}
	return candidates, nil
}

func publicDate(value *domain.Date) *appointments.Date {
	if value == nil {
		return nil
	}
	date := appointments.Date(*value)
	return &date
}

type latestOverrideRow struct {
	AppointmentID int64       `bun:"appointment_id"`
	LastDay       domain.Date `bun:"last_day,type:date"`
}

// latestOverrideDays returns, per appointment, the latest day an occurrence
// override starts or ends on, or the overridden occurrence itself.
func latestOverrideDays(ctx context.Context, db bun.IDB, tenantID int64, ids []int64) (map[int64]appointments.Date, error) {
	rows := []latestOverrideRow{}
	if err := db.NewSelect().
		ColumnExpr(`"override".appointment_id`).
		ColumnExpr(`MAX(GREATEST("override".occurrence_date, "override".start_date, "override".end_date)) AS last_day`).
		TableExpr(`calendar.appointment_occurrence_overrides AS "override"`).
		Where(`"override".tenant_id = ?`, tenantID).
		Where(`"override".appointment_id IN (?)`, bun.List(ids)).
		GroupExpr(`"override".appointment_id`).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("appointments postgres: list latest occurrence overrides: %w", err)
	}
	days := make(map[int64]appointments.Date, len(rows))
	for _, row := range rows {
		days[row.AppointmentID] = appointments.Date(row.LastDay)
	}
	return days, nil
}

// DeleteAppointments deletes the appointments of the school by id; their
// recipients, targets, rules, overrides and reminder claims cascade.
func (s *Store) DeleteAppointments(ctx context.Context, ids []int64) (int, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "delete expired appointments")
	if err != nil {
		return 0, err
	}
	query := db.NewDelete().Model((*appointmentRow)(nil)).ModelTableExpr(appointmentTable).
		Where(`"appointment".id IN (?)`, bun.List(ids))
	_, rows, err := execute(ctx, withTenant(query, "appointment", tenantID), "delete expired appointments")
	return int(rows), err
}
