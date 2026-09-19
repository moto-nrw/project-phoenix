package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

func (s *StudentStore) CreateEnrollment(ctx context.Context, input domain.EnrollmentStudent) (domain.Student, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Student{}, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	started := time.Now()
	result, err := s.createEnrollmentStudent(ctx, db, tenantID, input, &stats)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Student{}, stats, fmt.Errorf("people directory postgres: create enrollment student: %w", err)
	}
	return result, stats, nil
}

func (s *StudentStore) createEnrollmentStudent(ctx context.Context, db bun.IDB, tenantID int64, input domain.EnrollmentStudent, stats *domain.OperationStats) (domain.Student, error) {
	var gate *bun.SelectQuery
	var err error
	if s.classGateQuery != nil {
		gate, err = s.classGateQuery(ctx)
	} else {
		err = s.classGate(ctx, false)
		stats.Queries++
		gate = db.NewSelect().ColumnExpr("TRUE")
	}
	if err != nil {
		return domain.Student{}, err
	}
	record := domain.StudentRecord{PersonID: input.PersonID}
	if input.InitialProfile != nil {
		domain.ApplyEnrollmentPatch(&record, *input.InitialProfile)
	}
	row := studentRow{TenantID: tenantID, PersonID: input.PersonID, SchoolClass: input.SchoolClass, Status: input.Status}
	if row.Status == "" {
		row.Status = "active"
	}
	row.EnrolledFrom, err = optionalDate(input.EnrolledFrom)
	if err != nil {
		return domain.Student{}, err
	}
	row.EnrolledUntil, err = optionalDate(input.EnrolledUntil)
	if err != nil {
		return domain.Student{}, err
	}
	stats.Queries++
	err = db.NewRaw(`WITH class_write_gate AS MATERIALIZED (?),
  enrolled_person AS MATERIALIZED (
   SELECT person.id FROM users.persons AS person CROSS JOIN class_write_gate
   WHERE person.id = ? AND person.tenant_id = ? AND person.deleted_at IS NULL
   FOR KEY SHARE OF person
  )
  INSERT INTO users.student_profiles
   (tenant_id, person_id, address_street, address_city, address_postal_code, extra_info,
    photo_consent_given_at, photo_consent_given_by, agb_accepted_at, data_processing_accepted_at, email_contact_accepted_at)
  SELECT ?, id, ?, ?, ?, ?, ?, ?, ?, ?, ? FROM enrolled_person
  RETURNING id, created_at, updated_at`,
		gate, input.PersonID, tenantID, tenantID, record.AddressStreet, record.AddressCity,
		record.AddressPostalCode, record.ExtraInfo, record.PhotoConsentGivenAt, record.PhotoConsentGivenBy,
		record.AGBAcceptedAt, record.DataProcessingAcceptedAt, record.EmailContactAcceptedAt).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Student{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Student{}, err
	}
	stats.Rows = 1
	return toStudent(row), nil
}
