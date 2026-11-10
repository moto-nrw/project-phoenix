package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Closing days in booked holiday weeks (#6434). Every statement runs in the
// Träger transaction of the Träger it names and names it once more in its
// filter, so a missing policy never widens it to a foreign Träger.

const (
	// holidayClosureBookingsQuery reads the bookings of received and
	// confirmed registrations in the weeks that overlap [from, to] at the
	// host schools named (all with an empty list), placed and waitlisted.
	holidayClosureBookingsQuery = `SELECT registration.id AS registration_id, registration.guardian_email,
			registration.guardian_first_name, registration.guardian_last_name,
			child.id AS child_id, child.first_name, child.last_name, child.home_school_id IS NULL AS guest,
			period.id AS period_id, period.name AS period_name,
			week.id AS week_id, week.starts_on, week.ends_on, week.host_school_id,
			model.name AS time_model_name, to_char(model.starts_at, 'HH24:MI') AS starts_at,
			to_char(model.ends_at, 'HH24:MI') AS ends_at, booking.placement, booking.days::text AS days
		FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		JOIN enrollment.holiday_period_weeks AS week
			ON week.organization_id = booking.organization_id AND week.id = booking.holiday_period_week_id
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		JOIN enrollment.holiday_week_time_models AS model
			ON model.organization_id = booking.organization_id AND model.id = booking.holiday_week_time_model_id
		WHERE booking.organization_id = ? AND registration.status IN ` + holidayCountingStatuses + `
			AND week.ends_on >= ? AND week.starts_on <= ?
			AND (cardinality(?::bigint[]) = 0 OR week.host_school_id = ANY(?::bigint[]))
		ORDER BY week.starts_on, week.id, registration.id, child.position`
	holidayClosureNoticesQuery = `SELECT holiday_registration_id, holiday_period_week_id, closed_on, state
		FROM enrollment.holiday_closure_notices
		WHERE organization_id = ? AND holiday_registration_id = ANY(?::bigint[])`
	setHolidayClosureNoticesQuery = `INSERT INTO enrollment.holiday_closure_notices
			(organization_id, holiday_registration_id, holiday_period_week_id, closed_on, state, operation_id)
		SELECT ?, unnest(?::bigint[]), unnest(?::bigint[]), unnest(?::date[]), unnest(?::text[]), ?::uuid
		ON CONFLICT (organization_id, holiday_registration_id, holiday_period_week_id, closed_on) DO UPDATE
			SET state = EXCLUDED.state, operation_id = EXCLUDED.operation_id, changed_at = CURRENT_TIMESTAMP`
	insertHolidayClosureFamilyKeyQuery = `INSERT INTO enrollment.holiday_closure_family_keys (organization_id, secret)
		VALUES (?, ?) ON CONFLICT (organization_id) DO NOTHING`
	holidayClosureFamilyKeyQuery   = `SELECT secret FROM enrollment.holiday_closure_family_keys WHERE organization_id = ?`
	recordHolidayClosureSendsQuery = `INSERT INTO enrollment.holiday_closure_sends
			(organization_id, operation_id, family_key, holiday_registration_id, idempotency_key)
		SELECT ?, unnest(?::uuid[]), unnest(?::text[]), unnest(?::bigint[]), unnest(?::text[])
		ON CONFLICT DO NOTHING`
	holidayClosureSendKeysQuery = `SELECT DISTINCT idempotency_key FROM enrollment.holiday_closure_sends
		WHERE organization_id = ? AND operation_id = ?::uuid ORDER BY idempotency_key`
)

// LockHolidayClosureNotices serializes the notice read/enqueue/write sequence
// per Träger in the caller's transaction. It takes no foreign owner's locks.
func (s *HolidayRegistrationStore) LockHolidayClosureNotices(ctx context.Context, organizationID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('enrollment:holiday-closure:' || ?::text, 0))`, organizationID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("enrollment postgres: lock holiday closure notices: %w", err)
	}
	return nil
}

type holidayClosureBookingRow struct {
	RegistrationID    int64         `bun:"registration_id"`
	GuardianEmail     string        `bun:"guardian_email"`
	GuardianFirstName string        `bun:"guardian_first_name"`
	GuardianLastName  string        `bun:"guardian_last_name"`
	ChildID           int64         `bun:"child_id"`
	FirstName         string        `bun:"first_name"`
	LastName          string        `bun:"last_name"`
	Guest             bool          `bun:"guest"`
	PeriodID          int64         `bun:"period_id"`
	PeriodName        string        `bun:"period_name"`
	WeekID            int64         `bun:"week_id"`
	StartsOn          calendar.Date `bun:"starts_on"`
	EndsOn            calendar.Date `bun:"ends_on"`
	HostSchoolID      int64         `bun:"host_school_id"`
	TimeModelName     string        `bun:"time_model_name"`
	StartsAt          string        `bun:"starts_at"`
	EndsAt            string        `bun:"ends_at"`
	Placement         string        `bun:"placement"`
	Days              *string       `bun:"days"`
}

// HolidayClosureBookings reads the bookings that count of the weeks at
// schoolIDs (every host school when empty) that overlap [from, to].
func (s *HolidayRegistrationStore) HolidayClosureBookings(ctx context.Context, organizationID int64, schoolIDs []int64, from, to calendar.Date) ([]enrollment.HolidayClosureBookingRecord, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if schoolIDs == nil {
		schoolIDs = []int64{}
	}
	var rows []holidayClosureBookingRow
	if err := db.NewRaw(holidayClosureBookingsQuery, organizationID, from, to, pgdialect.Array(schoolIDs), pgdialect.Array(schoolIDs)).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday bookings of a closure: %w", err)
	}
	records := make([]enrollment.HolidayClosureBookingRecord, len(rows))
	for i, row := range rows {
		days, err := holidayDaysOf(row.Days)
		if err != nil {
			return nil, fmt.Errorf("enrollment postgres: read the booked days of a closure: %w", err)
		}
		records[i] = enrollment.HolidayClosureBookingRecord{
			RegistrationID: row.RegistrationID, GuardianEmail: row.GuardianEmail, GuardianFirstName: row.GuardianFirstName,
			GuardianLastName: row.GuardianLastName, ChildID: row.ChildID, FirstName: row.FirstName, LastName: row.LastName,
			Guest: row.Guest, PeriodID: row.PeriodID, PeriodName: row.PeriodName, WeekID: row.WeekID, StartsOn: row.StartsOn,
			EndsOn: row.EndsOn, HostSchoolID: row.HostSchoolID, TimeModelName: row.TimeModelName, StartsAt: row.StartsAt,
			EndsAt: row.EndsAt, Placement: row.Placement, Days: days,
		}
	}
	return records, nil
}

type holidayClosureNoticeRow struct {
	bun.BaseModel  `bun:"table:enrollment.holiday_closure_notices"`
	RegistrationID int64         `bun:"holiday_registration_id"`
	WeekID         int64         `bun:"holiday_period_week_id"`
	ClosedOn       calendar.Date `bun:"closed_on"`
	State          string        `bun:"state"`
}

// HolidayClosureNotices reads the closed days the parents of the
// registrations were told about.
func (s *HolidayRegistrationStore) HolidayClosureNotices(ctx context.Context, organizationID int64, registrationIDs []int64) ([]enrollment.HolidayClosureNoticeRecord, error) {
	if len(registrationIDs) == 0 {
		return nil, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayClosureNoticeRow
	if err := db.NewRaw(holidayClosureNoticesQuery, organizationID, pgdialect.Array(registrationIDs)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday closure notices: %w", err)
	}
	notices := make([]enrollment.HolidayClosureNoticeRecord, len(rows))
	for i, row := range rows {
		notices[i] = enrollment.HolidayClosureNoticeRecord{RegistrationID: row.RegistrationID, WeekID: row.WeekID, ClosedOn: row.ClosedOn, State: row.State}
	}
	return notices, nil
}

// SetHolidayClosureNotices records what the parents were told now: each
// day closed or open again, under the operation that told it.
func (s *HolidayRegistrationStore) SetHolidayClosureNotices(ctx context.Context, organizationID int64, operationID string, notices []enrollment.HolidayClosureNoticeRecord) error {
	if len(notices) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	registrations := make([]int64, len(notices))
	weeks := make([]int64, len(notices))
	days := make([]string, len(notices))
	states := make([]string, len(notices))
	for i, notice := range notices {
		registrations[i], weeks[i], days[i], states[i] = notice.RegistrationID, notice.WeekID, notice.ClosedOn.String(), notice.State
	}
	if _, err := db.NewRaw(setHolidayClosureNoticesQuery, organizationID, pgdialect.Array(registrations), pgdialect.Array(weeks),
		pgdialect.Array(days), pgdialect.Array(states), operationID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: record holiday closure notices: %w", err)
	}
	return nil
}

// HolidayClosureFamilyKey returns the Träger's key of the family
// fingerprints; candidate becomes it on first use.
func (s *HolidayRegistrationStore) HolidayClosureFamilyKey(ctx context.Context, organizationID int64, candidate []byte) ([]byte, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if _, err := db.NewRaw(insertHolidayClosureFamilyKeyQuery, organizationID, candidate).Exec(ctx); err != nil {
		return nil, fmt.Errorf("enrollment postgres: store the family key: %w", err)
	}
	var stored []byte
	if err := db.NewRaw(holidayClosureFamilyKeyQuery, organizationID).Scan(ctx, &stored); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read the family key: %w", err)
	}
	return stored, nil
}

// RecordHolidayClosureSends records the mails of an operation, one row per
// registration a family's mail names.
func (s *HolidayRegistrationStore) RecordHolidayClosureSends(ctx context.Context, organizationID int64, sends []enrollment.HolidayClosureSendRecord) error {
	if len(sends) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	operations := make([]string, len(sends))
	families := make([]string, len(sends))
	registrations := make([]int64, len(sends))
	keys := make([]string, len(sends))
	for i, send := range sends {
		operations[i], families[i], registrations[i], keys[i] = send.OperationID, send.FamilyKey, send.RegistrationID, send.IdempotencyKey
	}
	if _, err := db.NewRaw(recordHolidayClosureSendsQuery, organizationID, pgdialect.Array(operations), pgdialect.Array(families),
		pgdialect.Array(registrations), pgdialect.Array(keys)).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: record holiday closure sends: %w", err)
	}
	return nil
}

// HolidayClosureSendKeys lists the idempotency keys of the operation's mails.
func (s *HolidayRegistrationStore) HolidayClosureSendKeys(ctx context.Context, organizationID int64, operationID string) ([]string, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var keys []string
	if err := db.NewRaw(holidayClosureSendKeysQuery, organizationID, operationID).Scan(ctx, &keys); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday closure sends: %w", err)
	}
	return keys, nil
}
