package users

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StudentDeletionRepository owns the deliberately cross-schema preview for a
// permanent child deletion. Keeping the counts together prevents the UI
// preview and the transactional stale-preview check from drifting apart.
type StudentDeletionRepository struct {
	db                   *bun.DB
	countAuditReferences func(context.Context, int64) (int, error)
	// countConsents is served by the privacy-consent owner (student-presence,
	// #2662); the preview must not join users.privacy_consents itself.
	countConsents func(context.Context, int64) (int, error)
	carePlan      interface {
		CountCompanionLinks(context.Context, int64) (int, error)
		CountStudentScheduleRows(context.Context, int64) (int, error)
	}
	appointments interface {
		CountAppointmentRecipientStudents(context.Context, int64) (int, error)
	}
}

func (r *StudentDeletionRepository) BindCarePlan(capability interface {
	CountCompanionLinks(context.Context, int64) (int, error)
	CountStudentScheduleRows(context.Context, int64) (int, error)
}) {
	if capability == nil {
		panic("student deletion repository: care plan capability is required")
	}
	r.carePlan = capability
}

func (r *StudentDeletionRepository) BindAppointments(capability interface {
	CountAppointmentRecipientStudents(context.Context, int64) (int, error)
}) {
	if capability == nil {
		panic("student deletion repository: appointments capability is required")
	}
	r.appointments = capability
}

func NewStudentDeletionRepository(
	db *bun.DB,
	countAuditReferences func(context.Context, int64) (int, error),
	countConsents func(context.Context, int64) (int, error),
) userModels.StudentDeletionRepository {
	return &StudentDeletionRepository{db: db, countAuditReferences: countAuditReferences, countConsents: countConsents}
}

func (r *StudentDeletionRepository) Preview(ctx context.Context, studentID int64) (*userModels.StudentDeletionCounts, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, fmt.Errorf("preview student deletion: tenant context is required")
	}

	counts := new(userModels.StudentDeletionCounts)
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT
			(SELECT COUNT(*) FROM schedule.instance_students WHERE tenant_id = ? AND student_id = ?)::int AS timetable_assignments,
			(SELECT COUNT(*) FROM activities.student_enrollments WHERE tenant_id = ? AND student_id = ?)::int AS activity_enrollments,
			(
				active.count_student_visits_for_deletion(?, ?) +
				(SELECT COUNT(*) FROM active.attendance WHERE tenant_id = ? AND student_id = ?) +
				(SELECT COUNT(*) FROM active.student_status_days WHERE tenant_id = ? AND student_id = ?) +
				(SELECT COUNT(*) FROM active.scheduled_checkouts WHERE tenant_id = ? AND student_id = ?) +
				(SELECT COUNT(*) FROM active.excused_absence_requests WHERE tenant_id = ? AND student_id = ?)
			)::int AS attendance_records,
			0::int AS care_schedules,
			(
				(SELECT COUNT(*) FROM users.students_guardians WHERE tenant_id = ? AND student_id = ?) +
				(SELECT COUNT(*) FROM users.persons_guardians WHERE tenant_id = ? AND person_id = (SELECT person_id FROM users.students WHERE tenant_id = ? AND id = ?))
			)::int AS guardian_links,
			0::int AS companion_links,
			(
				(SELECT COUNT(*) FROM users.parent_message_threads WHERE tenant_id = ? AND student_id = ?) +
				(SELECT COUNT(*) FROM users.parent_messages WHERE tenant_id = ? AND student_id = ?) +
				(
					SELECT COUNT(*)
					FROM users.parent_message_reads AS "parent_message_read"
					JOIN users.parent_message_threads AS "parent_message_thread"
						ON "parent_message_thread".id = "parent_message_read".thread_id
						AND "parent_message_thread".tenant_id = "parent_message_read".tenant_id
					WHERE "parent_message_read".tenant_id = ? AND "parent_message_thread".student_id = ?
				) +
				(SELECT COUNT(*) FROM users.student_data_change_requests WHERE tenant_id = ? AND student_id = ?) +
				(SELECT COUNT(*) FROM schedule.care_schedule_change_requests WHERE tenant_id = ? AND student_id = ?) +
				(SELECT COUNT(*) FROM auth.guardian_invitations WHERE tenant_id = ? AND student_id = ?)
			)::int AS communications,
			(SELECT COUNT(*) FROM enrollment.request_children WHERE tenant_id = ? AND (created_student_id = ? OR matched_student_id = ?))::int AS enrollment_references,
			(
				(SELECT COUNT(*) FROM schedule.grade_transition_roster_removals WHERE tenant_id = ? AND student_id = ?) +
				(SELECT COUNT(*) FROM education.grade_transition_history WHERE tenant_id = ? AND student_id = ? AND person_name <> 'Gelöschtes Kind')
			)::int AS other_records
	`,
		tenantID, studentID,
		tenantID, studentID,
		tenantID, studentID, tenantID, studentID, tenantID, studentID, tenantID, studentID, tenantID, studentID,
		tenantID, studentID, tenantID, tenantID, studentID,
		tenantID, studentID, tenantID, studentID, tenantID, studentID, tenantID, studentID, tenantID, studentID, tenantID, studentID,
		tenantID, studentID, studentID,
		tenantID, studentID, tenantID, studentID,
	).Scan(ctx, counts)
	if err != nil {
		return nil, fmt.Errorf("preview student deletion: %w", err)
	}
	if r.carePlan == nil {
		return nil, fmt.Errorf("preview student deletion: care plan capability is required")
	}
	counts.CareSchedules, err = r.carePlan.CountStudentScheduleRows(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("preview student deletion: count care schedules: %w", err)
	}
	counts.CompanionLinks, err = r.carePlan.CountCompanionLinks(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("preview student deletion: count companion links: %w", err)
	}
	if r.appointments == nil {
		return nil, fmt.Errorf("preview student deletion: appointments capability is required")
	}
	appointmentRecipients, err := r.appointments.CountAppointmentRecipientStudents(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("preview student deletion: count appointment recipient students: %w", err)
	}
	counts.OtherRecords += appointmentRecipients
	if r.countAuditReferences == nil || r.countConsents == nil {
		return nil, fmt.Errorf("preview student deletion: audit and consent count capabilities are required")
	}
	counts.Consents, err = r.countConsents(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("preview student deletion: count consents: %w", err)
	}
	auditReferences, err := r.countAuditReferences(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("preview student deletion: count audit references: %w", err)
	}
	counts.OtherRecords += auditReferences
	return counts, nil
}

// LockMessageThreads prevents a read cursor or message from being added after
// the deletion preview is rechecked. Both rows reference their thread, so the
// FOR UPDATE lock serializes their FK checks until the deletion commits or
// rolls back.
func (r *StudentDeletionRepository) LockMessageThreads(ctx context.Context, studentID int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return fmt.Errorf("lock student message threads: tenant context is required")
	}

	var threadIDs []int64
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.parent_message_threads AS "parent_message_thread"`).
		ColumnExpr(`"parent_message_thread".id`).
		Where(`"parent_message_thread".tenant_id = ?`, tenantID).
		Where(`"parent_message_thread".student_id = ?`, studentID).
		OrderExpr(`"parent_message_thread".id ASC`).
		For("UPDATE").
		Scan(ctx, &threadIDs)
	if err != nil {
		return fmt.Errorf("lock student message threads: %w", err)
	}
	return nil
}

