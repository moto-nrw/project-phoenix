package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Public holiday registrations (#4661, ADR 0110). The link lookup runs in the
// administrative transaction, like the school slug of the online
// registration, and reads only the Träger and the period of a published
// token. Every other statement runs in the Träger transaction of the Träger
// it names and names it once more in its filter.

// HolidayRegistrationStore persists the public registrations of a Träger.
type HolidayRegistrationStore struct{}

// NewHolidayRegistrationStore builds the store.
func NewHolidayRegistrationStore() *HolidayRegistrationStore { return &HolidayRegistrationStore{} }

const findHolidayRegistrationLinkQuery = `SELECT organization_id, id FROM enrollment.holiday_periods
	WHERE public_token = ? AND status = 'published'`

// FindHolidayRegistrationLink reads the Träger and the published period of a
// link. Only the administrative transaction may read across Träger.
func (s *HolidayRegistrationStore) FindHolidayRegistrationLink(ctx context.Context, token string) (int64, int64, bool, error) {
	if !tenant.IsAdminTx(ctx) {
		return 0, 0, false, errors.New("enrollment postgres: the link lookup requires the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return 0, 0, false, err
	}
	var rows []struct {
		OrganizationID int64 `bun:"organization_id"`
		ID             int64 `bun:"id"`
	}
	if err := db.NewRaw(findHolidayRegistrationLinkQuery, token).Scan(ctx, &rows); err != nil {
		return 0, 0, false, fmt.Errorf("enrollment postgres: read holiday registration link: %w", err)
	}
	if len(rows) == 0 {
		return 0, 0, false, nil
	}
	return rows[0].OrganizationID, rows[0].ID, true, nil
}

const (
	// deleteExpiredHolidayRateLimitsQuery keeps the counters of a Träger
	// small: no window is longer than a day.
	deleteExpiredHolidayRateLimitsQuery = `DELETE FROM enrollment.holiday_registration_rate_limits
		WHERE organization_id = ? AND window_start < statement_timestamp() - INTERVAL '24 hours'`
	countHolidayRateLimitQuery = `INSERT INTO enrollment.holiday_registration_rate_limits AS bucket
			(organization_id, key_type, key_hash)
		VALUES (?, ?, encode(sha256(convert_to(?, 'UTF8')), 'hex'))
		ON CONFLICT (organization_id, key_type, key_hash) DO UPDATE SET
			attempts = CASE WHEN bucket.window_start > statement_timestamp() - (? * INTERVAL '1 second')
				THEN bucket.attempts + 1 ELSE 1 END,
			window_start = CASE WHEN bucket.window_start > statement_timestamp() - (? * INTERVAL '1 second')
				THEN bucket.window_start ELSE statement_timestamp() END,
			updated_at = statement_timestamp()
		RETURNING attempts`
)

// CountHolidayRegistrationAttempt removes the Träger's expired counters and
// counts one attempt in a single atomic upsert keyed by the SHA-256 digest
// of value; the value itself is never stored.
func (s *HolidayRegistrationStore) CountHolidayRegistrationAttempt(ctx context.Context, organizationID int64, keyType, value string, window time.Duration) (int, error) {
	seconds := int64(window.Seconds())
	if seconds <= 0 {
		return 0, fmt.Errorf("enrollment postgres: invalid rate-limit window %s", window)
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	if _, err := db.NewRaw(deleteExpiredHolidayRateLimitsQuery, organizationID).Exec(ctx); err != nil {
		return 0, fmt.Errorf("enrollment postgres: remove expired holiday registration counters: %w", err)
	}
	var attempts int
	if err := db.NewRaw(countHolidayRateLimitQuery, organizationID, keyType, value, seconds, seconds).Scan(ctx, &attempts); err != nil {
		return 0, fmt.Errorf("enrollment postgres: count holiday registration attempt: %w", err)
	}
	return attempts, nil
}

const (
	insertHolidayRegistrationQuery = `INSERT INTO enrollment.holiday_registrations
			(organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone,
			 privacy_consent_at, health_consent_at, binding_confirmed_at, benefits_declared)
		VALUES (?, ?, ?, ?, ?, ?, statement_timestamp(),
			CASE WHEN ? THEN statement_timestamp() END, CASE WHEN ? THEN statement_timestamp() END, ?)
		RETURNING id, submitted_at`
	insertHolidayBookingsQuery = `INSERT INTO enrollment.holiday_registration_bookings
			(organization_id, holiday_period_id, holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id)
		SELECT ?, ?, booking.child_id, booking.week_id, booking.time_model_id
		FROM unnest(?::bigint[], ?::bigint[], ?::bigint[]) AS booking(child_id, week_id, time_model_id)`
	insertHolidayContactsQuery = `INSERT INTO enrollment.holiday_registration_contacts
			(organization_id, holiday_registration_id, kind, position, name, phone, relationship)
		SELECT ?, ?, contact.kind, contact.position, contact.name, contact.phone, contact.relationship
		FROM unnest(?::text[], ?::smallint[], ?::text[], ?::text[], ?::text[])
			AS contact(kind, position, name, phone, relationship)`
)

// holidayChildRow is one child of a registration; a nil HomeSchoolID makes a
// holiday guest.
type holidayChildRow struct {
	bun.BaseModel `bun:"table:enrollment.holiday_registration_children"`

	ID                    int64         `bun:"id,pk,autoincrement"`
	OrganizationID        int64         `bun:"organization_id"`
	HolidayPeriodID       int64         `bun:"holiday_period_id"`
	HolidayRegistrationID int64         `bun:"holiday_registration_id"`
	Position              int           `bun:"position"`
	FirstName             string        `bun:"first_name"`
	LastName              string        `bun:"last_name"`
	BirthDate             calendar.Date `bun:"birth_date,type:date"`
	HomeSchoolID          *int64        `bun:"home_school_id"`
	GuestSchoolName       string        `bun:"guest_school_name"`
	Allergies             string        `bun:"allergies"`
	Medication            string        `bun:"medication"`
	HealthNotes           string        `bun:"health_notes"`
}

// InsertHolidayRegistration stores the registration with its children,
// bookings and contacts: one statement for the registration, one per child
// and one each for all bookings and all contacts.
func (s *HolidayRegistrationStore) InsertHolidayRegistration(ctx context.Context, organizationID, periodID int64, input enrollment.HolidayRegistrationInput) (enrollment.HolidayRegistrationReceipt, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	var receipt struct {
		ID          int64     `bun:"id"`
		SubmittedAt time.Time `bun:"submitted_at"`
	}
	guardian := input.Guardian
	if err := db.NewRaw(insertHolidayRegistrationQuery, organizationID, periodID, guardian.FirstName, guardian.LastName,
		guardian.Email, guardian.Phone, input.HealthConsent, input.BindingConfirmed, input.BenefitsDeclared).Scan(ctx, &receipt); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, fmt.Errorf("enrollment postgres: insert holiday registration: %w", err)
	}
	var childIDs, weekIDs, modelIDs []int64
	for i, child := range input.Children {
		row := holidayChildRow{
			OrganizationID: organizationID, HolidayPeriodID: periodID, HolidayRegistrationID: receipt.ID, Position: i + 1,
			FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
			GuestSchoolName: child.GuestSchoolName, Allergies: child.Allergies, Medication: child.Medication, HealthNotes: child.HealthNotes,
		}
		if child.HomeSchoolID > 0 {
			row.HomeSchoolID = &child.HomeSchoolID
		}
		if _, err := db.NewInsert().Model(&row).ExcludeColumn("id").Returning("id").Exec(ctx); err != nil {
			return enrollment.HolidayRegistrationReceipt{}, fmt.Errorf("enrollment postgres: insert holiday registration child: %w", err)
		}
		for _, booking := range child.Bookings {
			childIDs = append(childIDs, row.ID)
			weekIDs = append(weekIDs, booking.WeekID)
			modelIDs = append(modelIDs, booking.TimeModelID)
		}
	}
	if _, err := db.NewRaw(insertHolidayBookingsQuery, organizationID, periodID, pgdialect.Array(childIDs),
		pgdialect.Array(weekIDs), pgdialect.Array(modelIDs)).Exec(ctx); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, fmt.Errorf("enrollment postgres: insert holiday registration bookings: %w", err)
	}
	if err := insertHolidayContacts(ctx, db, organizationID, receipt.ID, input); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	return enrollment.HolidayRegistrationReceipt{ID: receipt.ID, SubmittedAt: receipt.SubmittedAt}, nil
}

func insertHolidayContacts(ctx context.Context, db bun.IDB, organizationID, registrationID int64, input enrollment.HolidayRegistrationInput) error {
	var kinds, names, phones, relationships []string
	var positions []int
	add := func(kind string, contacts []enrollment.HolidayContactInput) {
		for i, contact := range contacts {
			kinds = append(kinds, kind)
			positions = append(positions, i+1)
			names = append(names, contact.Name)
			phones = append(phones, contact.Phone)
			relationships = append(relationships, contact.Relationship)
		}
	}
	add(enrollment.HolidayContactEmergency, input.EmergencyContacts)
	add(enrollment.HolidayContactPickup, input.PickupPersons)
	if len(kinds) == 0 {
		return nil
	}
	if _, err := db.NewRaw(insertHolidayContactsQuery, organizationID, registrationID, pgdialect.Array(kinds),
		pgdialect.Array(positions), pgdialect.Array(names), pgdialect.Array(phones), pgdialect.Array(relationships),
	).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: insert holiday registration contacts: %w", err)
	}
	return nil
}

