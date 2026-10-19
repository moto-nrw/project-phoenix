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
	return s.retentionCandidates(ctx, cutoff, nil, false)
}

// LockRetentionCandidates rereads candidates immediately before deletion and
// keeps every row that can move an appointment's last day locked until the
// enclosing tenant transaction commits. New rules and overrides need a key
// share lock on the appointment, so its UPDATE lock also prevents them from
// appearing after the reread.
func (s *Store) LockRetentionCandidates(ctx context.Context, cutoff appointments.Date, ids []int64) ([]appointments.RetentionCandidate, error) {
	if len(ids) == 0 {
		return []appointments.RetentionCandidate{}, nil
	}
	return s.retentionCandidates(ctx, cutoff, ids, true)
}

func (s *Store) retentionCandidates(ctx context.Context, cutoff appointments.Date, ids []int64, lock bool) ([]appointments.RetentionCandidate, error) {
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
	if len(ids) > 0 {
		query = query.Where(`"appointment".id IN (?)`, bun.List(ids))
	}
	if lock {
		query = query.For("UPDATE")
	}
	if _, err := scanAll(ctx, withTenant(query, "appointment", tenantID), "list appointment retention candidates"); err != nil || len(rows) == 0 {
		return nil, err
	}
	appointmentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		appointmentIDs = append(appointmentIDs, row.ID)
	}
	rules := []recurrenceRuleRow{}
	ruleQuery := db.NewSelect().Model(&rules).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id IN (?)`, bun.List(appointmentIDs))
	if lock {
		ruleQuery = ruleQuery.For("UPDATE")
	}
	if _, err := scanAll(ctx, withTenant(ruleQuery, "recurrence_rule", tenantID), "list retention recurrence rules"); err != nil {
		return nil, err
	}
	overrides, err := latestOverrideDays(ctx, db, tenantID, appointmentIDs, lock)
	if err != nil {
		return nil, err
	}
	return retentionCandidatesFromRows(rows, rules, overrides), nil
}

func retentionCandidatesFromRows(rows []appointmentRow, rules []recurrenceRuleRow, overrides map[int64]appointments.Date) []appointments.RetentionCandidate {
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
	return candidates
}

func publicDate(value *domain.Date) *appointments.Date {
	if value == nil {
		return nil
	}
	date := appointments.Date(*value)
	return &date
}

type latestOverrideRow struct {
	AppointmentID  int64        `bun:"appointment_id"`
	OccurrenceDate domain.Date  `bun:"occurrence_date,type:date"`
	StartDate      *domain.Date `bun:"start_date,type:date"`
	EndDate        *domain.Date `bun:"end_date,type:date"`
}

// latestOverrideDays returns, per appointment, the latest day an occurrence
// override starts or ends on, or the overridden occurrence itself.
func latestOverrideDays(ctx context.Context, db bun.IDB, tenantID int64, ids []int64, lock bool) (map[int64]appointments.Date, error) {
	rows := []latestOverrideRow{}
	query := db.NewSelect().
		ColumnExpr(`"override".appointment_id, "override".occurrence_date, "override".start_date, "override".end_date`).
		TableExpr(`calendar.appointment_occurrence_overrides AS "override"`).
		Where(`"override".tenant_id = ?`, tenantID).
		Where(`"override".appointment_id IN (?)`, bun.List(ids))
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("appointments postgres: list latest occurrence overrides: %w", err)
	}
	days := make(map[int64]appointments.Date, len(rows))
	for _, row := range rows {
		day := appointments.Date(row.OccurrenceDate)
		for _, date := range []*domain.Date{row.StartDate, row.EndDate} {
			if date != nil && appointments.Date(*date).After(day) {
				day = appointments.Date(*date)
			}
		}
		if day.After(days[row.AppointmentID]) {
			days[row.AppointmentID] = day
		}
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
		Where(`"appointment".id IN (?)`, bun.List(ids)).
		// A cancellation or soft deletion can happen after the candidate scan.
		// Keep that fresh tombstone for the feed-retention window instead.
		Where(`"appointment".cancelled_at IS NULL AND "appointment".deleted_at IS NULL`)
	_, rows, err := execute(ctx, withTenant(query, "appointment", tenantID), "delete expired appointments")
	return int(rows), err
}
