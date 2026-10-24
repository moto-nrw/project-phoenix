package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
	"github.com/uptrace/bun"
)

const appointmentTable = `calendar.appointments AS "appointment"`

const (
	recipientTypeStaff    = "staff"
	recipientTypeGuardian = "guardian_profile"
)

type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

func New(database Database) *Store {
	if database == nil {
		panic("appointments postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) databaseForTenant(ctx context.Context, operation string) (bun.IDB, int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	if tenantID <= 0 {
		return nil, 0, fmt.Errorf("appointments postgres: tenant is required to %s", operation)
	}
	return db, tenantID, nil
}

func (s *Store) FindAppointment(ctx context.Context, id int64, lock bool) (*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "find an appointment")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	row := new(appointmentRow)
	query := withTenant(db.NewSelect().Model(row).ModelTableExpr(appointmentTable).Where(`"appointment".id = ?`, id), "appointment", tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	found, stats, err := scanOne(ctx, query, "find appointment")
	if !found {
		return nil, stats, err
	}
	return appointmentToPublic(*row), stats, nil
}

func (s *Store) FindReminderCandidateForUpdate(ctx context.Context, id int64) (*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "lock a reminder candidate")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	row := new(appointmentRow)
	query := db.NewSelect().Model(row).ModelTableExpr(appointmentTable).
		Where(`"appointment".id = ?`, id).
		Where(`"appointment".deleted_at IS NULL`).
		Where(`"appointment".cancelled_at IS NULL`).
		Where(`"appointment".notify_guardians`).
		For("NO KEY UPDATE")
	query = withTenant(query, "appointment", tenantID)
	found, stats, err := scanOne(ctx, query, "lock reminder candidate")
	if !found {
		return nil, stats, err
	}
	return appointmentToPublic(*row), stats, nil
}

func (s *Store) FindReminderCandidatesForUpdate(ctx context.Context, ids []int64) ([]*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "lock reminder candidates")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if len(ids) == 0 {
		return []*appointments.Appointment{}, domain.OperationStats{}, nil
	}
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(`"appointment".id IN (?)`, bun.List(ids)).
		Where(`"appointment".deleted_at IS NULL`).
		Where(`"appointment".cancelled_at IS NULL`).
		Where(`"appointment".notify_guardians`).
		OrderExpr(`"appointment".id`).
		For("NO KEY UPDATE")
	query = withTenant(query, "appointment", tenantID)
	stats, err := scanAll(ctx, query, "lock reminder candidates")
	stats.Rows = int64(len(rows))
	return appointmentsToPublic(rows), stats, err
}

func (s *Store) ListAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to appointments.Date) ([]*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "list staff appointments")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(`("appointment".organizer_staff_id = ? OR EXISTS (
			SELECT 1 FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ? AND ar.staff_id = ?
		))`, staffID, recipientTypeStaff, staffID).
		Where(`"appointment".deleted_at IS NULL`).
		OrderExpr(`"appointment".start_date, "appointment".start_time, "appointment".id`)
	query = withTenant(applyAppointmentWindow(query, from, to), "appointment", tenantID)
	stats, err := scanAll(ctx, query, "list visible staff appointments")
	stats.Rows = int64(len(rows))
	return appointmentsToPublic(rows), stats, err
}

func (s *Store) ListStaffCancellationTombstones(ctx context.Context, staffID int64, since time.Time) ([]*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "list staff cancellation tombstones")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(`(("appointment".deleted_at IS NOT NULL AND "appointment".deleted_at >= ?) OR ("appointment".cancelled_at IS NOT NULL AND "appointment".cancelled_at >= ?))`, since, since).
		Where(`("appointment".organizer_staff_id = ? OR EXISTS (
			SELECT 1 FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ? AND ar.staff_id = ?
		))`, staffID, recipientTypeStaff, staffID).
		OrderExpr(`"appointment".start_date, "appointment".start_time, "appointment".id`)
	query = withTenant(query, "appointment", tenantID)
	stats, err := scanAll(ctx, query, "list staff cancellation tombstones")
	stats.Rows = int64(len(rows))
	return appointmentsToPublic(rows), stats, err
}

