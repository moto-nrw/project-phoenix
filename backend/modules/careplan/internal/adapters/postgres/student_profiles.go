package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"
	"github.com/uptrace/bun"
)

func (s *Store) ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	members := studentdirectoryview.Query(db, tenantID).
		ColumnExpr("student.membership_id").Where("student.id IN (?)", bun.List(ids))
	query := db.NewUpdate().TableExpr("users.student_care_profiles AS care").
		Set("updated_at = NOW()").Where("care.tenant_id = ?", tenantID).
		Where("care.membership_id IN (?)", members)
	switch status {
	case "sick":
		query = query.Set("sick = FALSE").Set("sick_since = NULL").Where("care.sick = TRUE")
	case "excused":
		query = query.Set("excused = FALSE").Set("excused_since = NULL").Where("care.excused = TRUE")
	default:
		return 0, domain.OperationStats{}, errors.New("care plan: unsupported absence flag")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err == nil {
		stats.Rows, err = result.RowsAffected()
	}
	return stats.Rows, stats, err
}

func (s *Store) SaveStudentCareProfile(ctx context.Context, profile domain.StudentCareProfile, plan *domain.StudentDeparturePlan) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	departure, err := encodeStudentDeparture(plan)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if plan != nil && plan.PlanTouched {
		profile.PickupStatus = &plan.PickupStatus
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewRaw(`INSERT INTO users.student_care_profiles AS care
  (tenant_id, membership_id, supervisor_notes, health_info, pickup_status, sick, sick_since, excused, excused_since,
   allowed_departure_modes, departure_days, bus_days, pickup_days, departure_companion_note)
  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, ?::jsonb, ?::jsonb, ?)
  ON CONFLICT (membership_id) DO UPDATE SET
   supervisor_notes = EXCLUDED.supervisor_notes, health_info = EXCLUDED.health_info,
   pickup_status = EXCLUDED.pickup_status, sick = EXCLUDED.sick, sick_since = EXCLUDED.sick_since,
   excused = EXCLUDED.excused, excused_since = EXCLUDED.excused_since, updated_at = NOW(),
   allowed_departure_modes = CASE WHEN ? THEN EXCLUDED.allowed_departure_modes ELSE care.allowed_departure_modes END,
   departure_days = CASE WHEN ? THEN EXCLUDED.departure_days ELSE care.departure_days END,
   bus_days = CASE WHEN ? THEN EXCLUDED.bus_days ELSE care.bus_days END,
   pickup_days = CASE WHEN ? THEN EXCLUDED.pickup_days ELSE care.pickup_days END,
   departure_companion_note = CASE WHEN ? THEN EXCLUDED.departure_companion_note ELSE care.departure_companion_note END
  WHERE care.tenant_id = EXCLUDED.tenant_id`,
		tenantID, profile.MembershipID, profile.SupervisorNotes, profile.HealthInfo, profile.PickupStatus,
		profile.Sick, profile.SickSince, profile.Excused, profile.ExcusedSince,
		departure.modes, departure.days, departure.bus, departure.pickup, departure.note,
		departure.touched, departure.touched, departure.touched, departure.touched, plan != nil).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err == nil {
		stats.Rows, err = result.RowsAffected()
		if err == nil && stats.Rows != 1 {
			err = errors.New("care plan: student care profile was not written")
		}
	}
	return stats, err
}

func (s *Store) SetStudentLiveStatus(ctx context.Context, input domain.StudentLiveStatus) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	members := studentdirectoryview.Query(db, tenantID).
		ColumnExpr("student.membership_id").Where("student.id = ?", input.StudentID)
	query := db.NewUpdate().TableExpr("users.student_care_profiles AS care").
		Set("sick = ?", input.Sick).Set("sick_since = ?", input.SickSince).
		Set("excused = ?", input.Excused).Set("excused_since = ?", input.ExcusedSince).
		Set("updated_at = NOW()").Where("care.tenant_id = ?", tenantID).
		Where("care.membership_id IN (?)", members)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err == nil {
		stats.Rows, err = result.RowsAffected()
	}
	return stats.Rows, stats, err
}

type encodedStudentDeparture struct {
	modes, days, bus, pickup string
	note                     *string
	touched                  bool
}

func encodeStudentDeparture(plan *domain.StudentDeparturePlan) (encodedStudentDeparture, error) {
	result := encodedStudentDeparture{modes: "{}", days: "{}", bus: "{}", pickup: "{}"}
	if plan == nil {
		return result, nil
	}
	result.note, result.touched = plan.CompanionNote, plan.PlanTouched
	for _, field := range []struct {
		value  any
		target *string
	}{
		{plan.AllowedDepartureModes, &result.modes}, {plan.DepartureDays, &result.days},
		{plan.BusDays, &result.bus}, {plan.PickupDays, &result.pickup},
	} {
		encoded, err := json.Marshal(field.value)
		if err != nil {
			return result, err
		}
		if string(encoded) != "null" {
			*field.target = string(encoded)
		}
	}
	return result, nil
}
