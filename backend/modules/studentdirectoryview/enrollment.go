package studentdirectoryview

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	domain "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// This row is private to the adapter. The public snapshot has no ORM tags.
type enrollmentReadRow struct {
	ID                       int64               `bun:"id"`
	CreatedAt                time.Time           `bun:"created_at"`
	UpdatedAt                time.Time           `bun:"updated_at"`
	TenantID                 int64               `bun:"tenant_id"`
	PersonID                 int64               `bun:"person_id"`
	SchoolClass              string              `bun:"school_class"`
	Status                   string              `bun:"status"`
	EnrolledFrom             string              `bun:"enrolled_from"`
	EnrolledUntil            string              `bun:"enrolled_until"`
	AddressStreet            *string             `bun:"address_street"`
	AddressCity              *string             `bun:"address_city"`
	AddressPostalCode        *string             `bun:"address_postal_code"`
	ExtraInfo                *string             `bun:"extra_info"`
	SupervisorNotes          *string             `bun:"supervisor_notes"`
	HealthInfo               *string             `bun:"health_info"`
	PickupStatus             *string             `bun:"pickup_status"`
	DepartureCompanionNote   *string             `bun:"departure_companion_note"`
	PhotoPath                *string             `bun:"photo_path"`
	GroupID                  *int64              `bun:"group_id"`
	Sick                     *bool               `bun:"sick"`
	Excused                  *bool               `bun:"excused"`
	SickSince                *time.Time          `bun:"sick_since"`
	ExcusedSince             *time.Time          `bun:"excused_since"`
	PhotoConsentGivenAt      *time.Time          `bun:"photo_consent_given_at"`
	AGBAcceptedAt            *time.Time          `bun:"agb_accepted_at"`
	DataProcessingAcceptedAt *time.Time          `bun:"data_processing_accepted_at"`
	EmailContactAcceptedAt   *time.Time          `bun:"email_contact_accepted_at"`
	PhotoConsentGivenBy      *int64              `bun:"photo_consent_given_by"`
	AllowedDepartureModes    map[string][]string `bun:"allowed_departure_modes,type:jsonb"`
	DepartureDays            map[string]string   `bun:"departure_days,type:jsonb"`
	BusDays                  map[string]bool     `bun:"bus_days,type:jsonb"`
	PickupDays               map[string]bool     `bun:"pickup_days,type:jsonb"`
}

func (s *Projection) ReadEnrollment(ctx context.Context, id int64, lock string) (domain.EnrollmentRecord, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.EnrollmentRecord{}, OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.EnrollmentRecord{}, OperationStats{}, errors.New("student directory: tenant is required")
	}
	started := time.Now()
	stats := OperationStats{}

	row := enrollmentReadRow{}
	query := db.NewSelect().TableExpr(studentSource).
		ColumnExpr(enrollmentColumns).
		ColumnExpr("COALESCE(to_char(student.enrolled_from, 'YYYY-MM-DD'), '') AS enrolled_from").
		ColumnExpr("COALESCE(to_char(student.enrolled_until, 'YYYY-MM-DD'), '') AS enrolled_until").
		Where("student.id = ?", id).Where("student.tenant_id = ?", tenantID)
	switch lock {
	case "update":
		query = query.For("UPDATE")
	case "nowait":
		query = query.For("UPDATE NOWAIT")
	case "":
	default:
		return domain.EnrollmentRecord{}, stats, fmt.Errorf("invalid enrollment lock mode %q", lock)
	}
	stats.Queries++
	err = query.Scan(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.EnrollmentRecord{}, stats, domain.ErrStudentNotFound
	}
	if isLockNotAvailable(err) {
		return domain.EnrollmentRecord{}, stats, fmt.Errorf("%w: %w", domain.ErrStudentLockBusy, err)
	}
	if err != nil {
		return domain.EnrollmentRecord{}, stats, fmt.Errorf("people directory postgres: read enrollment student: %w", err)
	}
	return domain.EnrollmentRecord(row), stats, nil
}

const enrollmentColumns = `"student".id, "student".created_at, "student".updated_at, "student".tenant_id, "student".person_id, "student".school_class, "student".status, "student".address_street, "student".address_city, "student".address_postal_code, "student".extra_info, "student".supervisor_notes, "student".health_info, "student".pickup_status, "student".departure_companion_note, "student".photo_path, "student".group_id, "student".sick, "student".excused, "student".sick_since, "student".excused_since, "student".photo_consent_given_at, "student".agb_accepted_at, "student".data_processing_accepted_at, "student".email_contact_accepted_at, "student".photo_consent_given_by, "student".allowed_departure_modes, "student".departure_days, "student".bus_days, "student".pickup_days`