func (s *Store) ListAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to appointments.Date) ([]*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "list guardian appointments")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if len(guardianIDs) == 0 || len(studentIDs) == 0 {
		return []*appointments.Appointment{}, domain.OperationStats{}, nil
	}
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(guardianVisibilitySQL, recipientTypeGuardian, bun.List(guardianIDs), bun.List(studentIDs)).
		Where(`"appointment".deleted_at IS NULL`).
		OrderExpr(`"appointment".start_date, "appointment".start_time, "appointment".id`)
	query = withTenant(applyAppointmentWindow(query, from, to), "appointment", tenantID)
	stats, err := scanAll(ctx, query, "list visible guardian appointments")
	stats.Rows = int64(len(rows))
	return appointmentsToPublic(rows), stats, err
}

func (s *Store) ListGuardianCancellationTombstones(ctx context.Context, guardianIDs, studentIDs []int64, since time.Time) ([]*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "list guardian cancellation tombstones")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if len(guardianIDs) == 0 || len(studentIDs) == 0 {
		return []*appointments.Appointment{}, domain.OperationStats{}, nil
	}
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(`(("appointment".deleted_at IS NOT NULL AND "appointment".deleted_at >= ?) OR ("appointment".cancelled_at IS NOT NULL AND "appointment".cancelled_at >= ?))`, since, since).
		Where(guardianVisibilitySQL, recipientTypeGuardian, bun.List(guardianIDs), bun.List(studentIDs)).
		OrderExpr(`"appointment".start_date, "appointment".start_time, "appointment".id`)
	query = withTenant(query, "appointment", tenantID)
	stats, err := scanAll(ctx, query, "list guardian cancellation tombstones")
	stats.Rows = int64(len(rows))
	return appointmentsToPublic(rows), stats, err
}

const guardianVisibilitySQL = `EXISTS (
	SELECT 1 FROM calendar.appointment_recipients ar
	WHERE ar.appointment_id = "appointment".id AND ar.tenant_id = "appointment".tenant_id
	  AND ar.recipient_type = ? AND ar.guardian_profile_id IN (?)
	  AND EXISTS (
		SELECT 1 FROM calendar.appointment_recipient_students ars
		WHERE ars.recipient_id = ar.id AND ars.tenant_id = ar.tenant_id AND ars.student_id IN (?)
	  )
)`

func (s *Store) ListGuardianReminderCandidates(ctx context.Context, from, to appointments.Date) ([]*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "list guardian reminder candidates")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	storageFrom, storageTo := domain.Date(from), domain.Date(to)
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(`"appointment".deleted_at IS NULL`).Where(`"appointment".cancelled_at IS NULL`).
		Where(`"appointment".notify_guardians`).
		Where(`EXISTS (
			SELECT 1 FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ? AND ar.guardian_profile_id IS NOT NULL
		)`, recipientTypeGuardian).
		Where(reminderWindowSQL, storageFrom, storageTo, storageTo, storageFrom, storageFrom, storageFrom, storageTo).
		OrderExpr(`"appointment".start_date, "appointment".start_time, "appointment".id`)
	query = withTenant(query, "appointment", tenantID)
	stats, err := scanAll(ctx, query, "list guardian reminder candidates")
	stats.Rows = int64(len(rows))
	return appointmentsToPublic(rows), stats, err
}

