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
	ID                      int64      `bun:"id"`
	Status                  string     `bun:"status"`
	FreezeOverride          *string    `bun:"freeze_override"`
	SubmittedAt             time.Time  `bun:"submitted_at"`
	GuardianFirstName       string     `bun:"guardian_first_name"`
	GuardianLastName        string     `bun:"guardian_last_name"`
	GuardianEmail           string     `bun:"guardian_email"`
	GuardianPhone           string     `bun:"guardian_phone"`
	SecondGuardianFirstName string     `bun:"second_guardian_first_name"`
	SecondGuardianLastName  string     `bun:"second_guardian_last_name"`
	SecondGuardianEmail     string     `bun:"second_guardian_email"`
	SecondGuardianPhone     string     `bun:"second_guardian_phone"`
	AddressStreet           string     `bun:"guardian_address_street"`
	AddressPostalCode       string     `bun:"guardian_address_postal_code"`
	AddressCity             string     `bun:"guardian_address_city"`
	PrivacyConsentAt        time.Time  `bun:"privacy_consent_at"`
	HealthConsentAt         *time.Time `bun:"health_consent_at"`
	BindingConfirmedAt      *time.Time `bun:"binding_confirmed_at"`
	BenefitsDeclared        bool       `bun:"benefits_declared"`
	ConsentVersion          *int       `bun:"consent_version"`
	CustodyConfirmedAt      *time.Time `bun:"custody_confirmed_at"`
	DeclarationName         *string    `bun:"declaration_name"`
	DebtorNumber            *string    `bun:"debtor_number"`
	BenefitsProofAttached   bool       `bun:"benefits_proof_attached"`
}

// holidayReviewChildRow is one child as the review reads it, without its
// sealed health information.
type holidayReviewChildRow struct {
	ID                       int64         `bun:"id"`
	RegistrationID           int64         `bun:"holiday_registration_id"`
	FirstName                string        `bun:"first_name"`
	LastName                 string        `bun:"last_name"`
	BirthDate                calendar.Date `bun:"birth_date"`
	HomeSchoolID             *int64        `bun:"home_school_id"`
	GuestSchoolName          string        `bun:"guest_school_name"`
	DuplicateRegistrationIDs []int64       `bun:"duplicate_registration_ids,array"`
}

// holidayReviewBookingRow is one booking of a child.
type holidayReviewBookingRow struct {
	ChildID          int64      `bun:"holiday_registration_child_id"`
	WeekID           int64      `bun:"holiday_period_week_id"`
	TimeModelID      int64      `bun:"holiday_week_time_model_id"`
	Placement        string     `bun:"placement"`
	WaitlistPosition *int       `bun:"waitlist_position"`
	PromotedAt       *time.Time `bun:"promoted_at"`
}

