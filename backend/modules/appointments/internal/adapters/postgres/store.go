package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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

type appointmentRow struct {
	bun.BaseModel      `bun:"table:calendar.appointments,alias:appointment"`
	ID                 int64       `bun:"id,pk,autoincrement"`
	TenantID           int64       `bun:"tenant_id,notnull"`
	CreatedAt          time.Time   `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time   `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	OrganizerStaffID   int64       `bun:"organizer_staff_id,notnull"`
	Title              string      `bun:"title,notnull"`
	Description        *string     `bun:"description"`
	Location           *string     `bun:"location"`
	StartDate          domain.Date `bun:"start_date,notnull,type:date"`
	EndDate            domain.Date `bun:"end_date,notnull,type:date"`
	StartTime          time.Time   `bun:"start_time,notnull"`
	EndTime            time.Time   `bun:"end_time,notnull"`
	AllDay             bool        `bun:"all_day,notnull"`
	DeliveryMode       string      `bun:"delivery_mode,notnull"`
	OverviewVisibility string      `bun:"overview_visibility,notnull"`
	CancelledAt        *time.Time  `bun:"cancelled_at"`
	DeletedAt          *time.Time  `bun:"deleted_at"`
	NotifyGuardians    bool        `bun:"notify_guardians,notnull"`
	Revision           int         `bun:"revision,notnull"`
}

