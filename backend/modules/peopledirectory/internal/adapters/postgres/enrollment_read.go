package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
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
	GuardianName             *string             `bun:"guardian_name"`
	GuardianContact          *string             `bun:"guardian_contact"`
	GuardianEmail            *string             `bun:"guardian_email"`
	GuardianPhone            *string             `bun:"guardian_phone"`
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

func (s *StudentStore) LockEnrollmentClassWrites(ctx context.Context) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	err = lockEnrollmentClassWrites(ctx, db, tenantID)
	return domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}, err
}

func (s *StudentStore) ReadEnrollment(ctx context.Context, id int64, lock string) (domain.EnrollmentRecord, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.EnrollmentRecord{}, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return domain.EnrollmentRecord{}, domain.OperationStats{}, err
	}
	started := time.Now()
	stats := domain.OperationStats{}
	if lock != "" {
		if err := lockEnrollmentClassWrites(ctx, db, tenantID); err != nil {
			return domain.EnrollmentRecord{}, stats, err
		}
		stats.Queries++
	}
	row := enrollmentReadRow{}
	query := db.NewSelect().TableExpr("users.students AS student").
		Column("student.id").
		Column("student.created_at").
		Column("student.updated_at").
		Column("student.tenant_id").
		Column("student.person_id").
		Column("student.school_class").
		Column("student.status").
		ColumnExpr("COALESCE(to_char(student.enrolled_from, 'YYYY-MM-DD'), '') AS enrolled_from").
		ColumnExpr("COALESCE(to_char(student.enrolled_until, 'YYYY-MM-DD'), '') AS enrolled_until").
		Column("student.guardian_name").
		Column("student.guardian_contact").
		Column("student.guardian_email").
		Column("student.guardian_phone").
		Column("student.address_street").
		Column("student.address_city").
		Column("student.address_postal_code").
		Column("student.extra_info").
		Column("student.supervisor_notes").
		Column("student.health_info").
		Column("student.pickup_status").
		Column("student.departure_companion_note").
		Column("student.photo_path").
		Column("student.group_id").
		Column("student.sick").
		Column("student.excused").
		Column("student.sick_since").
		Column("student.excused_since").
		Column("student.photo_consent_given_at").
		Column("student.agb_accepted_at").
		Column("student.data_processing_accepted_at").
		Column("student.email_contact_accepted_at").
		Column("student.photo_consent_given_by").
		Column("student.allowed_departure_modes").
		Column("student.departure_days").
		Column("student.bus_days").
		Column("student.pickup_days").
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
	if err != nil {
		return domain.EnrollmentRecord{}, stats, fmt.Errorf("people directory postgres: read enrollment student: %w", err)
	}
	return domain.EnrollmentRecord(row), stats, nil
}
