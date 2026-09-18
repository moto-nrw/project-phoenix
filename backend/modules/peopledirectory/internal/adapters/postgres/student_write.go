package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres/calendar"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// studentWritableColumns are the owned columns a full-row write sets. The five
// departure columns are deliberately absent: PersistDeparturePlan is their only
// writer, because the plan that lands there is resolved against the stored one
// rather than taken from the caller's struct.
const studentWritableColumns = `person_id, school_class, group_id, status,
	enrolled_from, enrolled_until,
	guardian_name, guardian_contact, guardian_email, guardian_phone,
	address_street, address_city, address_postal_code,
	extra_info, supervisor_notes, health_info, pickup_status,
	sick, sick_since, excused, excused_since,
	photo_path, photo_consent_given_at, photo_consent_given_by,
	agb_accepted_at, data_processing_accepted_at, email_contact_accepted_at`

// studentDateParam binds an unset calendar day as NULL rather than as the zero
// date, which a DATE column would otherwise store as year zero. An unparseable
// value is a bug on the caller's side of a contract that already validated the
// format; NULL is the honest outcome rather than a year-zero row.
func studentDateParam(value string) *calendar.Date {
	if value == "" {
		return nil
	}
	parsed, err := calendar.ParseDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}

// InsertRecord writes a new child and returns the row as stored, so the caller
// sees the assigned id and the database's timestamps.
func (s *StudentStore) InsertRecord(
	ctx context.Context,
	record domain.StudentRecord,
) (domain.StudentRecord, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StudentRecord{}, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return domain.StudentRecord{}, domain.OperationStats{}, err
	}
	// An unset status means the column's default, which a check constraint
	// would otherwise reject as an empty string. The caller that says nothing
	// about the lifecycle gets an actively enrolled child, as it always did.
	if record.Status == "" {
		record.Status = domain.StudentStatusActive
	}

	row := newStudentWriteRow(record, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().Model(&row).
		ModelTableExpr(`users.students`).
		// Only the owned columns: id and the timestamps are the database's,
		// and the departure columns belong to PersistDeparturePlan alone.
		Column(append([]string{"tenant_id"}, studentWritableColumnList()...)...).
		Returning(studentReturningColumns).
		Exec(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StudentRecord{}, stats, fmt.Errorf("people directory postgres: insert student: %w", err)
	}
	stats.Rows = 1
	return row.toDomain(), stats, nil
}

// UpdateRecord rewrites the owned columns of one child. It reports false when
// the tenant has no such row, which is the caller's not-found.
func (s *StudentStore) UpdateRecord(
	ctx context.Context,
	record domain.StudentRecord,
) (domain.StudentRecord, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StudentRecord{}, false, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return domain.StudentRecord{}, false, domain.OperationStats{}, err
	}

	row := newStudentWriteRow(record, tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().Model(&row).
		ModelTableExpr(`users.students AS "student"`).
		Column(studentWritableColumnList()...).
		Set(`updated_at = NOW()`).
		Where(`"student".id = ?`, record.ID).
		Where(`"student".tenant_id = ?`, tenantID).
		Returning(studentReturningColumns).
		Exec(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		// RETURNING over a WHERE that matched nothing: the tenant has no such
		// child, which is the caller's not-found rather than a failure.
		if errors.Is(err, sql.ErrNoRows) {
			return domain.StudentRecord{}, false, stats, nil
		}
		return domain.StudentRecord{}, false, stats, fmt.Errorf("people directory postgres: update student: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.StudentRecord{}, false, stats, fmt.Errorf("people directory postgres: update student: %w", err)
	}
	if affected == 0 {
		return domain.StudentRecord{}, false, stats, nil
	}
	stats.Rows = affected
	return row.toDomain(), true, stats, nil
}

// DeleteRecord removes one child of the tenant, reporting whether a row went.
func (s *StudentStore) DeleteRecord(
	ctx context.Context,
	studentID int64,
) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewDelete().
		TableExpr(`users.students AS "student"`).
		Where(`"student".id = ?`, studentID).
		Where(`"student".tenant_id = ?`, tenantID).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: delete student: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: delete student: %w", err)
	}
	stats.Rows = affected
	return affected > 0, stats, nil
}

// PersistDeparturePlan writes the unified per-weekday plan and its derived
// legacy mirrors in one statement, so every caller that touches the plan —
// through the mode set, the unified days or a legacy map — leaves the columns
// consistent.
//
// planTouched false writes only the companion note: a caller that changed an
// unrelated field must not have its stale plan re-persisted, but an orphan note
// is still cleared, because the free-text "mit wem" must never outlive the
// accompanied mode that justifies it.
func (s *StudentStore) PersistDeparturePlan(
	ctx context.Context,
	studentID int64,
	plan domain.DeparturePlan,
	note *string,
	planTouched bool,
) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().
		TableExpr(`users.students AS "student"`).
		Where(`"student".id = ?`, studentID)
	query = withStudentTenant(query, tenantID)

	if planTouched {
		query = query.
			Set(`pickup_status = ?`, plan.AllowedDepartureModes.LegacyPickupStatus()).
			Set(`departure_days = ?`, plan.DepartureDays).
			Set(`allowed_departure_modes = ?`, plan.AllowedDepartureModes).
			Set(`bus_days = ?`, plan.BusDays).
			Set(`pickup_days = ?`, plan.PickupDays)
	}
	query = query.Set(`departure_companion_note = ?`, note)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: persist departure plan: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: persist departure plan: %w", err)
	}
	stats.Rows = affected
	if affected != 1 {
		return stats, fmt.Errorf("people directory postgres: persist departure plan: expected 1 row, got %d", affected)
	}
	return stats, nil
}