type targetRow struct {
	bun.BaseModel `bun:"table:calendar.appointment_targets,alias:appointment_target"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	AppointmentID int64     `bun:"appointment_id,notnull"`
	TargetType    string    `bun:"target_type,notnull"`
	TargetID      *int64    `bun:"target_id"`
	TargetValue   *string   `bun:"target_value"`
}

type recurrenceRuleRow struct {
	bun.BaseModel   `bun:"table:calendar.recurrence_rules,alias:recurrence_rule"`
	ID              int64        `bun:"id,pk,autoincrement"`
	TenantID        int64        `bun:"tenant_id,notnull"`
	CreatedAt       time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	AppointmentID   int64        `bun:"appointment_id,notnull"`
	Frequency       string       `bun:"frequency,notnull"`
	IntervalCount   int          `bun:"interval_count,notnull"`
	Weekdays        []string     `bun:"weekdays,array"`
	MonthDays       []int        `bun:"month_days,array"`
	EndsOn          *domain.Date `bun:"ends_on,type:date"`
	OccurrenceCount *int         `bun:"occurrence_count"`
}

type occurrenceOverrideRow struct {
	bun.BaseModel  `bun:"table:calendar.appointment_occurrence_overrides,alias:appointment_occurrence_override"`
	ID             int64        `bun:"id,pk,autoincrement"`
	TenantID       int64        `bun:"tenant_id,notnull"`
	CreatedAt      time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	AppointmentID  int64        `bun:"appointment_id,notnull"`
	OccurrenceDate domain.Date  `bun:"occurrence_date,notnull,type:date"`
	Cancelled      bool         `bun:"cancelled,notnull"`
	Title          *string      `bun:"title"`
	Description    *string      `bun:"description"`
	Location       *string      `bun:"location"`
	StartDate      *domain.Date `bun:"start_date,type:date"`
	EndDate        *domain.Date `bun:"end_date,type:date"`
	StartTime      *time.Time   `bun:"start_time"`
	EndTime        *time.Time   `bun:"end_time"`
	AllDay         *bool        `bun:"all_day"`
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

func (s *Store) FindAppointment(ctx context.Context, id int64, lock bool) (domain.Appointment, bool, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "find an appointment")
	if err != nil {
		return domain.Appointment{}, false, domain.OperationStats{}, err
	}
	row := new(appointmentRow)
	query := withTenant(db.NewSelect().Model(row).ModelTableExpr(appointmentTable).Where(`"appointment".id = ?`, id), "appointment", tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	found, stats, err := scanOne(ctx, query, "find appointment")
	return appointmentToDomain(*row), found, stats, err
}

func (s *Store) FindReminderCandidateForUpdate(ctx context.Context, id int64) (domain.Appointment, bool, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "lock a reminder candidate")
	if err != nil {
		return domain.Appointment{}, false, domain.OperationStats{}, err
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
	return appointmentToDomain(*row), found, stats, err
}

func (s *Store) ListAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to domain.Date) ([]domain.Appointment, domain.OperationStats, error) {
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
	return appointmentsToDomain(rows), stats, err
}

func (s *Store) ListStaffCancellationTombstones(ctx context.Context, staffID int64, since time.Time) ([]domain.Appointment, domain.OperationStats, error) {
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
	return appointmentsToDomain(rows), stats, err
}

func (s *Store) ListAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to domain.Date) ([]domain.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "list guardian appointments")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if len(guardianIDs) == 0 || len(studentIDs) == 0 {
		return []domain.Appointment{}, domain.OperationStats{}, nil
	}
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(guardianVisibilitySQL, recipientTypeGuardian, bun.List(guardianIDs), bun.List(studentIDs)).
		Where(`"appointment".deleted_at IS NULL`).
		OrderExpr(`"appointment".start_date, "appointment".start_time, "appointment".id`)
	query = withTenant(applyAppointmentWindow(query, from, to), "appointment", tenantID)
	stats, err := scanAll(ctx, query, "list visible guardian appointments")
	stats.Rows = int64(len(rows))
	return appointmentsToDomain(rows), stats, err
}

func (s *Store) ListGuardianCancellationTombstones(ctx context.Context, guardianIDs, studentIDs []int64, since time.Time) ([]domain.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "list guardian cancellation tombstones")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if len(guardianIDs) == 0 || len(studentIDs) == 0 {
		return []domain.Appointment{}, domain.OperationStats{}, nil
	}
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(`(("appointment".deleted_at IS NOT NULL AND "appointment".deleted_at >= ?) OR ("appointment".cancelled_at IS NOT NULL AND "appointment".cancelled_at >= ?))`, since, since).
		Where(guardianVisibilitySQL, recipientTypeGuardian, bun.List(guardianIDs), bun.List(studentIDs)).
		OrderExpr(`"appointment".start_date, "appointment".start_time, "appointment".id`)
	query = withTenant(query, "appointment", tenantID)
	stats, err := scanAll(ctx, query, "list guardian cancellation tombstones")
	stats.Rows = int64(len(rows))
	return appointmentsToDomain(rows), stats, err
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

func (s *Store) ListGuardianReminderCandidates(ctx context.Context, from, to domain.Date) ([]domain.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "list guardian reminder candidates")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []appointmentRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(appointmentTable).
		Where(`"appointment".deleted_at IS NULL`).Where(`"appointment".cancelled_at IS NULL`).
		Where(`"appointment".notify_guardians`).
		Where(`EXISTS (
			SELECT 1 FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ? AND ar.guardian_profile_id IS NOT NULL
		)`, recipientTypeGuardian).
		Where(reminderWindowSQL, from, to, to, from, from, from, to).
		OrderExpr(`"appointment".start_date, "appointment".start_time, "appointment".id`)
	query = withTenant(query, "appointment", tenantID)
	stats, err := scanAll(ctx, query, "list guardian reminder candidates")
	stats.Rows = int64(len(rows))
	return appointmentsToDomain(rows), stats, err
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

func (s *Store) FindAppointmentTargets(ctx context.Context, appointmentID int64) ([]domain.AppointmentTarget, domain.OperationStats, error) {
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
	return targetsToDomain(rows), stats, err
}

func (s *Store) FindRecurrenceRule(ctx context.Context, appointmentID int64) (domain.RecurrenceRule, bool, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "find a recurrence rule")
	if err != nil {
		return domain.RecurrenceRule{}, false, domain.OperationStats{}, err
	}
	row := new(recurrenceRuleRow)
	query := db.NewSelect().Model(row).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id = ?`, appointmentID).
		Limit(1)
	found, stats, err := scanOne(ctx, withTenant(query, "recurrence_rule", tenantID), "find recurrence rule")
	return recurrenceRuleToDomain(*row), found, stats, err
}