const (
	deleteEndedHolidayRegistrationsQuery = `DELETE FROM enrollment.holiday_registrations
		WHERE holiday_period_id IN (
			SELECT week.holiday_period_id FROM enrollment.holiday_period_weeks AS week
			GROUP BY week.holiday_period_id
			HAVING max(week.ends_on) < ?)`
	// deleteEndedHolidayAuditQuery deletes the audit trail of the ended
	// periods: the entries of a list or an export, which name no
	// registration, and with them those of the deleted registrations (#4663).
	deleteEndedHolidayAuditQuery = `DELETE FROM enrollment.holiday_registration_audit
		WHERE holiday_period_id IN (
			SELECT week.holiday_period_id FROM enrollment.holiday_period_weeks AS week
			GROUP BY week.holiday_period_id
			HAVING max(week.ends_on) < ?)`
	deleteAllExpiredHolidayRateLimitsQuery = `DELETE FROM enrollment.holiday_registration_rate_limits
		WHERE window_start < statement_timestamp() - INTERVAL '24 hours'`
)

// DeleteHolidayRegistrationsEndedBefore deletes, across every Träger, the
// registrations and the audit trail of the periods whose last week ended
// before cutoff and the expired abuse counters. Only the administrative transaction of the hourly
// cleanup may delete across Träger.
func (s *HolidayRegistrationStore) DeleteHolidayRegistrationsEndedBefore(ctx context.Context, cutoff calendar.Date) (int, error) {
	if !tenant.IsAdminTx(ctx) {
		return 0, errors.New("enrollment postgres: the holiday registration retention requires the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return 0, err
	}
	registrations, err := affectedRows(db.NewRaw(deleteEndedHolidayRegistrationsQuery, cutoff).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("enrollment postgres: delete ended holiday registrations: %w", err)
	}
	audit, err := affectedRows(db.NewRaw(deleteEndedHolidayAuditQuery, cutoff).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("enrollment postgres: delete ended holiday registration audit: %w", err)
	}
	counters, err := affectedRows(db.NewRaw(deleteAllExpiredHolidayRateLimitsQuery).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("enrollment postgres: delete expired holiday registration counters: %w", err)
	}
	return registrations + audit + counters, nil
}

func affectedRows(result sql.Result, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	return int(rows), err
}
