package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Review of the holiday registrations by the Träger-Büro (#4663, ADR 0114).
// Every statement runs in the Träger transaction of the Träger it names and
// names it once more in its filter. A zero registration ID reads the whole
// period.

// holidayReviewRegistrationRow is one registration as the review reads it.
type holidayReviewRegistrationRow struct {
	ID                 int64      `bun:"id"`
	Status             string     `bun:"status"`
	FreezeOverride     *string    `bun:"freeze_override"`
	SubmittedAt        time.Time  `bun:"submitted_at"`
	GuardianFirstName  string     `bun:"guardian_first_name"`
	GuardianLastName   string     `bun:"guardian_last_name"`
	GuardianEmail      string     `bun:"guardian_email"`
	GuardianPhone      string     `bun:"guardian_phone"`
	PrivacyConsentAt   time.Time  `bun:"privacy_consent_at"`
	HealthConsentAt    *time.Time `bun:"health_consent_at"`
	BindingConfirmedAt *time.Time `bun:"binding_confirmed_at"`
}

// holidayReviewChildRow is one child as the review reads it. The health
// fields stay empty unless the read asks for them.
type holidayReviewChildRow struct {
	ID                       int64         `bun:"id"`
	RegistrationID           int64         `bun:"holiday_registration_id"`
	FirstName                string        `bun:"first_name"`
	LastName                 string        `bun:"last_name"`
	BirthDate                calendar.Date `bun:"birth_date"`
	HomeSchoolID             *int64        `bun:"home_school_id"`
	GuestSchoolName          string        `bun:"guest_school_name"`
	Allergies                string        `bun:"allergies"`
	Medication               string        `bun:"medication"`
	HealthNotes              string        `bun:"health_notes"`
	DuplicateRegistrationIDs []int64       `bun:"duplicate_registration_ids,array"`
}

// holidayReviewBookingRow is one booking of a child.
type holidayReviewBookingRow struct {
	ChildID     int64 `bun:"holiday_registration_child_id"`
	WeekID      int64 `bun:"holiday_period_week_id"`
	TimeModelID int64 `bun:"holiday_week_time_model_id"`
}

const (
	reviewRegistrationsQuery = `SELECT id, status, freeze_override, submitted_at,
			guardian_first_name, guardian_last_name, guardian_email, guardian_phone,
			privacy_consent_at, health_consent_at, binding_confirmed_at
		FROM enrollment.holiday_registrations
		WHERE organization_id = ? AND holiday_period_id = ? AND (?::bigint = 0 OR id = ?)
		ORDER BY submitted_at, id`
	// reviewChildrenColumns name the other registrations of the period, not
	// cancelled, with a child of the same name and date of birth.
	reviewChildrenColumns = `child.id, child.holiday_registration_id, child.first_name, child.last_name,
			child.birth_date, child.home_school_id, child.guest_school_name,
			ARRAY(SELECT DISTINCT other.holiday_registration_id
				FROM enrollment.holiday_registration_children AS other
				JOIN enrollment.holiday_registrations AS registration
					ON registration.organization_id = other.organization_id AND registration.id = other.holiday_registration_id
				WHERE other.organization_id = child.organization_id
					AND other.holiday_period_id = child.holiday_period_id
					AND other.holiday_registration_id <> child.holiday_registration_id
					AND registration.status <> 'cancelled'
					AND lower(other.first_name) = lower(child.first_name)
					AND lower(other.last_name) = lower(child.last_name)
					AND other.birth_date = child.birth_date
				ORDER BY 1) AS duplicate_registration_ids`
	reviewChildrenFilter = ` FROM enrollment.holiday_registration_children AS child
		WHERE child.organization_id = ? AND child.holiday_period_id = ? AND (?::bigint = 0 OR child.holiday_registration_id = ?)
		ORDER BY child.holiday_registration_id, child.position`
	reviewChildrenQuery           = `SELECT ` + reviewChildrenColumns + reviewChildrenFilter
	reviewChildrenWithHealthQuery = `SELECT ` + reviewChildrenColumns +
		`, child.allergies, child.medication, child.health_notes` + reviewChildrenFilter
	reviewBookingsQuery = `SELECT booking.holiday_registration_child_id, booking.holiday_period_week_id,
			booking.holiday_week_time_model_id
		FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		WHERE booking.organization_id = ? AND booking.holiday_period_id = ? AND (?::bigint = 0 OR child.holiday_registration_id = ?)`
)

// ReadHolidayReview reads the registrations of a period, or the one
// registration registrationID names, with their children and bookings: three
// statements for any number of registrations. health reads the health
// information of the children too. A booking names only its week and time
// model. The second result is the freeze override of each registration:
// "frozen", "reopened" or empty when the deadline decides.
func (s *HolidayRegistrationStore) ReadHolidayReview(ctx context.Context, organizationID, periodID, registrationID int64, health bool) ([]enrollment.HolidayRegistrationDetail, map[int64]string, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, nil, err
	}
	var registrations []holidayReviewRegistrationRow
	if err := db.NewRaw(reviewRegistrationsQuery, organizationID, periodID, registrationID, registrationID).
		Scan(ctx, &registrations); err != nil {
		return nil, nil, fmt.Errorf("enrollment postgres: read holiday registrations: %w", err)
	}
	if len(registrations) == 0 {
		return []enrollment.HolidayRegistrationDetail{}, map[int64]string{}, nil
	}
	var children []holidayReviewChildRow
	if health {
		err = db.NewRaw(reviewChildrenWithHealthQuery, organizationID, periodID, registrationID, registrationID).Scan(ctx, &children)
	} else {
		err = db.NewRaw(reviewChildrenQuery, organizationID, periodID, registrationID, registrationID).Scan(ctx, &children)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("enrollment postgres: read holiday registration children: %w", err)
	}
	var bookings []holidayReviewBookingRow
	if err := db.NewRaw(reviewBookingsQuery, organizationID, periodID, registrationID, registrationID).
		Scan(ctx, &bookings); err != nil {
		return nil, nil, fmt.Errorf("enrollment postgres: read holiday registration bookings: %w", err)
	}
	return holidayReviewDetails(registrations, children, bookings), holidayFreezeOverrides(registrations), nil
}

