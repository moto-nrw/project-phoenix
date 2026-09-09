package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

// This is the existing class-write advisory lock protocol ("clas") shared
// with grade transitions. It must precede student row locks.
const enrollmentClassWritesLockClass = int32(0x636c6173)

func (s *StudentStore) ApplyEnrollmentProfile(ctx context.Context, id int64, input domain.EnrollmentProfilePatch) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db).Where(`"student".id = ?`, id), tenantID)
	changed := false
	if input.DepartureSet {
		allowed, marshalErr := json.Marshal(input.AllowedDepartureModes)
		if marshalErr != nil {
			return domain.OperationStats{}, marshalErr
		}
		departure, marshalErr := json.Marshal(input.DepartureDays)
		if marshalErr != nil {
			return domain.OperationStats{}, marshalErr
		}
		bus, marshalErr := json.Marshal(input.BusDays)
		if marshalErr != nil {
			return domain.OperationStats{}, marshalErr
		}
		pickup, marshalErr := json.Marshal(input.PickupDays)
		if marshalErr != nil {
			return domain.OperationStats{}, marshalErr
		}
		query = query.Set("allowed_departure_modes = ?::jsonb", string(allowed)).
			Set("departure_days = ?::jsonb", string(departure)).
			Set("bus_days = ?::jsonb", string(bus)).
			Set("pickup_days = ?::jsonb", string(pickup)).
			Set("pickup_status = ?", input.PickupStatus).
			Set("departure_companion_note = ?", input.DepartureCompanionNote)
		changed = true
	}
	if input.HealthInfoSet {
		query = query.Set("health_info = ?", input.HealthInfo)
		changed = true
	}
	if input.ExtraInfoSet {
		query = query.Set("extra_info = ?", input.ExtraInfo)
		changed = true
	}
	if input.PhotoConsentGivenAtSet {
		query = query.Set("photo_consent_given_at = ?", input.PhotoConsentGivenAt)
		changed = true
	}
	if input.PhotoConsentGivenBySet {
		query = query.Set("photo_consent_given_by = ?", input.PhotoConsentGivenBy)
		changed = true
	}
	if input.AGBAcceptedAtSet {
		query = query.Set("agb_accepted_at = ?", input.AGBAcceptedAt)
		changed = true
	}
	if input.DataProcessingAcceptedAtSet {
		query = query.Set("data_processing_accepted_at = ?", input.DataProcessingAcceptedAt)
		changed = true
	}
	if input.EmailContactAcceptedAtSet {
		query = query.Set("email_contact_accepted_at = ?", input.EmailContactAcceptedAt)
		changed = true
	}
	if !changed {
		return domain.OperationStats{}, nil
	}
	affected, stats, err := execStudents(ctx, query, "apply enrollment profile")
	if err == nil && affected == 0 {
		err = domain.ErrStudentNotFound
	}
	return stats, err
}

func lockEnrollmentClassWrites(ctx context.Context, db bun.IDB, tenantID int64) error {
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return err
	}
	if tenantID > 0x7fffffff {
		return fmt.Errorf("lockClassWrites: tenant_id %d exceeds advisory-lock obj id range", tenantID)
	}
	_, err := db.NewRaw("SELECT pg_advisory_xact_lock_shared(?, ?)", enrollmentClassWritesLockClass, int32(tenantID)).Exec(ctx)
	return err
}

func (s *StudentStore) CreateEnrollment(ctx context.Context, input domain.EnrollmentStudent) (domain.Student, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Student{}, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	started := time.Now()
	result, err := createEnrollmentStudent(ctx, db, tenantID, input, &stats)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Student{}, stats, fmt.Errorf("people directory postgres: create enrollment student: %w", err)
	}
	return result, stats, nil
}

type enrollmentStudentRow struct {
	studentRow
	GuardianEmail *string `bun:"guardian_email"`
	GuardianPhone *string `bun:"guardian_phone"`
}

func createEnrollmentStudent(ctx context.Context, db bun.IDB, tenantID int64, input domain.EnrollmentStudent, stats *domain.OperationStats) (domain.Student, error) {
	if err := lockEnrollmentClassWrites(ctx, db, tenantID); err != nil {
		return domain.Student{}, err
	}
	stats.Queries++
	// The foreign key alone does not prove that the person belongs to this
	// school. Lock the same-tenant person before using it as the new identity.
	var personID int64
	stats.Queries++
	err := db.NewSelect().TableExpr(`users.persons AS "person"`).Column("person.id").
		Where("person.id = ?", input.PersonID).Where("person.tenant_id = ?", tenantID).
		Where("person.deleted_at IS NULL").For("KEY SHARE").Scan(ctx, &personID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Student{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Student{}, err
	}
	from, err := optionalDate(input.EnrolledFrom)
	if err != nil {
		return domain.Student{}, err
	}
	until, err := optionalDate(input.EnrolledUntil)
	if err != nil {
		return domain.Student{}, err
	}
	status := input.Status
	if status == "" {
		status = "active"
	}
	row := enrollmentStudentRow{
		studentRow:    studentRow{TenantID: tenantID, PersonID: personID, SchoolClass: input.SchoolClass, Status: status, EnrolledFrom: from, EnrolledUntil: until},
		GuardianEmail: input.GuardianEmail, GuardianPhone: input.GuardianPhone,
	}
	stats.Queries++
	err = db.NewInsert().Model(&row).ModelTableExpr("users.students").
		Column("tenant_id", "person_id", "school_class", "status", "enrolled_from", "enrolled_until", "guardian_email", "guardian_phone").
		Returning("id, created_at, updated_at").Scan(ctx)
	if err != nil {
		return domain.Student{}, err
	}
	stats.Rows = 1
	return toStudent(row.studentRow), nil
}

func (s *StudentStore) RenewEnrollment(ctx context.Context, id int64, input domain.EnrollmentStudent) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if err := lockEnrollmentClassWrites(ctx, db, tenantID); err != nil {
		return domain.OperationStats{}, err
	}
	from, err := optionalDate(input.EnrolledFrom)
	if err != nil {
		return domain.OperationStats{}, err
	}
	until, err := optionalDate(input.EnrolledUntil)
	if err != nil {
		return domain.OperationStats{}, err
	}
	// Never rewrite departure plans from a stale hydrated student. This command
	// changes only the fields controlled by an enrollment renewal.
	query := withStudentTenant(studentUpdate(db).
		Set("school_class = ?", input.SchoolClass).Set("status = ?", input.Status).
		Set("enrolled_from = ?", from).Set("enrolled_until = ?", until).
		Set("guardian_email = ?", input.GuardianEmail).Set("guardian_phone = ?", input.GuardianPhone).
		Where(`"student".id = ?`, id), tenantID)
	affected, stats, err := execStudents(ctx, query, "renew enrollment student")
	stats.Queries++
	if err == nil && affected == 0 {
		err = domain.ErrStudentNotFound
	}
	return stats, err
}
