package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

const findHolidayRegistrationLinkQuery = `SELECT organization_id, id, holiday_wish_rule,
		(SELECT min(week.starts_on) FROM enrollment.holiday_period_weeks AS week
			WHERE week.organization_id = period.organization_id AND week.holiday_period_id = period.id) AS first_day
	FROM enrollment.holiday_periods AS period
	WHERE public_token = ? AND status = 'published'`

// FindHolidayRegistrationLink reads the Träger and the published period of a
// link with the period's holiday care wish rule and first day (#4703). Only
// the administrative transaction may read across Träger.
func (s *HolidayRegistrationStore) FindHolidayRegistrationLink(ctx context.Context, token string) (enrollment.HolidayRegistrationLink, bool, error) {
	if !tenant.IsAdminTx(ctx) {
		return enrollment.HolidayRegistrationLink{}, false, errors.New("enrollment postgres: the link lookup requires the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return enrollment.HolidayRegistrationLink{}, false, err
	}
	var rows []struct {
		OrganizationID  int64          `bun:"organization_id"`
		ID              int64          `bun:"id"`
		HolidayWishRule bool           `bun:"holiday_wish_rule"`
		FirstDay        *calendar.Date `bun:"first_day"`
	}
	if err := db.NewRaw(findHolidayRegistrationLinkQuery, token).Scan(ctx, &rows); err != nil {
		return enrollment.HolidayRegistrationLink{}, false, fmt.Errorf("enrollment postgres: read holiday registration link: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.HolidayRegistrationLink{}, false, nil
	}
	link := enrollment.HolidayRegistrationLink{OrganizationID: rows[0].OrganizationID, PeriodID: rows[0].ID, HolidayWishRule: rows[0].HolidayWishRule}
	if rows[0].FirstDay != nil {
		link.FirstDay = *rows[0].FirstDay
	}
	return link, true, nil
}

const announcedHolidayRegistrationsQuery = `SELECT period.organization_id, period.id, period.public_token, period.name,
		period.registration_opens_on, period.registration_closes_on,
		(SELECT min(week.starts_on) FROM enrollment.holiday_period_weeks AS week
			WHERE week.organization_id = period.organization_id AND week.holiday_period_id = period.id) AS first_day,
		(SELECT max(week.ends_on) FROM enrollment.holiday_period_weeks AS week
			WHERE week.organization_id = period.organization_id AND week.holiday_period_id = period.id) AS last_day
	FROM enrollment.holiday_periods AS period
	WHERE period.organization_id = ANY(?) AND period.status = 'published' AND period.registration_closes_on >= ?
	ORDER BY period.registration_closes_on, period.registration_opens_on, period.id`

// AnnouncedHolidayRegistrations reads the published periods of the given
// Träger whose registration has not closed before on, in one statement
// (#6671). Only the administrative transaction may read across Träger.
func (s *HolidayRegistrationStore) AnnouncedHolidayRegistrations(ctx context.Context, organizationIDs []int64, on calendar.Date) ([]enrollment.HolidayRegistrationAnnouncement, error) {
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("enrollment postgres: the announced holiday registrations require the administrative transaction")
	}
	if len(organizationIDs) == 0 {
		return []enrollment.HolidayRegistrationAnnouncement{}, nil
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		OrganizationID       int64          `bun:"organization_id"`
		ID                   int64          `bun:"id"`
		PublicToken          string         `bun:"public_token"`
		Name                 string         `bun:"name"`
		RegistrationOpensOn  calendar.Date  `bun:"registration_opens_on"`
		RegistrationClosesOn calendar.Date  `bun:"registration_closes_on"`
		FirstDay             *calendar.Date `bun:"first_day"`
		LastDay              *calendar.Date `bun:"last_day"`
	}
	if err := db.NewRaw(announcedHolidayRegistrationsQuery, pgdialect.Array(organizationIDs), on).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read announced holiday registrations: %w", err)
	}
	announcements := make([]enrollment.HolidayRegistrationAnnouncement, len(rows))
	for i, row := range rows {
		announcements[i] = enrollment.HolidayRegistrationAnnouncement{
			OrganizationID: row.OrganizationID, PeriodID: row.ID, Token: row.PublicToken, Name: row.Name,
			RegistrationOpensOn: row.RegistrationOpensOn, RegistrationClosesOn: row.RegistrationClosesOn,
		}
		if row.FirstDay != nil && row.LastDay != nil {
			announcements[i].FirstDay, announcements[i].LastDay = *row.FirstDay, *row.LastDay
		}
	}
	return announcements, nil
}