const reminderWindowSQL = `(
	("appointment".end_date >= ? AND "appointment".start_date <= ?)
	OR EXISTS (
		SELECT 1 FROM calendar.recurrence_rules rr
		WHERE rr.appointment_id = "appointment".id AND rr.tenant_id = "appointment".tenant_id
		  AND "appointment".start_date <= ?
		  AND (rr.ends_on IS NULL OR rr.ends_on + ("appointment".end_date - "appointment".start_date) >= ?)
		  AND (rr.occurrence_count IS NULL OR CASE
			WHEN rr.frequency = 'daily' AND rr.occurrence_count::bigint * rr.interval_count <= 1000000 THEN "appointment".start_date + (rr.occurrence_count * rr.interval_count)
			WHEN rr.frequency = 'weekly' AND rr.occurrence_count::bigint * rr.interval_count * 7 <= 1000000 THEN "appointment".start_date + (rr.occurrence_count * rr.interval_count * 7)
			WHEN rr.frequency = 'monthly' AND (rr.occurrence_count::bigint + 1) * rr.interval_count <= 32000 THEN ("appointment".start_date + make_interval(months => (rr.occurrence_count + 1) * rr.interval_count))::date
			WHEN rr.frequency = 'yearly' AND (rr.occurrence_count::bigint + 401) * rr.interval_count <= 100000 THEN ("appointment".start_date + make_interval(years => (rr.occurrence_count + 401) * rr.interval_count))::date
			ELSE 'infinity'::date
		  END + ("appointment".end_date - "appointment".start_date) >= ?)
	)
	OR EXISTS (
		SELECT 1 FROM calendar.appointment_occurrence_overrides aoo
		WHERE aoo.appointment_id = "appointment".id AND aoo.tenant_id = "appointment".tenant_id
		  AND aoo.start_date BETWEEN ? AND ?
	)
)`

func (s *Store) FindAppointmentTargets(ctx context.Context, appointmentID int64) ([]*appointments.AppointmentTarget, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "find appointment targets")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []targetRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`calendar.appointment_targets AS "appointment_target"`).
		Where(`"appointment_target".appointment_id = ?`, appointmentID).
		OrderExpr(`"appointment_target".id`)
	query = withTenant(query, "appointment_target", tenantID)
	stats, err := scanAll(ctx, query, "find appointment targets")
	stats.Rows = int64(len(rows))
	return targetsToPublic(rows), stats, err
}

func (s *Store) FindRecurrenceRule(ctx context.Context, appointmentID int64) (*appointments.RecurrenceRule, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "find a recurrence rule")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	row := new(recurrenceRuleRow)
	query := db.NewSelect().Model(row).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id = ?`, appointmentID).
		Limit(1)
	found, stats, err := scanOne(ctx, withTenant(query, "recurrence_rule", tenantID), "find recurrence rule")
	if !found {
		return nil, stats, err
	}
	return recurrenceRuleToPublic(*row), stats, nil
}

func (s *Store) FindRecurrenceRules(ctx context.Context, appointmentIDs []int64) ([]*appointments.RecurrenceRule, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 {
		return []*appointments.RecurrenceRule{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "find recurrence rules")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []recurrenceRuleRow{}
	query := db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id IN (?)`, bun.List(appointmentIDs))
	stats, err := scanAll(ctx, withTenant(query, "recurrence_rule", tenantID), "find recurrence rules")
	stats.Rows = int64(len(rows))
	return recurrenceRulesToPublic(rows), stats, err
}