func (s *Store) FindRecurrenceRules(ctx context.Context, appointmentIDs []int64) ([]domain.RecurrenceRule, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 {
		return []domain.RecurrenceRule{}, domain.OperationStats{}, nil
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
	return recurrenceRulesToDomain(rows), stats, err
}

func (s *Store) FindOccurrenceOverrides(ctx context.Context, appointmentIDs []int64, dates []domain.Date) ([]domain.AppointmentOccurrenceOverride, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 || len(dates) == 0 {
		return []domain.AppointmentOccurrenceOverride{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "find occurrence overrides")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []occurrenceOverrideRow{}
	query := db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".occurrence_date IN (?)`, bun.List(dates)).
		OrderExpr(`"appointment_occurrence_override".occurrence_date, "appointment_occurrence_override".id`)
	stats, err := scanAll(ctx, withTenant(query, "appointment_occurrence_override", tenantID), "find occurrence overrides")
	stats.Rows = int64(len(rows))
	return occurrenceOverridesToDomain(rows), stats, err
}

func (s *Store) FindOccurrenceOverridesByStartDates(ctx context.Context, appointmentIDs []int64, dates []domain.Date) ([]domain.AppointmentOccurrenceOverride, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 || len(dates) == 0 {
		return []domain.AppointmentOccurrenceOverride{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForTenant(ctx, "find occurrence overrides by start date")
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []occurrenceOverrideRow{}
	query := db.NewSelect().Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".start_date IN (?)`, bun.List(dates)).
		OrderExpr(`"appointment_occurrence_override".start_date, "appointment_occurrence_override".id`)
	stats, err := scanAll(ctx, withTenant(query, "appointment_occurrence_override", tenantID), "find occurrence overrides by start date")
	stats.Rows = int64(len(rows))
	return occurrenceOverridesToDomain(rows), stats, err
}

func (s *Store) FindCancelledOccurrenceOverrides(ctx context.Context, appointmentIDs []int64) ([]domain.AppointmentOccurrenceOverride, domain.OperationStats, error) {
	if len(appointmentIDs) == 0 {
		return []domain.AppointmentOccurrenceOverride{}, domain.OperationStats{}, nil
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
	return occurrenceOverridesToDomain(rows), stats, err
}

func (s *Store) CreateAppointment(ctx context.Context, fields domain.AppointmentFields) (domain.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "create an appointment")
	if err != nil {
		return domain.Appointment{}, domain.OperationStats{}, err
	}
	row := appointmentRow{TenantID: tenantID}
	applyAppointmentFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`calendar.appointments`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Appointment{}, stats, fmt.Errorf("appointments postgres: create appointment: %w", err)
	}
	stats.Rows = 1
	return appointmentToDomain(row), stats, nil
}

func (s *Store) InsertAppointmentTargets(ctx context.Context, appointmentID int64, fields []domain.AppointmentTargetFields) ([]domain.AppointmentTarget, domain.OperationStats, error) {
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
	return targetsToDomain(rows), stats, nil
}

func (s *Store) UpdateAppointment(ctx context.Context, id int64, fields domain.AppointmentFields) (domain.Appointment, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "update an appointment")
	if err != nil {
		return domain.Appointment{}, domain.OperationStats{}, err
	}
	row := appointmentRow{ID: id}
	query := db.NewUpdate().TableExpr(appointmentTable).
		Set(`title = ?`, fields.Title).Set(`description = ?`, fields.Description).Set(`location = ?`, fields.Location).
		Set(`start_date = ?`, fields.StartDate).Set(`end_date = ?`, fields.EndDate).
		Set(`start_time = ?`, normalizeWallClock(fields.StartTime).Format("15:04:05")).
		Set(`end_time = ?`, normalizeWallClock(fields.EndTime).Format("15:04:05")).
		Set(`all_day = ?`, fields.AllDay).Set(`delivery_mode = ?`, fields.DeliveryMode).
		Set(`overview_visibility = ?`, fields.OverviewVisibility).Set(`notify_guardians = ?`, fields.NotifyGuardians).
		Set(`updated_at = ?`, time.Now()).Set(`revision = revision + 1`).
		Where(`"appointment".id = ?`, id).Where(`"appointment".cancelled_at IS NULL`).Where(`"appointment".deleted_at IS NULL`)
	query = withTenant(query, "appointment", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning("*").Scan(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Appointment{}, stats, domain.ErrAppointmentLifecycleConflict
	}
	if err != nil {
		return domain.Appointment{}, stats, fmt.Errorf("appointments postgres: update appointment: %w", err)
	}
	stats.Rows = 1
	return appointmentToDomain(row), stats, nil
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

func (s *Store) CreateRecurrenceRule(ctx context.Context, rule domain.RecurrenceRule) (domain.RecurrenceRule, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "create a recurrence rule")
	if err != nil {
		return domain.RecurrenceRule{}, domain.OperationStats{}, err
	}
	row := recurrenceRuleFromDomain(rule)
	row.TenantID = tenantID
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`calendar.recurrence_rules`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.RecurrenceRule{}, stats, fmt.Errorf("appointments postgres: create recurrence rule: %w", err)
	}
	stats.Rows = 1
	return recurrenceRuleToDomain(row), stats, nil
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

func (s *Store) CreateOccurrenceOverride(ctx context.Context, override domain.AppointmentOccurrenceOverride) (domain.AppointmentOccurrenceOverride, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "create an occurrence override")
	if err != nil {
		return domain.AppointmentOccurrenceOverride{}, domain.OperationStats{}, err
	}
	row := occurrenceOverrideFromDomain(override)
	row.TenantID = tenantID
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`calendar.appointment_occurrence_overrides`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.AppointmentOccurrenceOverride{}, stats, fmt.Errorf("appointments postgres: create occurrence override: %w", err)
	}
	stats.Rows = 1
	return occurrenceOverrideToDomain(row), stats, nil
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

func (s *Store) CancelOccurrence(ctx context.Context, appointmentID int64, occurrenceDate domain.Date) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.databaseForTenant(ctx, "cancel an occurrence")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	row := &occurrenceOverrideRow{
		TenantID: tenantID, AppointmentID: appointmentID,
		OccurrenceDate: occurrenceDate, Cancelled: true,
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

func applyAppointmentWindow(query *bun.SelectQuery, from, to domain.Date) *bun.SelectQuery {
	return query.Where(`(
		("appointment".end_date >= ? AND "appointment".start_date <= ?)
		OR EXISTS (
			SELECT 1 FROM calendar.recurrence_rules rr
			WHERE rr.appointment_id = "appointment".id AND rr.tenant_id = "appointment".tenant_id
			  AND "appointment".start_date <= ?
			  AND (rr.ends_on IS NULL OR rr.ends_on + ("appointment".end_date - "appointment".start_date) >= ?)
		)
	)`, from, to, to, from)
}

func applyAppointmentFields(row *appointmentRow, fields domain.AppointmentFields) {
	row.OrganizerStaffID = fields.OrganizerStaffID
	row.Title = fields.Title
	row.Description = fields.Description
	row.Location = fields.Location
	row.StartDate = fields.StartDate
	row.EndDate = fields.EndDate
	row.StartTime = fields.StartTime
	row.EndTime = fields.EndTime
	row.AllDay = fields.AllDay
	row.DeliveryMode = fields.DeliveryMode
	row.OverviewVisibility = fields.OverviewVisibility
	row.NotifyGuardians = fields.NotifyGuardians
}

func appointmentToDomain(row appointmentRow) domain.Appointment {
	return domain.Appointment{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		OrganizerStaffID: row.OrganizerStaffID, Title: row.Title, Description: row.Description, Location: row.Location,
		StartDate: domain.Date(row.StartDate), EndDate: domain.Date(row.EndDate), StartTime: row.StartTime, EndTime: row.EndTime,
		AllDay: row.AllDay, DeliveryMode: row.DeliveryMode, OverviewVisibility: row.OverviewVisibility,
		CancelledAt: row.CancelledAt, DeletedAt: row.DeletedAt, NotifyGuardians: row.NotifyGuardians, Revision: row.Revision}
}

func appointmentsToDomain(rows []appointmentRow) []domain.Appointment {
	result := make([]domain.Appointment, 0, len(rows))
	for _, row := range rows {
		result = append(result, appointmentToDomain(row))
	}
	return result
}

func targetsToDomain(rows []targetRow) []domain.AppointmentTarget {
	result := make([]domain.AppointmentTarget, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.AppointmentTarget{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			AppointmentID: row.AppointmentID, TargetType: row.TargetType, TargetID: row.TargetID, TargetValue: row.TargetValue})
	}
	return result
}

func recurrenceRuleFromDomain(value domain.RecurrenceRule) recurrenceRuleRow {
	return recurrenceRuleRow{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AppointmentID: value.AppointmentID, Frequency: value.Frequency, IntervalCount: value.IntervalCount,
		Weekdays: value.Weekdays, MonthDays: value.MonthDays, EndsOn: value.EndsOn,
		OccurrenceCount: value.OccurrenceCount,
	}
}

func recurrenceRuleToDomain(row recurrenceRuleRow) domain.RecurrenceRule {
	return domain.RecurrenceRule{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		AppointmentID: row.AppointmentID, Frequency: row.Frequency, IntervalCount: row.IntervalCount,
		Weekdays: row.Weekdays, MonthDays: row.MonthDays, EndsOn: row.EndsOn,
		OccurrenceCount: row.OccurrenceCount,
	}
}

func recurrenceRulesToDomain(rows []recurrenceRuleRow) []domain.RecurrenceRule {
	result := make([]domain.RecurrenceRule, 0, len(rows))
	for _, row := range rows {
		result = append(result, recurrenceRuleToDomain(row))
	}
	return result
}

func occurrenceOverrideFromDomain(value domain.AppointmentOccurrenceOverride) occurrenceOverrideRow {
	return occurrenceOverrideRow{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AppointmentID: value.AppointmentID, OccurrenceDate: value.OccurrenceDate, Cancelled: value.Cancelled,
		Title: value.Title, Description: value.Description, Location: value.Location,
		StartDate: value.StartDate, EndDate: value.EndDate,
		StartTime: normalizedWallClockPtr(value.StartTime), EndTime: normalizedWallClockPtr(value.EndTime), AllDay: value.AllDay,
	}
}

func occurrenceOverrideToDomain(row occurrenceOverrideRow) domain.AppointmentOccurrenceOverride {
	return domain.AppointmentOccurrenceOverride{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		AppointmentID: row.AppointmentID, OccurrenceDate: domain.Date(row.OccurrenceDate), Cancelled: row.Cancelled,
		Title: row.Title, Description: row.Description, Location: row.Location,
		StartDate: row.StartDate, EndDate: row.EndDate,
		StartTime: row.StartTime, EndTime: row.EndTime, AllDay: row.AllDay,
	}
}

func occurrenceOverridesToDomain(rows []occurrenceOverrideRow) []domain.AppointmentOccurrenceOverride {
	result := make([]domain.AppointmentOccurrenceOverride, 0, len(rows))
	for _, row := range rows {
		result = append(result, occurrenceOverrideToDomain(row))
	}
	return result
}

func normalizedWallClockPtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	converted := normalizeWallClock(*value)
	return &converted
}

func normalizeWallClock(value time.Time) time.Time {
	return time.Date(1, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

func withTenant[Q interface{ Where(string, ...any) Q }](query Q, alias string, tenantID int64) Q {
	if tenantID <= 0 {
		return query
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
	FindAppointment(context.Context, int64, bool) (domain.Appointment, bool, domain.OperationStats, error)
} = (*Store)(nil)