const (
	reviewRegistrationsQuery = `SELECT id, status, freeze_override, submitted_at,
			guardian_first_name, guardian_last_name, guardian_email, guardian_phone,
			privacy_consent_at, health_consent_at, binding_confirmed_at, benefits_declared, consent_version,
			custody_confirmed_at, declaration_name,
			second_guardian_first_name, second_guardian_last_name, second_guardian_email, second_guardian_phone,
			guardian_address_street, guardian_address_postal_code, guardian_address_city, debtor_number,
			EXISTS (SELECT 1 FROM enrollment.holiday_registration_proofs AS proof
				WHERE proof.organization_id = holiday_registrations.organization_id
					AND proof.holiday_registration_id = holiday_registrations.id) AS benefits_proof_attached
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
	reviewChildrenQuery = `SELECT ` + reviewChildrenColumns + reviewChildrenFilter
	// reviewBookingsQuery reads every booking with its placement and, for a
	// waitlisted one of a registration that counts, its place in the waiting
	// list of the week (#4699): the order of the placement.
	reviewBookingsQuery = `SELECT booking.holiday_registration_child_id, booking.holiday_period_week_id,
			booking.holiday_week_time_model_id, booking.placement, booking.promoted_at,
			CASE WHEN booking.placement = 'waitlisted' AND registration.status IN ('received', 'confirmed') THEN (
				SELECT count(*) + 1
				FROM enrollment.holiday_registration_bookings AS ahead
				JOIN enrollment.holiday_registration_children AS ahead_child
					ON ahead_child.organization_id = ahead.organization_id AND ahead_child.id = ahead.holiday_registration_child_id
				JOIN enrollment.holiday_registrations AS ahead_registration
					ON ahead_registration.organization_id = ahead_child.organization_id
						AND ahead_registration.id = ahead_child.holiday_registration_id
				WHERE ahead.organization_id = booking.organization_id
					AND ahead.holiday_period_week_id = booking.holiday_period_week_id
					AND ahead.placement = 'waitlisted'
					AND ahead_registration.status IN ('received', 'confirmed')
					AND (ahead_registration.submitted_at, ahead_registration.id, ahead_child.position)
						< (registration.submitted_at, registration.id, child.position)
			) END AS waitlist_position
		FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		WHERE booking.organization_id = ? AND booking.holiday_period_id = ? AND (?::bigint = 0 OR child.holiday_registration_id = ?)`
)

// ReadHolidayReview reads the registrations of a period, or the one
// registration registrationID names, with their children and bookings: three
// statements for any number of registrations. It reads no health
// information; ReadHolidaySealedHealth does (#4722). A booking names only its
// week and time model. The second result is the freeze override of each
// registration: "frozen", "reopened" or empty when the deadline decides.
func (s *HolidayRegistrationStore) ReadHolidayReview(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayRegistrationDetail, map[int64]string, error) {
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
	if err := db.NewRaw(reviewChildrenQuery, organizationID, periodID, registrationID, registrationID).Scan(ctx, &children); err != nil {
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
		childBookings[booking.ChildID] = append(childBookings[booking.ChildID], enrollment.HolidayRegistrationBooking{
			WeekID: booking.WeekID, TimeModelID: booking.TimeModelID, Placement: booking.Placement,
			WaitlistPosition: booking.WaitlistPosition, PromotedAt: booking.PromotedAt,
		})
	}
	byRegistration := map[int64][]enrollment.HolidayRegistrationChild{}
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
	}
	details := make([]enrollment.HolidayRegistrationDetail, len(registrations))
	for i, row := range registrations {
		details[i] = enrollment.HolidayRegistrationDetail{
			HolidayRegistrationSummary: enrollment.HolidayRegistrationSummary{
				ID: row.ID, Status: row.Status, SubmittedAt: row.SubmittedAt,
				GuardianFirstName: row.GuardianFirstName, GuardianLastName: row.GuardianLastName,
				GuardianEmail: row.GuardianEmail, GuardianPhone: row.GuardianPhone, BenefitsDeclared: row.BenefitsDeclared,
				Children: byRegistration[row.ID],
			},
			PrivacyConsentAt: row.PrivacyConsentAt, HealthConsentAt: row.HealthConsentAt, BindingConfirmedAt: row.BindingConfirmedAt,
			CustodyConfirmedAt: row.CustodyConfirmedAt, BenefitsProofAttached: row.BenefitsProofAttached,
		}
		if row.DeclarationName != nil {
			details[i].DeclarationName = *row.DeclarationName
		}
		if row.ConsentVersion != nil {
			details[i].ConsentVersion = *row.ConsentVersion
		}
		if row.DebtorNumber != nil {
			details[i].DebtorNumber = *row.DebtorNumber
		}
		if row.AddressStreet != "" {
			details[i].GuardianAddress = &enrollment.HolidayAddress{
				Street: row.AddressStreet, PostalCode: row.AddressPostalCode, City: row.AddressCity,
			}
		}
		if row.SecondGuardianFirstName != "" {
			details[i].SecondGuardian = &enrollment.HolidayGuardianInput{
				FirstName: row.SecondGuardianFirstName, LastName: row.SecondGuardianLastName,
				Email: row.SecondGuardianEmail, Phone: row.SecondGuardianPhone,
			}
		}
	}
	return details
}

const (
	reviewFamilyHealthQuery = `SELECT health_data FROM enrollment.holiday_registrations
		WHERE organization_id = ? AND holiday_period_id = ? AND id = ?`
	reviewChildrenHealthQuery = `SELECT id, position, health_data FROM enrollment.holiday_registration_children
		WHERE organization_id = ? AND holiday_period_id = ? AND holiday_registration_id = ?
		ORDER BY position`
)

// ReadHolidaySealedHealth reads the sealed health information of one
// registration of the period: two statements; found is false for a
// registration outside the period (#4722).
func (s *HolidayRegistrationStore) ReadHolidaySealedHealth(ctx context.Context, organizationID, periodID, registrationID int64) (enrollment.HolidaySealedRegistrationHealth, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return enrollment.HolidaySealedRegistrationHealth{}, false, err
	}
	var families []struct {
		HealthData []byte `bun:"health_data"`
	}
	if err := db.NewRaw(reviewFamilyHealthQuery, organizationID, periodID, registrationID).Scan(ctx, &families); err != nil {
		return enrollment.HolidaySealedRegistrationHealth{}, false, fmt.Errorf("enrollment postgres: read sealed holiday family health: %w", err)
	}
	if len(families) == 0 {
		return enrollment.HolidaySealedRegistrationHealth{}, false, nil
	}
	var children []struct {
		ID         int64  `bun:"id"`
		Position   int    `bun:"position"`
		HealthData []byte `bun:"health_data"`
	}
	if err := db.NewRaw(reviewChildrenHealthQuery, organizationID, periodID, registrationID).Scan(ctx, &children); err != nil {
		return enrollment.HolidaySealedRegistrationHealth{}, false, fmt.Errorf("enrollment postgres: read sealed holiday child health: %w", err)
	}
	sealed := enrollment.HolidaySealedRegistrationHealth{Family: families[0].HealthData,
		Children: make([]enrollment.HolidaySealedChildHealth, len(children))}
	for i, child := range children {
		sealed.Children[i] = enrollment.HolidaySealedChildHealth{ChildID: child.ID, Position: child.Position, Sealed: child.HealthData}
	}
	return sealed, true, nil
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
			AND action IN ('confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'debtor_number', 'edit', 'proof_upload', 'sibling_merge', 'sibling_split', 'sibling_cancel')
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
	lockHolidayRegistrationQuery = `SELECT id, status, freeze_override, debtor_number
		FROM enrollment.holiday_registrations
		WHERE organization_id = ? AND holiday_period_id = ? AND id = ?
		FOR UPDATE`
	updateHolidayRegistrationStatusQuery = `UPDATE enrollment.holiday_registrations SET status = ?
		WHERE organization_id = ? AND id = ?`
	updateHolidayRegistrationFreezeQuery = `UPDATE enrollment.holiday_registrations SET freeze_override = ?
		WHERE organization_id = ? AND id = ?`
	updateHolidayDebtorNumberQuery = `UPDATE enrollment.holiday_registrations SET debtor_number = NULLIF(?, '')
		WHERE organization_id = ? AND id = ?`
)

// LockHolidayRegistration reads the status, the freeze override and the
// debtor number of one registration of the period and holds its row until
// the end of the transaction.
func (s *HolidayRegistrationStore) LockHolidayRegistration(ctx context.Context, organizationID, periodID, registrationID int64) (string, string, string, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return "", "", "", false, err
	}
	var rows []holidayReviewRegistrationRow
	if err := db.NewRaw(lockHolidayRegistrationQuery, organizationID, periodID, registrationID).Scan(ctx, &rows); err != nil {
		return "", "", "", false, fmt.Errorf("enrollment postgres: lock holiday registration: %w", err)
	}
	if len(rows) == 0 {
		return "", "", "", false, nil
	}
	debtorNumber := ""
	if rows[0].DebtorNumber != nil {
		debtorNumber = *rows[0].DebtorNumber
	}
	return rows[0].Status, holidayFreezeOverrides(rows)[rows[0].ID], debtorNumber, true, nil
}

// SetHolidayDebtorNumber writes the debtor number of a registration; an
// empty one stores NULL (#5621).
func (s *HolidayRegistrationStore) SetHolidayDebtorNumber(ctx context.Context, organizationID, registrationID int64, debtorNumber string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(updateHolidayDebtorNumberQuery, debtorNumber, organizationID, registrationID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: update holiday debtor number: %w", err)
	}
	return nil
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
	// deleteHolidayChildBookingsQuery removes the bookings of the weeks the
	// child no longer books and names their weeks.
	deleteHolidayChildBookingsQuery = `DELETE FROM enrollment.holiday_registration_bookings
		WHERE organization_id = ? AND holiday_registration_child_id = ? AND NOT (holiday_period_week_id = ANY(?::bigint[]))
		RETURNING holiday_period_week_id`
	// insertHolidayChildBookingsQuery keeps a week the child already books
	// with its placement and changes only its time model; a new week waits
	// for its placement (#4699).
	insertHolidayChildBookingsQuery = `INSERT INTO enrollment.holiday_registration_bookings AS booking
			(organization_id, holiday_period_id, holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement)
		SELECT ?, ?, ?, wanted.week_id, wanted.time_model_id, 'waitlisted'
		FROM unnest(?::bigint[], ?::bigint[]) AS wanted(week_id, time_model_id)
		ON CONFLICT (holiday_registration_child_id, holiday_period_week_id)
			DO UPDATE SET holiday_week_time_model_id = EXCLUDED.holiday_week_time_model_id
		RETURNING booking.holiday_period_week_id, (xmax = 0) AS inserted`
)

// ReplaceHolidayChildBookings makes bookings the bookings of the child and
// returns the weeks whose places changed: the weeks it left and the weeks
// it newly books, which wait for their placement (#4699). A week it keeps
// keeps its placement. The caller checked that the child belongs to the
// locked registration and that every week and time model belongs to the
// period.
func (s *HolidayRegistrationStore) ReplaceHolidayChildBookings(ctx context.Context, organizationID, periodID, childID int64, bookings []enrollment.HolidayBookingInput) ([]int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	weekIDs := make([]int64, len(bookings))
	modelIDs := make([]int64, len(bookings))
	for i, booking := range bookings {
		weekIDs[i], modelIDs[i] = booking.WeekID, booking.TimeModelID
	}
	var left []int64
	if err := db.NewRaw(deleteHolidayChildBookingsQuery, organizationID, childID, pgdialect.Array(weekIDs)).Scan(ctx, &left); err != nil {
		return nil, fmt.Errorf("enrollment postgres: remove holiday child bookings: %w", err)
	}
	var written []struct {
		WeekID   int64 `bun:"holiday_period_week_id"`
		Inserted bool  `bun:"inserted"`
	}
	if err := db.NewRaw(insertHolidayChildBookingsQuery, organizationID, periodID, childID,
		pgdialect.Array(weekIDs), pgdialect.Array(modelIDs)).Scan(ctx, &written); err != nil {
		return nil, fmt.Errorf("enrollment postgres: insert holiday child bookings: %w", err)
	}
	for _, week := range written {
		if week.Inserted {
			left = append(left, week.WeekID)
		}
	}
	return left, nil
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

const (
	reviewRegistrationAnswersQuery = `SELECT id, form_version, custom_data, legal_acceptances
		FROM enrollment.holiday_registrations
		WHERE organization_id = ? AND holiday_period_id = ? AND (?::bigint = 0 OR id = ?)`
	reviewChildAnswersQuery = `SELECT id, holiday_registration_id, custom_data
		FROM enrollment.holiday_registration_children
		WHERE organization_id = ? AND holiday_period_id = ? AND (?::bigint = 0 OR holiday_registration_id = ?)
		ORDER BY holiday_registration_id, position`
)

// ReadHolidayAnswers reads the form version and the answers of the
// registrations of a period, or of the one registrationID names, with the
// answers of their children (#5198): two statements for any number of
// registrations. Only the detail and the export read them; the list does
// not.
func (s *HolidayRegistrationStore) ReadHolidayAnswers(ctx context.Context, organizationID, periodID, registrationID int64) (map[int64]enrollment.HolidayRegistrationAnswers, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var registrations []struct {
		ID               int64                     `bun:"id"`
		LegalAcceptances map[string]time.Time      `bun:"legal_acceptances,type:jsonb"`
		FormVersion      int                       `bun:"form_version"`
		CustomData       enrollment.HolidayAnswers `bun:"custom_data,type:jsonb"`
	}
	if err := db.NewRaw(reviewRegistrationAnswersQuery, organizationID, periodID, registrationID, registrationID).
		Scan(ctx, &registrations); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday registration answers: %w", err)
	}
	answers := make(map[int64]enrollment.HolidayRegistrationAnswers, len(registrations))
	if len(registrations) == 0 {
		return answers, nil
	}
	for _, row := range registrations {
		answers[row.ID] = enrollment.HolidayRegistrationAnswers{
			LegalAcceptances: row.LegalAcceptances, FormVersion: row.FormVersion, CustomData: answersOrEmpty(row.CustomData), Children: []enrollment.HolidayChildAnswers{},
		}
	}
	var children []struct {
		ID             int64                     `bun:"id"`
		RegistrationID int64                     `bun:"holiday_registration_id"`
		CustomData     enrollment.HolidayAnswers `bun:"custom_data,type:jsonb"`
	}
	if err := db.NewRaw(reviewChildAnswersQuery, organizationID, periodID, registrationID, registrationID).
		Scan(ctx, &children); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday registration child answers: %w", err)
	}
	for _, child := range children {
		registration, ok := answers[child.RegistrationID]
		if !ok {
			continue
		}
		registration.Children = append(registration.Children, enrollment.HolidayChildAnswers{
			ChildID: child.ID, CustomData: answersOrEmpty(child.CustomData),
		})
		answers[child.RegistrationID] = registration
	}
	return answers, nil
}