func (s *Store) FindOccurrenceOverrides(ctx context.Context, appointmentIDs []int64, dates []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 || len(dates) == 0 {
		return []*appointments.AppointmentOccurrenceOverride{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "find occurrence overrides")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []occurrenceOverrideRow{}
	query := db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".occurrence_date IN (?)`, bun.List(storageDates(dates))).
		OrderExpr(`"appointment_occurrence_override".occurrence_date, "appointment_occurrence_override".id`)
	stats, err := scanAll(ctx, withTenant(query, "appointment_occurrence_override", tenantID), "find occurrence overrides")
	stats.Rows = int64(len(rows))
	return occurrenceOverridesToPublic(rows), stats, err
}

func (s *Store) FindOccurrenceOverridesByStartDates(ctx context.Context, appointmentIDs []int64, dates []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 || len(dates) == 0 {
		return []*appointments.AppointmentOccurrenceOverride{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "find occurrence overrides by start date")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []occurrenceOverrideRow{}
	query := db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".start_date IN (?)`, bun.List(storageDates(dates))).
		OrderExpr(`"appointment_occurrence_override".start_date, "appointment_occurrence_override".id`)
	stats, err := scanAll(ctx, withTenant(query, "appointment_occurrence_override", tenantID), "find occurrence overrides by start date")
	stats.Rows = int64(len(rows))
	return occurrenceOverridesToPublic(rows), stats, err
}

func (s *Store) FindCancelledOccurrenceOverrides(ctx context.Context, appointmentIDs []int64) ([]*appointments.AppointmentOccurrenceOverride, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 {
		return []*appointments.AppointmentOccurrenceOverride{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "find cancelled occurrence overrides")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []occurrenceOverrideRow{}
	query := db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".cancelled`).
		OrderExpr(`"appointment_occurrence_override".occurrence_date, "appointment_occurrence_override".id`)
	stats, err := scanAll(ctx, withTenant(query, "appointment_occurrence_override", tenantID), "find cancelled occurrence overrides")
	stats.Rows = int64(len(rows))
	return occurrenceOverridesToPublic(rows), stats, err
}

func (s *Store) FindAppointmentRecipient(ctx context.Context, recipientID int64) (*appointments.AppointmentRecipient, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "find an appointment recipient")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	row := new(appointmentRecipientRow)
	query := withTenant(db.NewSelect().Model(row).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".id = ?`, recipientID), "appointment_recipient", tenantID)
	found, stats, err := scanOne(ctx, query, "find appointment recipient")
	if !found {
		return nil, stats, err
	}
	return appointmentRecipientToPublic(*row), stats, nil
}

func (s *Store) FindAppointmentRecipients(ctx context.Context, appointmentIDs []int64) ([]*appointments.AppointmentRecipient, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 {
		return []*appointments.AppointmentRecipient{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "find appointment recipients")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]appointmentRecipientRow, 0)
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".appointment_id IN (?)`, bun.List(appointmentIDs)).
		OrderExpr(`"appointment_recipient".appointment_id ASC, "appointment_recipient".recipient_type ASC, "appointment_recipient".id ASC`), "appointment_recipient", tenantID)
	stats, err := scanAll(ctx, query, "find appointment recipients")
	stats.Rows = int64(len(rows))
	return appointmentRecipientsToPublic(rows), stats, err
}

func (s *Store) FindAppointmentRecipientStudents(ctx context.Context, recipientIDs []int64) ([]*appointments.AppointmentRecipientStudent, domain.OperationStats, error) {
	if len(recipientIDs) == 0 {
		return []*appointments.AppointmentRecipientStudent{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "find appointment recipient students")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]appointmentRecipientStudentRow, 0)
	query := withTenant(db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.appointment_recipient_students AS "appointment_recipient_student"`).
		Where(`"appointment_recipient_student".recipient_id IN (?)`, bun.List(recipientIDs)).
		OrderExpr(`"appointment_recipient_student".recipient_id ASC, "appointment_recipient_student".student_id ASC`), "appointment_recipient_student", tenantID)
	stats, err := scanAll(ctx, query, "find appointment recipient students")
	stats.Rows = int64(len(rows))
	return appointmentRecipientStudentsToPublic(rows), stats, err
}

func (s *Store) CountAppointmentRecipientStudents(ctx context.Context, studentID int64) (int, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "count appointment recipient students")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	var count int
	query := withTenant(db.NewSelect().
		TableExpr(`calendar.appointment_recipient_students AS "appointment_recipient_student"`).
		ColumnExpr("COUNT(*)").
		Where(`"appointment_recipient_student".student_id = ?`, studentID), "appointment_recipient_student", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &count)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("appointments postgres: count appointment recipient students: %w", err)
	}
	stats.Rows = 1
	return count, stats, nil
}

func (s *Store) CreateAppointment(ctx context.Context, fields appointments.AppointmentFields) (*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "create an appointment")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	row := appointmentRow{TenantID: tenantID}
	applyAppointmentFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`calendar.appointments`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("appointments postgres: create appointment: %w", err)
	}
	stats.Rows = 1
	return appointmentToPublic(row), stats, nil
}

func (s *Store) InsertAppointmentTargets(ctx context.Context, appointmentID int64, fields []appointments.AppointmentTargetFields) ([]*appointments.AppointmentTarget, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "insert appointment targets")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]targetRow, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, targetRow{TenantID: tenantID, AppointmentID: appointmentID, TargetType: field.TargetType, TargetID: field.TargetID, TargetValue: field.TargetValue})
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&rows).ModelTableExpr(`calendar.appointment_targets`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("appointments postgres: insert appointment targets: %w", err)
	}
	stats.Rows = int64(len(rows))
	return targetsToPublic(rows), stats, nil
}

func (s *Store) UpdateAppointment(ctx context.Context, id int64, fields appointments.AppointmentFields) (*appointments.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "update an appointment")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	row := appointmentRow{ID: id}
	query := db.NewUpdate().TableExpr(appointmentTable).
		Set(`title = ?`, fields.Title).Set(`description = ?`, fields.Description).Set(`location = ?`, fields.Location).
		Set(`start_date = ?`, domain.Date(fields.StartDate)).Set(`end_date = ?`, domain.Date(fields.EndDate)).
		Set(`start_time = ?`, normalizeWallClock(fields.StartTime).Format("15:04:05")).
		Set(`end_time = ?`, normalizeWallClock(fields.EndTime).Format("15:04:05")).
		Set(`all_day = ?`, fields.AllDay).Set(`delivery_mode = ?`, fields.DeliveryMode).
		Set(`overview_visibility = ?`, fields.OverviewVisibility).Set(`notify_guardians = ?`, fields.NotifyGuardians).
		Set(`school_event_type = ?`, fields.SchoolEventType).
		Set(`updated_at = ?`, time.Now()).Set(`revision = revision + 1`).
		Where(`"appointment".id = ?`, id).Where(`"appointment".cancelled_at IS NULL`).Where(`"appointment".deleted_at IS NULL`)
	query = withTenant(query, "appointment", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, stats, appointments.ErrAppointmentLifecycleConflict
	}
	if err != nil {
		return nil, stats, fmt.Errorf("appointments postgres: update appointment: %w", err)
	}
	stats.Rows = 1
	return appointmentToPublic(row), stats, nil
}

func (s *Store) DeleteAppointment(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "delete an appointment")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*appointmentRow)(nil)).ModelTableExpr(appointmentTable).Where(`"appointment".id = ?`, id), "appointment", tenantID)
	return execAny(ctx, query, "delete appointment")
}

func (s *Store) BumpAppointmentRevision(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "bump an appointment revision")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewUpdate().TableExpr(appointmentTable).Set(`revision = revision + 1`).Set(`updated_at = ?`, time.Now()).Where(`"appointment".id = ?`, id)
	return execAny(ctx, withTenant(query, "appointment", tenantID), "bump appointment revision")
}

func (s *Store) CancelAppointment(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "cancel an appointment")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	now := time.Now()
	query := db.NewUpdate().TableExpr(appointmentTable).Set(`cancelled_at = ?`, now).Set(`revision = revision + 1`).Set(`updated_at = ?`, now).
		Where(`"appointment".id = ?`, id).Where(`"appointment".cancelled_at IS NULL`).Where(`"appointment".deleted_at IS NULL`)
	stats, rows, err := execute(ctx, withTenant(query, "appointment", tenantID), "cancel appointment")
	return rows > 0, stats, err
}

func (s *Store) SoftDeleteAppointment(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "soft-delete an appointment")
	if err != nil {
		return domain.OperationStats{}, err
	}
	now := time.Now()
	query := db.NewUpdate().TableExpr(appointmentTable).Set(`deleted_at = ?`, now).Set(`revision = revision + 1`).Set(`updated_at = ?`, now).
		Where(`"appointment".id = ?`, id).Where(`"appointment".deleted_at IS NULL`)
	return execAny(ctx, withTenant(query, "appointment", tenantID), "soft-delete appointment")
}

func (s *Store) DeleteFeedTombstonesBefore(ctx context.Context, before time.Time) (int, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "delete appointment tombstones")
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*appointmentRow)(nil)).ModelTableExpr(appointmentTable).
		Where(`COALESCE("appointment".deleted_at, "appointment".cancelled_at) < ?`, before)
	stats, rows, err := execute(ctx, withTenant(query, "appointment", tenantID), "delete appointment tombstones")
	return int(rows), stats, err
}

func (s *Store) DeleteAppointmentTargets(ctx context.Context, appointmentID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "delete appointment targets")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*targetRow)(nil)).ModelTableExpr(`calendar.appointment_targets AS "appointment_target"`).Where(`"appointment_target".appointment_id = ?`, appointmentID)
	return execAny(ctx, withTenant(query, "appointment_target", tenantID), "delete appointment targets")
}

func (s *Store) CreateRecurrenceRule(ctx context.Context, rule appointments.RecurrenceRule) (*appointments.RecurrenceRule, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "create a recurrence rule")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	row := recurrenceRuleFromPublic(rule)
	row.TenantID = tenantID
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`calendar.recurrence_rules`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("appointments postgres: create recurrence rule: %w", err)
	}
	stats.Rows = 1
	return recurrenceRuleToPublic(row), stats, nil
}

func (s *Store) DeleteRecurrenceRule(ctx context.Context, appointmentID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "delete a recurrence rule")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*recurrenceRuleRow)(nil)).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id = ?`, appointmentID)
	return execAny(ctx, withTenant(query, "recurrence_rule", tenantID), "delete recurrence rule")
}