func holidayReviewDetails(registrations []holidayReviewRegistrationRow, children []holidayReviewChildRow, bookings []holidayReviewBookingRow) []enrollment.HolidayRegistrationDetail {
	childBookings := map[int64][]enrollment.HolidayRegistrationBooking{}
	for _, booking := range bookings {
		childBookings[booking.ChildID] = append(childBookings[booking.ChildID],
			enrollment.HolidayRegistrationBooking{WeekID: booking.WeekID, TimeModelID: booking.TimeModelID})
	}
	byRegistration := map[int64][]enrollment.HolidayRegistrationChild{}
	health := map[int64][]enrollment.HolidayRegistrationHealth{}
	for _, child := range children {
		duplicates := child.DuplicateRegistrationIDs
		if duplicates == nil {
			duplicates = []int64{}
		}
		byRegistration[child.RegistrationID] = append(byRegistration[child.RegistrationID], enrollment.HolidayRegistrationChild{
			ID: child.ID, FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
			HomeSchoolID: child.HomeSchoolID, GuestSchoolName: child.GuestSchoolName,
			Bookings: childBookings[child.ID], DuplicateRegistrationIDs: duplicates,
		})
		if child.Allergies != "" || child.Medication != "" || child.HealthNotes != "" {
			health[child.RegistrationID] = append(health[child.RegistrationID], enrollment.HolidayRegistrationHealth{
				ChildID: child.ID, Allergies: child.Allergies, Medication: child.Medication, HealthNotes: child.HealthNotes,
			})
		}
	}
	details := make([]enrollment.HolidayRegistrationDetail, len(registrations))
	for i, row := range registrations {
		details[i] = enrollment.HolidayRegistrationDetail{
			HolidayRegistrationSummary: enrollment.HolidayRegistrationSummary{
				ID: row.ID, Status: row.Status, SubmittedAt: row.SubmittedAt,
				GuardianFirstName: row.GuardianFirstName, GuardianLastName: row.GuardianLastName,
				GuardianEmail: row.GuardianEmail, GuardianPhone: row.GuardianPhone, Children: byRegistration[row.ID],
			},
			Health:           health[row.ID],
			PrivacyConsentAt: row.PrivacyConsentAt, HealthConsentAt: row.HealthConsentAt, BindingConfirmedAt: row.BindingConfirmedAt,
		}
	}
	return details
}

