package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// InsertStudentCareProfiles inserts the care profiles of new memberships in
// one statement, each row with the values SaveStudentCareProfile writes when
// the membership has no care profile yet (its INSERT branch): the health
// field sealed for its own membership, the departure plan encoded the same
// way, the pickup status of a touched plan. A membership that has a care
// profile already violates the unique key and fails the statement; the
// single command would update it instead, which a new membership never needs.
func (s *DirectoryStore) InsertStudentCareProfiles(ctx context.Context, saves []domain.StudentCareProfileSave) (domain.OperationStats, error) {
	if len(saves) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	values := make([]string, 0, len(saves))
	args := make([]any, 0, 13*len(saves))
	for _, save := range saves {
		profile := save.Profile
		departure, err := encodeStudentDeparture(save.Plan)
		if err != nil {
			return domain.OperationStats{}, err
		}
		health, err := s.directory.SealStudentHealth(tenantID, profile.MembershipID, profile.HealthInfo)
		if err != nil {
			return domain.OperationStats{}, err
		}
		if save.Plan != nil && save.Plan.PlanTouched {
			profile.PickupStatus = &save.Plan.PickupStatus
		}
		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, ?::jsonb, ?::jsonb, ?)")
		args = append(args, tenantID, profile.MembershipID, health, profile.PickupStatus,
			profile.Sick, profile.SickSince, profile.Excused, profile.ExcusedSince,
			departure.modes, departure.days, departure.bus, departure.pickup, departure.note)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	// The VALUES list holds placeholders only; every value is an argument.
	result, err := db.NewRaw(`INSERT INTO users.student_care_profiles
  (tenant_id, membership_id, health_data, pickup_status, sick, sick_since, excused, excused_since,
   allowed_departure_modes, departure_days, bus_days, pickup_days, departure_companion_note)
  VALUES `+strings.Join(values, ", "), args...).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, err
	}
	stats.Rows, err = result.RowsAffected()
	if err == nil && stats.Rows != int64(len(saves)) {
		err = fmt.Errorf("care plan: %d of %d student care profiles written", stats.Rows, len(saves))
	}
	return stats, err
}