func (s *Store) CreateOccurrenceOverride(ctx context.Context, override appointments.AppointmentOccurrenceOverride) (*appointments.AppointmentOccurrenceOverride, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "create an occurrence override")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	row := occurrenceOverrideFromPublic(override)
	row.TenantID = tenantID
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`calendar.appointment_occurrence_overrides`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("appointments postgres: create occurrence override: %w", err)
	}
	stats.Rows = 1
	return occurrenceOverrideToPublic(row), stats, nil
}

func (s *Store) DeleteOccurrenceOverrides(ctx context.Context, appointmentID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "delete occurrence overrides")
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*occurrenceOverrideRow)(nil)).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id = ?`, appointmentID)
	return execAny(ctx, withTenant(query, "appointment_occurrence_override", tenantID), "delete occurrence overrides")
}

func (s *Store) CancelOccurrence(ctx context.Context, appointmentID int64, occurrenceDate appointments.Date) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "cancel an occurrence")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	row := &occurrenceOverrideRow{
		TenantID: tenantID, AppointmentID: appointmentID,
		OccurrenceDate: domain.Date(occurrenceDate), Cancelled: true,
	}
	query := db.NewInsert().Model(row).
		ModelTableExpr(`calendar.appointment_occurrence_overrides`).
		On("CONFLICT (tenant_id, appointment_id, occurrence_date) DO UPDATE").
		Set("cancelled = TRUE").
		Set("updated_at = NOW()").
		Where("NOT appointment_occurrence_overrides.cancelled")
	stats, rows, err := execute(ctx, query, "cancel occurrence")
	if err == nil && rows == 0 {
		stats.DuplicatePreventionConflicts = 1
	}
	return rows > 0, stats, err
}

func (s *Store) InsertAppointmentRecipients(ctx context.Context, appointmentID int64, fields []appointments.AppointmentRecipientFields) ([]*appointments.AppointmentRecipient, domain.OperationStats, error) {
	if len(fields) == 0 {
		return []*appointments.AppointmentRecipient{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "create appointment recipients")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]appointmentRecipientRow, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, appointmentRecipientRow{
			TenantID: tenantID, AppointmentID: appointmentID,
			RecipientType: field.RecipientType, StaffID: field.StaffID,
			GuardianProfileID: field.GuardianProfileID, Status: field.Status,
		})
	}
	stats, _, err := execute(ctx, db.NewInsert().Model(&rows).
		ModelTableExpr(`calendar.appointment_recipients`).Returning("*"), "create appointment recipients")
	return appointmentRecipientsToPublic(rows), stats, err
}

func (s *Store) InsertAppointmentRecipientStudents(ctx context.Context, links []appointments.AppointmentRecipientStudent) ([]*appointments.AppointmentRecipientStudent, domain.OperationStats, error) {
	if len(links) == 0 {
		return []*appointments.AppointmentRecipientStudent{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "create appointment recipient students")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]appointmentRecipientStudentRow, 0, len(links))
	for _, link := range links {
		rows = append(rows, appointmentRecipientStudentRow{
			TenantID: tenantID, RecipientID: link.RecipientID, StudentID: link.StudentID,
		})
	}
	stats, _, err := execute(ctx, db.NewInsert().Model(&rows).
		ModelTableExpr(`calendar.appointment_recipient_students`).Returning("*"), "create appointment recipient students")
	return appointmentRecipientStudentsToPublic(rows), stats, err
}

func (s *Store) UpdateAppointmentRecipientResponse(ctx context.Context, recipientID int64, status string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "update an appointment recipient response")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withTenant(db.NewUpdate().Model((*appointmentRecipientRow)(nil)).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Set("status = ?", status).
		Where(`"appointment_recipient".id = ?`, recipientID), "appointment_recipient", tenantID)
	if status == "accepted" || status == "declined" {
		query = query.Set("responded_at = NOW()")
	} else {
		query = query.Set("responded_at = NULL")
	}
	stats, rows, err := execute(ctx, query, "update appointment recipient response")
	return rows == 1, stats, err
}

func (s *Store) ClaimReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, occurrenceDate appointments.Date, guardianProfileID int64) (bool, domain.OperationStats, error) {
	db, _, err := s.databaseForTenant(ctx, "claim a reminder push delivery")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	var claimed bool
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT calendar.claim_appointment_reminder_push_delivery(?, ?, ?, ?)`, appointmentID, revision, domain.Date(occurrenceDate), guardianProfileID).Scan(ctx, &claimed)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("appointments postgres: claim reminder push delivery: %w", err)
	}
	if claimed {
		stats.Rows = 1
	} else {
		stats.DuplicatePreventionConflicts = 1
	}
	return claimed, stats, nil
}