func holidayFreezeOverrides(registrations []holidayReviewRegistrationRow) map[int64]string {
	overrides := make(map[int64]string, len(registrations))
	for _, row := range registrations {
		if row.FreezeOverride != nil {
			overrides[row.ID] = *row.FreezeOverride
		}
	}
	return overrides
}

const (
	reviewContactsQuery = `SELECT kind, name, phone, relationship
		FROM enrollment.holiday_registration_contacts
		WHERE organization_id = ? AND holiday_registration_id = ?
		ORDER BY kind, position`
	reviewHistoryQuery = `SELECT action, occurred_at, actor_account_id, details
		FROM enrollment.holiday_registration_audit
		WHERE organization_id = ? AND holiday_registration_id = ?
			AND action IN ('confirm', 'cancel', 'rebook', 'freeze', 'reopen')
		ORDER BY occurred_at, id`
)

// ReadHolidayContacts reads the emergency contacts and the persons allowed to
// collect the children of a registration.
func (s *HolidayRegistrationStore) ReadHolidayContacts(ctx context.Context, organizationID, registrationID int64) ([]enrollment.HolidayRegistrationContact, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Kind         string `bun:"kind"`
		Name         string `bun:"name"`
		Phone        string `bun:"phone"`
		Relationship string `bun:"relationship"`
	}
	if err := db.NewRaw(reviewContactsQuery, organizationID, registrationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday registration contacts: %w", err)
	}
	contacts := make([]enrollment.HolidayRegistrationContact, len(rows))
	for i, row := range rows {
		contacts[i] = enrollment.HolidayRegistrationContact(row)
	}
	return contacts, nil
}

// ReadHolidayHistory reads the changes of a registration, oldest first.
func (s *HolidayRegistrationStore) ReadHolidayHistory(ctx context.Context, organizationID, registrationID int64) ([]enrollment.HolidayRegistrationEvent, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Action         string          `bun:"action"`
		OccurredAt     time.Time       `bun:"occurred_at"`
		ActorAccountID *int64          `bun:"actor_account_id"`
		Details        json.RawMessage `bun:"details"`
	}
	if err := db.NewRaw(reviewHistoryQuery, organizationID, registrationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday registration history: %w", err)
	}
	events := make([]enrollment.HolidayRegistrationEvent, len(rows))
	for i, row := range rows {
		var details enrollment.HolidayRegistrationEventDetails
		if err := json.Unmarshal(row.Details, &details); err != nil {
			return nil, fmt.Errorf("enrollment postgres: decode holiday registration history: %w", err)
		}
		events[i] = enrollment.HolidayRegistrationEvent{
			Action: row.Action, OccurredAt: row.OccurredAt, ActorAccountID: row.ActorAccountID, Details: details,
		}
	}
	return events, nil
}

const (
	lockHolidayRegistrationQuery = `SELECT id, status, freeze_override
		FROM enrollment.holiday_registrations
		WHERE organization_id = ? AND holiday_period_id = ? AND id = ?
		FOR UPDATE`
	updateHolidayRegistrationStatusQuery = `UPDATE enrollment.holiday_registrations SET status = ?
		WHERE organization_id = ? AND id = ?`
	updateHolidayRegistrationFreezeQuery = `UPDATE enrollment.holiday_registrations SET freeze_override = ?
		WHERE organization_id = ? AND id = ?`
)

