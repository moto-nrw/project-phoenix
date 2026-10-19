package test

import (
	"context"
	"testing"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// These writers arrange rows only. Tests that exercise Enrollment construct
// the owner in their own suite or through a typed root.
func insertEnrollmentPhase(tb testing.TB, ctx context.Context, db *bun.DB, phase *enrollmentOwner.Phase) error {
	tb.Helper()
	phase.TenantID = tenant.FromContext(ctx)
	return db.NewRaw(`INSERT INTO enrollment.phases
 (tenant_id, name, kind, service_start_date, service_end_date, care_overflow_mode,
 care_offering_selection_mode, calendar_period_id, is_active)
 VALUES (?, ?, COALESCE(NULLIF(?, ''), 'school_year'), ?, ?, COALESCE(NULLIF(?, ''), 'waitlist'),
 COALESCE(NULLIF(?, ''), 'optional'), ?, ?)
 RETURNING id, created_at, updated_at, kind, care_overflow_mode, care_offering_selection_mode`,
		phase.TenantID, phase.Name, phase.Kind, phase.ServiceStartDate, phase.ServiceEndDate,
		phase.CareOverflowMode, phase.CareOfferingSelectionMode, phase.CalendarPeriodID, phase.IsActive).
		Scan(ctx, &phase.ID, &phase.CreatedAt, &phase.UpdatedAt, &phase.Kind, &phase.CareOverflowMode, &phase.CareOfferingSelectionMode)
}

func insertEnrollmentRequest(tb testing.TB, ctx context.Context, db *bun.DB, request *enrollmentOwner.Request) error {
	tb.Helper()
	request.TenantID = tenant.FromContext(ctx)
	return db.NewRaw(`INSERT INTO enrollment.requests
 (tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, guardian_account_id,
 status_token, submitted_at, submission_source)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), 'public'))
 RETURNING id, created_at, updated_at, submission_source, consent_flags, custom_data, source_metadata, legal_blocks_snapshot`,
		request.TenantID, request.PhaseID, request.GuardianFirstName, request.GuardianLastName, request.GuardianEmail,
		request.GuardianAccountID, request.StatusToken, request.SubmittedAt, request.SubmissionSource).
		Scan(ctx, &request.ID, &request.CreatedAt, &request.UpdatedAt, &request.SubmissionSource,
			&request.ConsentFlags, &request.CustomData, &request.SourceMetadata, &request.LegalBlocksSnapshot)
}

func insertEnrollmentChild(tb testing.TB, ctx context.Context, db *bun.DB, child *enrollmentOwner.RequestChild) error {
	tb.Helper()
	child.TenantID = tenant.FromContext(ctx)
	return db.NewRaw(`INSERT INTO enrollment.request_children
 (tenant_id, request_id, first_name, last_name, date_of_birth, status, activation_mode, created_student_id)
 VALUES (?, ?, ?, ?, ?, COALESCE(NULLIF(?, ''), 'submitted'), COALESCE(NULLIF(?, ''), 'scheduled'), ?)
 RETURNING id, created_at, updated_at, status, activation_mode, custom_data`,
		child.TenantID, child.RequestID, child.FirstName, child.LastName, child.DateOfBirth, child.Status, child.ActivationMode, child.CreatedStudentID).
		Scan(ctx, &child.ID, &child.CreatedAt, &child.UpdatedAt, &child.Status, &child.ActivationMode, &child.CustomData)
}