func (s *Store) ReleaseReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, occurrenceDate appointments.Date, guardianProfileID int64) (bool, domain.OperationStats, error) {
	db, _, err := s.databaseForTenant(ctx, "release a reminder push delivery")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	var released bool
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT calendar.release_appointment_reminder_push_delivery(?, ?, ?, ?)`, appointmentID, revision, domain.Date(occurrenceDate), guardianProfileID).Scan(ctx, &released)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("appointments postgres: release reminder push delivery: %w", err)
	}
	if released {
		stats.Rows = 1
	}
	return released, stats, nil
}

func applyAppointmentWindow(query *bun.SelectQuery, from, to appointments.Date) *bun.SelectQuery {
	return query.Where(`(
		("appointment".end_date >= ? AND "appointment".start_date <= ?)
		OR EXISTS (
			SELECT 1 FROM calendar.recurrence_rules rr
			WHERE rr.appointment_id = "appointment".id AND rr.tenant_id = "appointment".tenant_id
			  AND "appointment".start_date <= ?
			  AND (rr.ends_on IS NULL OR rr.ends_on + ("appointment".end_date - "appointment".start_date) >= ?)
		)
	)`, domain.Date(from), domain.Date(to), domain.Date(to), domain.Date(from))
}

func withTenant[Q interface {
	Where(string, ...any) Q
	Err(error) Q
}](query Q, alias string, tenantID int64) Q {
	if tenantID <= 0 {
		return query.Err(errors.New("tenant is required"))
	}
	return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
}

func scanOne(ctx context.Context, query *bun.SelectQuery, operation string) (bool, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return false, stats, nil
	}
	if err != nil {
		return false, stats, fmt.Errorf("appointments postgres: %s: %w", operation, err)
	}
	stats.Rows = 1
	return true, stats, nil
}

func scanAll(ctx context.Context, query *bun.SelectQuery, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("appointments postgres: %s: %w", operation, err)
	}
	return stats, nil
}

type executable interface {
	Exec(context.Context, ...any) (sql.Result, error)
}

func execAny(ctx context.Context, query executable, operation string) (domain.OperationStats, error) {
	stats, _, err := execute(ctx, query, operation)
	return stats, err
}

func execute(ctx context.Context, query executable, operation string) (domain.OperationStats, int64, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, 0, fmt.Errorf("appointments postgres: %s: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, 0, fmt.Errorf("appointments postgres: %s count rows: %w", operation, err)
	}
	stats.Rows = rows
	return stats, rows, nil
}

var _ interface {
	FindAppointment(context.Context, int64, bool) (*appointments.Appointment, domain.OperationStats, error)
} = (*Store)(nil)
