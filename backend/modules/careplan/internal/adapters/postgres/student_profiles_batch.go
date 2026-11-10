package postgres

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

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
	columns := newCareProfileColumns(len(saves))
	for i, save := range saves {
		if err := s.fillCareProfileColumns(&columns, i, tenantID, save); err != nil {
			return domain.OperationStats{}, err
		}
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewRaw(insertStudentCareProfilesSQL, append([]any{tenantID}, columns.arguments()...)...).Exec(ctx)
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

const insertStudentCareProfilesSQL = `INSERT INTO users.student_care_profiles
  (tenant_id, membership_id, health_data, pickup_status, sick, sick_since, excused, excused_since,
   allowed_departure_modes, departure_days, bus_days, pickup_days, departure_companion_note)
  SELECT ?, k.membership_id, CASE WHEN k.health_set THEN decode(k.health, 'hex') END,
   CASE WHEN k.pickup_set THEN k.pickup END, k.sick, CASE WHEN k.sick_since_set THEN k.sick_since END,
   k.excused, CASE WHEN k.excused_since_set THEN k.excused_since END,
   k.modes::jsonb, k.days::jsonb, k.bus::jsonb, k.pickup_days::jsonb, CASE WHEN k.note_set THEN k.note END
  FROM unnest(?::bigint[], ?::text[], ?::boolean[], ?::text[], ?::boolean[], ?::boolean[], ?::timestamptz[], ?::boolean[],
   ?::boolean[], ?::timestamptz[], ?::boolean[], ?::text[], ?::text[], ?::text[], ?::text[], ?::text[], ?::boolean[])
  WITH ORDINALITY AS k(membership_id, health, health_set, pickup, pickup_set, sick, sick_since, sick_since_set,
   excused, excused_since, excused_since_set, modes, days, bus, pickup_days, note, note_set, ord)
  ORDER BY k.ord`

// careProfileColumns holds the unnest arrays of InsertStudentCareProfiles;
// each *Set flag tells a NULL apart from an empty or zero value.
type careProfileColumns struct {
	memberships                   []int64
	health, pickup                []string
	healthSet, pickupSet          []bool
	sick, excused                 []bool
	sickSince, excusedSince       []time.Time
	sickSinceSet, excusedSinceSet []bool
	modes, days, bus, pickupDays  []string
	note                          []string
	noteSet                       []bool
}

func newCareProfileColumns(n int) careProfileColumns {
	return careProfileColumns{
		memberships: make([]int64, n),
		health:      make([]string, n), healthSet: make([]bool, n),
		pickup: make([]string, n), pickupSet: make([]bool, n),
		sick: make([]bool, n), excused: make([]bool, n),
		sickSince: make([]time.Time, n), sickSinceSet: make([]bool, n),
		excusedSince: make([]time.Time, n), excusedSinceSet: make([]bool, n),
		modes: make([]string, n), days: make([]string, n), bus: make([]string, n), pickupDays: make([]string, n),
		note: make([]string, n), noteSet: make([]bool, n),
	}
}

func (s *DirectoryStore) fillCareProfileColumns(c *careProfileColumns, i int, tenantID int64, save domain.StudentCareProfileSave) error {
	profile := save.Profile
	departure, err := encodeStudentDeparture(save.Plan)
	if err != nil {
		return err
	}
	sealed, err := s.directory.SealStudentHealth(tenantID, profile.MembershipID, profile.HealthInfo)
	if err != nil {
		return err
	}
	if save.Plan != nil && save.Plan.PlanTouched {
		profile.PickupStatus = &save.Plan.PickupStatus
	}
	c.memberships[i], c.sick[i], c.excused[i] = profile.MembershipID, profile.Sick, profile.Excused
	if sealed != nil {
		c.health[i], c.healthSet[i] = hex.EncodeToString(sealed), true
	}
	c.pickup[i], c.pickupSet[i] = optional(profile.PickupStatus)
	c.sickSince[i], c.sickSinceSet[i] = optional(profile.SickSince)
	c.excusedSince[i], c.excusedSinceSet[i] = optional(profile.ExcusedSince)
	c.modes[i], c.days[i], c.bus[i], c.pickupDays[i] = departure.modes, departure.days, departure.bus, departure.pickup
	c.note[i], c.noteSet[i] = optional(departure.note)
	return nil
}

func optional[T any](value *T) (T, bool) {
	if value == nil {
		var zero T
		return zero, false
	}
	return *value, true
}

func (c *careProfileColumns) arguments() []any {
	return []any{
		pgdialect.Array(c.memberships), pgdialect.Array(c.health), pgdialect.Array(c.healthSet),
		pgdialect.Array(c.pickup), pgdialect.Array(c.pickupSet), pgdialect.Array(c.sick), pgdialect.Array(c.sickSince), pgdialect.Array(c.sickSinceSet),
		pgdialect.Array(c.excused), pgdialect.Array(c.excusedSince), pgdialect.Array(c.excusedSinceSet),
		pgdialect.Array(c.modes), pgdialect.Array(c.days), pgdialect.Array(c.bus), pgdialect.Array(c.pickupDays),
		pgdialect.Array(c.note), pgdialect.Array(c.noteSet),
	}
}