// FindDeparturePlan reads the plan the row currently holds, already resolved
// through the owner's read precedence.
func (s *StudentStore) FindDeparturePlan(
	ctx context.Context,
	studentID int64,
) (domain.DeparturePlan, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.DeparturePlan{}, false, domain.OperationStats{}, err
	}
	var rows []struct {
		BusDays               domain.BusDays               `bun:"bus_days"`
		PickupDays            domain.PickupDays            `bun:"pickup_days"`
		DepartureDays         domain.DepartureDays         `bun:"departure_days"`
		AllowedDepartureModes domain.AllowedDepartureModes `bun:"allowed_departure_modes"`
	}
	query := withStudentTenant(db.NewSelect().
		TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".bus_days, "student".pickup_days`).
		ColumnExpr(`"student".departure_days, "student".allowed_departure_modes`).
		Where(`"student".id = ?`, studentID), tenantID)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.DeparturePlan{}, false, stats, fmt.Errorf("people directory postgres: find departure plan: %w", err)
	}
	if len(rows) == 0 {
		return domain.DeparturePlan{}, false, stats, nil
	}
	stats.Rows = 1
	stored := domain.DeparturePlan{
		AllowedDepartureModes: rows[0].AllowedDepartureModes,
		DepartureDays:         rows[0].DepartureDays,
		BusDays:               rows[0].BusDays,
		PickupDays:            rows[0].PickupDays,
	}
	// Effective, not merely normalized: a row whose plan lives only in the
	// legacy maps still has to read as the plan it means, or a write comparing
	// against it would see a change the caller never made.
	return stored.Effective(), true, stats, nil
}

// studentReturningColumns is studentRecordColumns without the "student" alias,
// which a RETURNING clause cannot use.
const studentReturningColumns = `id, created_at, updated_at, tenant_id,
	person_id, school_class, group_id, status,
	enrolled_from, enrolled_until,
	guardian_name, guardian_contact, guardian_email, guardian_phone,
	address_street, address_city, address_postal_code,
	extra_info, supervisor_notes, health_info, pickup_status,
	departure_days, allowed_departure_modes, pickup_days, bus_days,
	departure_companion_note,
	sick, sick_since, excused, excused_since,
	photo_path, photo_consent_given_at, photo_consent_given_by,
	agb_accepted_at, data_processing_accepted_at, email_contact_accepted_at`

// studentWriteRow is the row a write binds. It carries the same columns
// studentRecordRow scans, so a write can return the stored row directly.
type studentWriteRow struct {
	studentRecordRow
}

func newStudentWriteRow(record domain.StudentRecord, tenantID int64) studentWriteRow {
	row := studentWriteRow{}
	row.ID, row.TenantID = record.ID, tenantID
	row.PersonID, row.SchoolClass = record.PersonID, record.SchoolClass
	row.GroupID, row.Status = record.GroupID, record.Status
	row.EnrolledFrom = studentDateParam(record.EnrolledFrom)
	row.EnrolledUntil = studentDateParam(record.EnrolledUntil)
	row.GuardianName, row.GuardianContact = record.GuardianName, record.GuardianContact
	row.GuardianEmail, row.GuardianPhone = record.GuardianEmail, record.GuardianPhone
	row.AddressStreet, row.AddressCity = record.AddressStreet, record.AddressCity
	row.AddressPostalCode = record.AddressPostalCode
	row.ExtraInfo, row.SupervisorNotes = record.ExtraInfo, record.SupervisorNotes
	row.HealthInfo, row.PickupStatus = record.HealthInfo, record.PickupStatus
	row.Sick, row.SickSince = record.Sick, record.SickSince
	row.Excused, row.ExcusedSince = record.Excused, record.ExcusedSince
	row.PhotoPath = record.PhotoPath
	row.PhotoConsentGivenAt, row.PhotoConsentGivenBy = record.PhotoConsentGivenAt, record.PhotoConsentGivenBy
	row.AGBAcceptedAt = record.AGBAcceptedAt
	row.DataProcessingAcceptedAt = record.DataProcessingAcceptedAt
	row.EmailContactAcceptedAt = record.EmailContactAcceptedAt
	return row
}

// studentWritableColumnList is studentWritableColumns as bun's Column() takes it.
func studentWritableColumnList() []string {
	out := make([]string, 0, 28)
	for column := range strings.SplitSeq(studentWritableColumns, ",") {
		if trimmed := strings.TrimSpace(column); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