const (
	// deleteExpiredHolidayRateLimitsQuery keeps the counters of a Träger
	// small.
	deleteExpiredHolidayRateLimitsQuery = `DELETE FROM enrollment.holiday_registration_rate_limits
		WHERE organization_id = ? AND window_start < statement_timestamp() - (? * INTERVAL '1 second')`
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
func (s *HolidayRegistrationStore) CountHolidayRegistrationAttempt(ctx context.Context, organizationID int64, keyType, value string, window, retention time.Duration) (int, error) {
	seconds := int64(window.Seconds())
	if seconds <= 0 {
		return 0, fmt.Errorf("enrollment postgres: invalid rate-limit window %s", window)
	}
	retentionSeconds, err := positiveSeconds(retention)
	if err != nil {
		return 0, err
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	if _, err := db.NewRaw(deleteExpiredHolidayRateLimitsQuery, organizationID, retentionSeconds).Exec(ctx); err != nil {
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
			 privacy_consent_at, health_consent_at, binding_confirmed_at, custody_confirmed_at, declaration_name, benefits_declared,
			second_guardian_first_name, second_guardian_last_name, second_guardian_email, second_guardian_phone,
			form_version, custom_data, legal_acceptances, consent_version,
			guardian_address_street, guardian_address_postal_code, guardian_address_city, retention_days)
		VALUES (?, ?, ?, ?, ?, ?, statement_timestamp(),
			CASE WHEN ? THEN statement_timestamp() END, CASE WHEN ? THEN statement_timestamp() END,
			CASE WHEN ? THEN statement_timestamp() END, NULLIF(?, ''), ?,
			NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?::jsonb,
			(SELECT COALESCE(jsonb_object_agg(key, statement_timestamp()), '{}'::jsonb) FROM unnest(?::text[]) AS accepted(key)),
			NULLIF(?, 0), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)
		RETURNING id, submitted_at`
	setHolidayFamilyHealthQuery = `UPDATE enrollment.holiday_registrations SET health_data = ?
		WHERE organization_id = ? AND id = ?`
	// insertHolidayBookingsQuery puts every booking on the waiting list:
	// only the placement under the lock of its week gives it a place (#4699).
	// In a period by criteria it is pending instead: only the Büro gives it
	// a place (#4700).
	// A booking of days carries its days as an array literal, a booking of
	// the whole week an empty text (#4701).
	insertHolidayBookingsQuery = `INSERT INTO enrollment.holiday_registration_bookings
			(organization_id, holiday_period_id, holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement, days)
		SELECT ?, ?, booking.child_id, booking.week_id, booking.time_model_id, ?::text, NULLIF(booking.days, '')::date[]
		FROM unnest(?::bigint[], ?::bigint[], ?::bigint[], ?::text[]) AS booking(child_id, week_id, time_model_id, days)`
	insertHolidayContactsQuery = `INSERT INTO enrollment.holiday_registration_contacts
			(organization_id, holiday_registration_id, kind, position, name, phone, relationship)
		SELECT ?, ?, contact.kind, contact.position, contact.name, contact.phone, contact.relationship
		FROM unnest(?::text[], ?::smallint[], ?::text[], ?::text[], ?::text[])
			AS contact(kind, position, name, phone, relationship)`
)

// holidayChildRow is one child of a registration; a nil HomeSchoolID makes a
// holiday guest. HealthData is its sealed health information (#4722).
type holidayChildRow struct {
	bun.BaseModel `bun:"table:enrollment.holiday_registration_children"`

	ID                    int64                     `bun:"id,pk,autoincrement"`
	OrganizationID        int64                     `bun:"organization_id"`
	HolidayPeriodID       int64                     `bun:"holiday_period_id"`
	HolidayRegistrationID int64                     `bun:"holiday_registration_id"`
	Position              int                       `bun:"position"`
	FirstName             string                    `bun:"first_name"`
	LastName              string                    `bun:"last_name"`
	BirthDate             calendar.Date             `bun:"birth_date,type:date"`
	HomeSchoolID          *int64                    `bun:"home_school_id"`
	GuestSchoolName       string                    `bun:"guest_school_name"`
	HealthData            []byte                    `bun:"health_data"`
	CustomData            enrollment.HolidayAnswers `bun:"custom_data,type:jsonb"`
	HolidayWish           *string                   `bun:"holiday_wish"`
	HolidayWishSource     *string                   `bun:"holiday_wish_source"`
}

// answersOrEmpty keeps a JSONB object column an object.
func answersOrEmpty(answers enrollment.HolidayAnswers) enrollment.HolidayAnswers {
	if answers == nil {
		return enrollment.HolidayAnswers{}
	}
	return answers
}

// encodeAnswers encodes the answers of a registration for a JSONB column.
func encodeAnswers(answers enrollment.HolidayAnswers) (string, error) {
	encoded, err := json.Marshal(answersOrEmpty(answers))
	if err != nil {
		return "", fmt.Errorf("enrollment postgres: encode holiday registration answers: %w", err)
	}
	return string(encoded), nil
}

// InsertHolidayRegistration stores the registration with its children,
// bookings and contacts: one statement for the registration, one for its
// sealed health information when it has any, one per child and one each for
// all bookings and all contacts. seal returns the sealed values for the new
// registration's ID; the store writes no plaintext health information.
func (s *HolidayRegistrationStore) InsertHolidayRegistration(ctx context.Context, organizationID, periodID int64, input enrollment.HolidayRegistrationInput, waiting string,
	seal func(registrationID int64) (enrollment.HolidaySealedHealth, error)) (enrollment.HolidayRegistrationReceipt, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	var receipt struct {
		ID          int64     `bun:"id"`
		SubmittedAt time.Time `bun:"submitted_at"`
	}
	guardian := input.Guardian
	second := enrollment.HolidayGuardianInput{}
	if input.SecondGuardian != nil {
		second = *input.SecondGuardian
	}
	address := enrollment.HolidayAddress{}
	if input.GuardianAddress != nil {
		address = *input.GuardianAddress
	}
	answers, err := encodeAnswers(input.CustomData)
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	if err := db.NewRaw(insertHolidayRegistrationQuery, organizationID, periodID, guardian.FirstName, guardian.LastName,
		guardian.Email, guardian.Phone, input.HealthConsent, input.BindingConfirmed, input.CustodyConfirmed, input.DeclarationName, input.BenefitsDeclared,
		second.FirstName, second.LastName, second.Email, second.Phone, input.FormVersion, answers, pgdialect.Array(acceptedHolidayKeys(input.LegalAcceptances)),
		input.ConsentVersion, address.Street, address.PostalCode, address.City, input.RetentionDays).Scan(ctx, &receipt); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, fmt.Errorf("enrollment postgres: insert holiday registration: %w", err)
	}
	sealed, err := storeHolidayFamilyHealth(ctx, db, organizationID, receipt.ID, len(input.Children), seal)
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	childIDs, err := insertHolidayChildren(ctx, db, organizationID, periodID, receipt.ID, input.Children, sealed.Children, waiting)
	if err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	if err := insertHolidayCriterionAnswers(ctx, db, organizationID, periodID, receipt.ID, input, childIDs); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	if err := insertHolidayContacts(ctx, db, organizationID, receipt.ID, input); err != nil {
		return enrollment.HolidayRegistrationReceipt{}, err
	}
	return enrollment.HolidayRegistrationReceipt{ID: receipt.ID, SubmittedAt: receipt.SubmittedAt}, nil
}

// storeHolidayFamilyHealth seals the health information for the new
// registration and stores its family part; it returns the children's parts.
func storeHolidayFamilyHealth(ctx context.Context, db bun.IDB, organizationID, registrationID int64, children int,
	seal func(registrationID int64) (enrollment.HolidaySealedHealth, error)) (enrollment.HolidaySealedHealth, error) {
	sealed, err := seal(registrationID)
	if err != nil {
		return enrollment.HolidaySealedHealth{}, err
	}
	if len(sealed.Children) != children {
		return enrollment.HolidaySealedHealth{}, errors.New("enrollment postgres: the sealed health information does not match the children")
	}
	if sealed.Family != nil {
		if _, err := db.NewRaw(setHolidayFamilyHealthQuery, sealed.Family, organizationID, registrationID).Exec(ctx); err != nil {
			return enrollment.HolidaySealedHealth{}, fmt.Errorf("enrollment postgres: store sealed holiday family health: %w", err)
		}
	}
	return sealed, nil
}

// insertHolidayChildren stores one row per child with its sealed health
// information, then all bookings in one statement with the placement
// waiting, and returns the children's IDs in their order.
func insertHolidayChildren(ctx context.Context, db bun.IDB, organizationID, periodID, registrationID int64,
	children []enrollment.HolidayChildInput, sealed [][]byte, waiting string) ([]int64, error) {
	ids := make([]int64, len(children))
	var childIDs, weekIDs, modelIDs []int64
	var days []string
	for i, child := range children {
		row := holidayChildRow{
			OrganizationID: organizationID, HolidayPeriodID: periodID, HolidayRegistrationID: registrationID, Position: i + 1,
			FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
			GuestSchoolName: child.GuestSchoolName, HealthData: sealed[i],
			CustomData: answersOrEmpty(child.CustomData),
		}
		if child.HomeSchoolID > 0 {
			row.HomeSchoolID = &child.HomeSchoolID
			if child.HolidayWish != "" {
				row.HolidayWish, row.HolidayWishSource = &child.HolidayWish, &child.HolidayWishSource
			}
		}
		if _, err := db.NewInsert().Model(&row).ExcludeColumn("id").Returning("id").Exec(ctx); err != nil {
			return nil, fmt.Errorf("enrollment postgres: insert holiday registration child: %w", err)
		}
		ids[i] = row.ID
		for _, booking := range child.Bookings {
			childIDs = append(childIDs, row.ID)
			weekIDs = append(weekIDs, booking.WeekID)
			modelIDs = append(modelIDs, booking.TimeModelID)
			days = append(days, holidayDaysLiteral(booking.Days))
		}
	}
	if _, err := db.NewRaw(insertHolidayBookingsQuery, organizationID, periodID, waiting, pgdialect.Array(childIDs),
		pgdialect.Array(weekIDs), pgdialect.Array(modelIDs), pgdialect.Array(days)).Exec(ctx); err != nil {
		return nil, fmt.Errorf("enrollment postgres: insert holiday registration bookings: %w", err)
	}
	return ids, nil
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
	// deleteEndedHolidayRegistrationsQuery deletes, in one statement, every
	// registration whose period ended more days before the given day than
	// the registration keeps (#6691), with its children, contacts and
	// bookings (cascade) and its entries of the audit trail (#4663). The
	// entries of a list or an export name no registration: they go once the
	// period ended more than the health retention ago and none of its
	// registrations stays. In the registrations that stay, the statement
	// deletes the sealed health information of the periods that ended more
	// than the health retention ago. It writes one row per period into the
	// deletion log with the counts of what went (#4667, ADR 0118). All parts
	// read the same snapshot, so the counted children are the ones the
	// cascade deletes. A period with nothing to delete writes no row.
	// Parameters: today, health retention in days.
	deleteEndedHolidayRegistrationsQuery = `WITH run AS (
			SELECT ?::date AS today, ?::int AS health_days
		), ended AS (
			SELECT week.organization_id, week.holiday_period_id, max(week.ends_on) AS ended_on
			FROM enrollment.holiday_period_weeks AS week
			GROUP BY week.organization_id, week.holiday_period_id
			HAVING max(week.ends_on) + (SELECT health_days FROM run) < (SELECT today FROM run)
		), expired AS (
			SELECT registration.organization_id, registration.holiday_period_id, registration.id
			FROM enrollment.holiday_registrations AS registration
			JOIN ended ON ended.organization_id = registration.organization_id
				AND ended.holiday_period_id = registration.holiday_period_id
			WHERE ended.ended_on + registration.retention_days < (SELECT today FROM run)
		), registrations AS (
			DELETE FROM enrollment.holiday_registrations AS registration
			USING expired
			WHERE registration.organization_id = expired.organization_id AND registration.id = expired.id
			RETURNING registration.organization_id, registration.holiday_period_id, registration.id
		), audit AS (
			DELETE FROM enrollment.holiday_registration_audit AS entry
			USING ended
			WHERE entry.organization_id = ended.organization_id
				AND entry.holiday_period_id = ended.holiday_period_id
				AND (entry.holiday_registration_id IN (SELECT id FROM expired WHERE expired.organization_id = entry.organization_id)
					OR (entry.holiday_registration_id IS NULL AND NOT EXISTS (
						SELECT 1 FROM enrollment.holiday_registrations AS kept
						WHERE kept.organization_id = ended.organization_id
							AND kept.holiday_period_id = ended.holiday_period_id
							AND kept.id NOT IN (SELECT id FROM expired WHERE expired.organization_id = kept.organization_id))))
			RETURNING entry.organization_id, entry.holiday_period_id
		), family_health AS (
			UPDATE enrollment.holiday_registrations AS registration SET health_data = NULL
			FROM ended
			WHERE registration.organization_id = ended.organization_id
				AND registration.holiday_period_id = ended.holiday_period_id
				AND registration.health_data IS NOT NULL
				AND registration.id NOT IN (SELECT id FROM expired WHERE expired.organization_id = registration.organization_id)
			RETURNING registration.organization_id, registration.holiday_period_id
		), child_health AS (
			UPDATE enrollment.holiday_registration_children AS child SET health_data = NULL
			FROM ended
			WHERE child.organization_id = ended.organization_id
				AND child.holiday_period_id = ended.holiday_period_id
				AND child.health_data IS NOT NULL
				AND child.holiday_registration_id NOT IN (SELECT id FROM expired WHERE expired.organization_id = child.organization_id)
			RETURNING child.organization_id, child.holiday_period_id
		), counts AS (
			SELECT ended.organization_id, ended.holiday_period_id, ended.ended_on,
				(SELECT count(*) FROM registrations
					WHERE registrations.organization_id = ended.organization_id
						AND registrations.holiday_period_id = ended.holiday_period_id) AS registrations,
				(SELECT count(*) FROM enrollment.holiday_registration_children AS child
					JOIN registrations ON registrations.organization_id = child.organization_id
						AND registrations.id = child.holiday_registration_id
					WHERE registrations.organization_id = ended.organization_id
						AND registrations.holiday_period_id = ended.holiday_period_id) AS children,
				(SELECT count(*) FROM audit
					WHERE audit.organization_id = ended.organization_id
						AND audit.holiday_period_id = ended.holiday_period_id) AS audit_entries,
				(SELECT count(*) FROM family_health
					WHERE family_health.organization_id = ended.organization_id
						AND family_health.holiday_period_id = ended.holiday_period_id)
				+ (SELECT count(*) FROM child_health
					WHERE child_health.organization_id = ended.organization_id
						AND child_health.holiday_period_id = ended.holiday_period_id) AS health_records
			FROM ended
		)
		INSERT INTO enrollment.holiday_registration_deletions
			(organization_id, holiday_period_id, period_ended_on, registrations, children, audit_entries, health_records)
		SELECT organization_id, holiday_period_id, ended_on, registrations, children, audit_entries, health_records
		FROM counts
		WHERE registrations + audit_entries + health_records > 0
		RETURNING id, organization_id, holiday_period_id, period_ended_on, registrations, children, audit_entries, health_records, deleted_at`
	deleteAllExpiredHolidayRateLimitsQuery = `DELETE FROM enrollment.holiday_registration_rate_limits
		WHERE window_start < statement_timestamp() - (? * INTERVAL '1 second')`
	deleteExpiredHolidayRegistrationDeletionsQuery = `DELETE FROM enrollment.holiday_registration_deletions
		WHERE deleted_at < statement_timestamp() - make_interval(years => ?)`
)

// holidayRegistrationDeletionRow is one row of the deletion log: the counts
// of what the retention deleted for one period, no names (ADR 0118).
type holidayRegistrationDeletionRow struct {
	bun.BaseModel `bun:"table:enrollment.holiday_registration_deletions"`

	ID              int64         `bun:"id,pk,autoincrement"`
	OrganizationID  int64         `bun:"organization_id"`
	HolidayPeriodID int64         `bun:"holiday_period_id"`
	PeriodEndedOn   calendar.Date `bun:"period_ended_on,type:date"`
	Registrations   int           `bun:"registrations"`
	Children        int           `bun:"children"`
	AuditEntries    int           `bun:"audit_entries"`
	HealthRecords   int           `bun:"health_records"`
	DeletedAt       time.Time     `bun:"deleted_at"`
}

// DeleteExpiredHolidayRegistrations deletes, across every Träger, the
// registrations whose period ended more days before today than they keep
// with their audit trail, and the health information of the periods that
// ended more than healthDays before today. It logs each period in the
// deletion log of its Träger and deletes the expired abuse counters. Only
// the administrative transaction of the hourly cleanup may delete across
// Träger.
func (s *HolidayRegistrationStore) DeleteExpiredHolidayRegistrations(ctx context.Context, today calendar.Date, healthDays int, counterRetention time.Duration) (int, error) {
	if !tenant.IsAdminTx(ctx) {
		return 0, errors.New("enrollment postgres: the holiday registration retention requires the administrative transaction")
	}
	retentionSeconds, err := positiveSeconds(counterRetention)
	if err != nil {
		return 0, err
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return 0, err
	}
	var logged []holidayRegistrationDeletionRow
	if err := db.NewRaw(deleteEndedHolidayRegistrationsQuery, today, healthDays).Scan(ctx, &logged); err != nil {
		return 0, fmt.Errorf("enrollment postgres: delete ended holiday registrations: %w", err)
	}
	deleted := 0
	for _, period := range logged {
		deleted += period.Registrations + period.AuditEntries + period.HealthRecords
	}
	counters, err := affectedRows(db.NewRaw(deleteAllExpiredHolidayRateLimitsQuery, retentionSeconds).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("enrollment postgres: delete expired holiday registration counters: %w", err)
	}
	return deleted + counters, nil
}

// DeleteExpiredHolidayRegistrationDeletions deletes, across every Träger,
// the rows of the deletion log older than years. Only the administrative
// transaction of the hourly cleanup may delete across Träger.
func (s *HolidayRegistrationStore) DeleteExpiredHolidayRegistrationDeletions(ctx context.Context, years int) (int, error) {
	if !tenant.IsAdminTx(ctx) {
		return 0, errors.New("enrollment postgres: the holiday registration deletion log retention requires the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return 0, err
	}
	deleted, err := affectedRows(db.NewRaw(deleteExpiredHolidayRegistrationDeletionsQuery, years).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("enrollment postgres: delete expired holiday registration deletion log: %w", err)
	}
	return deleted, nil
}

func affectedRows(result sql.Result, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	return int(rows), err
}

func acceptedHolidayKeys(acceptances map[string]bool) []string {
	keys := []string{}
	for key, accepted := range acceptances {
		if accepted {
			keys = append(keys, key)
		}
	}
	return keys
}

// holidayDaysLiteral writes the days of a booking as a PostgreSQL array
// literal; no days, a booking of the whole week, is an empty text (#4701).
func holidayDaysLiteral(days []calendar.Date) string {
	if len(days) == 0 {
		return ""
	}
	parts := make([]string, len(days))
	for i, day := range days {
		parts[i] = day.String()
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// holidayDaysOf reads the days of a booking scanned as text, in the form
// PostgreSQL writes a date array; NULL or an empty text is none.
func holidayDaysOf(raw *string) ([]calendar.Date, error) {
	if raw == nil {
		return nil, nil
	}
	text := strings.Trim(*raw, "{}")
	if text == "" {
		return nil, nil
	}
	parts := strings.Split(text, ",")
	days := make([]calendar.Date, len(parts))
	for i, part := range parts {
		day, err := calendar.ParseDate(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("enrollment postgres: read booked holiday day: %w", err)
		}
		days[i] = day
	}
	return days, nil
}