// LockHolidayRegistration reads the status and the freeze override of one
// registration of the period and holds its row until the end of the
// transaction.
func (s *HolidayRegistrationStore) LockHolidayRegistration(ctx context.Context, organizationID, periodID, registrationID int64) (string, string, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return "", "", false, err
	}
	var rows []holidayReviewRegistrationRow
	if err := db.NewRaw(lockHolidayRegistrationQuery, organizationID, periodID, registrationID).Scan(ctx, &rows); err != nil {
		return "", "", false, fmt.Errorf("enrollment postgres: lock holiday registration: %w", err)
	}
	if len(rows) == 0 {
		return "", "", false, nil
	}
	return rows[0].Status, holidayFreezeOverrides(rows)[rows[0].ID], true, nil
}

// SetHolidayRegistrationStatus writes the status of a registration.
func (s *HolidayRegistrationStore) SetHolidayRegistrationStatus(ctx context.Context, organizationID, registrationID int64, status string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(updateHolidayRegistrationStatusQuery, status, organizationID, registrationID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: update holiday registration status: %w", err)
	}
	return nil
}

// SetHolidayRegistrationFreeze writes the freeze override of a
// registration: "frozen" or "reopened".
func (s *HolidayRegistrationStore) SetHolidayRegistrationFreeze(ctx context.Context, organizationID, registrationID int64, override string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(updateHolidayRegistrationFreezeQuery, override, organizationID, registrationID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: update holiday registration freeze: %w", err)
	}
	return nil
}

const (
	deleteHolidayChildBookingsQuery = `DELETE FROM enrollment.holiday_registration_bookings
		WHERE organization_id = ? AND holiday_registration_child_id = ?`
	insertHolidayChildBookingsQuery = `INSERT INTO enrollment.holiday_registration_bookings
			(organization_id, holiday_period_id, holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id)
		SELECT ?, ?, ?, booking.week_id, booking.time_model_id
		FROM unnest(?::bigint[], ?::bigint[]) AS booking(week_id, time_model_id)`
)

// ReplaceHolidayChildBookings makes bookings the bookings of the child. The
// caller checked that the child belongs to the locked registration and that
// every week and time model belongs to the period.
func (s *HolidayRegistrationStore) ReplaceHolidayChildBookings(ctx context.Context, organizationID, periodID, childID int64, bookings []enrollment.HolidayBookingInput) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(deleteHolidayChildBookingsQuery, organizationID, childID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: remove holiday child bookings: %w", err)
	}
	weekIDs := make([]int64, len(bookings))
	modelIDs := make([]int64, len(bookings))
	for i, booking := range bookings {
		weekIDs[i], modelIDs[i] = booking.WeekID, booking.TimeModelID
	}
	if _, err := db.NewRaw(insertHolidayChildBookingsQuery, organizationID, periodID, childID,
		pgdialect.Array(weekIDs), pgdialect.Array(modelIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: insert holiday child bookings: %w", err)
	}
	return nil
}

const insertHolidayAuditQuery = `INSERT INTO enrollment.holiday_registration_audit
		(organization_id, holiday_period_id, holiday_registration_id, action, details, actor_account_id)
	VALUES (?, ?, NULLIF(?::bigint, 0), ?, ?::jsonb, NULLIF(?::bigint, 0))`

// AppendHolidayAudit writes one entry of the audit trail with details as
// JSON. A zero registrationID records a read of the whole period (a list or
// an export).
func (s *HolidayRegistrationStore) AppendHolidayAudit(ctx context.Context, organizationID, periodID, registrationID int64, action string, details any, actorAccountID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if details == nil {
		details = struct{}{}
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("enrollment postgres: encode holiday registration audit: %w", err)
	}
	if _, err := db.NewRaw(insertHolidayAuditQuery, organizationID, periodID, registrationID, action, string(payload),
		actorAccountID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: append holiday registration audit: %w", err)
	}
	return nil
}