// DeleteLegacyGuardianLinks removes relationships from the superseded
// person-based guardian junction. Current guardian links cascade from the
// student row; these do not because the anonymized person tombstone remains.
func (r *StudentDeletionRepository) DeleteLegacyGuardianLinks(ctx context.Context, personID int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return fmt.Errorf("delete legacy student guardian links: tenant context is required")
	}
	_, err := base.GetDB(ctx, r.db).NewDelete().
		TableExpr(`users.persons_guardians AS "person_guardian"`).
		Where(`"person_guardian".tenant_id = ?`, tenantID).
		Where(`"person_guardian".person_id = ?`, personID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete legacy student guardian links: %w", err)
	}
	return nil
}

// AnonymizePersonIfUnchanged removes the child's remaining direct identifiers
// and creates the soft-deleted person tombstone only if the previewed person
// row is still current. The updated_at predicate closes the race between name
// confirmation and a concurrent person edit.
func (r *StudentDeletionRepository) AnonymizePersonIfUnchanged(
	ctx context.Context,
	personID int64,
	updatedAt time.Time,
) (bool, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return false, fmt.Errorf("anonymize deleted student person: tenant context is required")
	}
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		TableExpr(`users.persons AS "person"`).
		Set(`first_name = ?`, "Gelöscht").
		Set(`last_name = ?`, "Benutzer").
		Set(`birthday = NULL`).
		Set(`tag_id = NULL`).
		Set(`account_id = NULL`).
		Set(`deleted_at = NOW()`).
		Where(`"person".tenant_id = ?`, tenantID).
		Where(`"person".id = ?`, personID).
		Where(`"person".updated_at = ?`, updatedAt).
		Where(`"person".deleted_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("anonymize deleted student person: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count anonymized deleted student persons: %w", err)
	}
	return rows == 1, nil
}

// DeleteTimetableAssignments removes only the child rows from shared
// activity instances. The instance itself and every other child's assignment
// remain untouched.
func (r *StudentDeletionRepository) DeleteTimetableAssignments(ctx context.Context, studentID int64) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, fmt.Errorf("delete student timetable assignments: tenant context is required")
	}
	result, err := base.GetDB(ctx, r.db).NewDelete().
		TableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".tenant_id = ?`, tenantID).
		Where(`"instance_student".student_id = ?`, studentID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete student timetable assignments: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted student timetable assignments: %w", err)
	}
	return rows, nil
}
