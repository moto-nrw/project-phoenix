package calendar

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/uptrace/bun"
)

const (
	tableAppointments                = "calendar.appointments"
	tableRecurrenceRules             = "calendar.recurrence_rules"
	tableAppointmentRecipients       = "calendar.appointment_recipients"
	tableAppointmentRecipientStudent = "calendar.appointment_recipient_students"
	tableAppointmentTargets          = "calendar.appointment_targets"
	tableAppointmentOverrides        = "calendar.appointment_occurrence_overrides"

	tableExprAppointmentsAsAppointment = `calendar.appointments AS "appointment"`
)

type AppointmentRepository struct {
	*base.Repository[*calModels.Appointment]
}

func NewAppointmentRepository(db *bun.DB) calModels.AppointmentRepository {
	repo := base.NewRepository[*calModels.Appointment](db, tableAppointments, "Appointment")
	repo.TenantScoped = true
	return &AppointmentRepository{Repository: repo}
}

func (r *AppointmentRepository) FindByID(ctx context.Context, id int64) (*calModels.Appointment, error) {
	appointment, err := r.FindByIDOrNil(ctx, id)
	if err != nil {
		return nil, err
	}
	return appointment, nil
}

func (r *AppointmentRepository) ListVisibleForStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*calModels.Appointment, error) {
	var rows []*calModels.Appointment
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`("appointment".organizer_staff_id = ? OR EXISTS (
			SELECT 1
			FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id
			  AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ?
			  AND ar.staff_id = ?
		))`, staffID, calModels.RecipientTypeStaff, staffID).
		OrderExpr(`"appointment".start_date ASC, "appointment".start_time ASC, "appointment".id ASC`)

	query = applyAppointmentWindow(query, from, to)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list visible staff calendar appointments: %w", err)
	}
	return rows, nil
}

func (r *AppointmentRepository) ListVisibleForGuardianProfiles(ctx context.Context, guardianProfileIDs []int64, from, to timezone.Date) ([]*calModels.Appointment, error) {
	if len(guardianProfileIDs) == 0 {
		return []*calModels.Appointment{}, nil
	}

	var rows []*calModels.Appointment
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`EXISTS (
			SELECT 1
			FROM calendar.appointment_recipients ar
			WHERE ar.appointment_id = "appointment".id
			  AND ar.tenant_id = "appointment".tenant_id
			  AND ar.recipient_type = ?
			  AND ar.guardian_profile_id IN (?)
		)`, calModels.RecipientTypeGuardianProfile, bun.In(guardianProfileIDs)).
		OrderExpr(`"appointment".start_date ASC, "appointment".start_time ASC, "appointment".id ASC`)

	query = applyAppointmentWindow(query, from, to)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list visible guardian calendar appointments: %w", err)
	}
	return rows, nil
}

func (r *AppointmentRepository) ListOrganizedByStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*calModels.Appointment, error) {
	var rows []*calModels.Appointment
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprAppointmentsAsAppointment).
		Where(`"appointment".organizer_staff_id = ?`, staffID).
		OrderExpr(`"appointment".start_date ASC, "appointment".start_time ASC, "appointment".id ASC`)

	query = applyAppointmentWindow(query, from, to)
	if where, val, ok := base.TenantWhere(ctx, "appointment"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list organized calendar appointments: %w", err)
	}
	return rows, nil
}

func applyAppointmentWindow(query *bun.SelectQuery, from, to timezone.Date) *bun.SelectQuery {
	return query.Where(`(
		("appointment".end_date >= ? AND "appointment".start_date <= ?)
		OR EXISTS (
			SELECT 1
			FROM calendar.recurrence_rules rr
			WHERE rr.appointment_id = "appointment".id
			  AND rr.tenant_id = "appointment".tenant_id
			  AND "appointment".start_date <= ?
			  AND (rr.ends_on IS NULL OR rr.ends_on >= ?)
		)
	)`, from, to, to, from)
}

type RecurrenceRuleRepository struct {
	*base.Repository[*calModels.RecurrenceRule]
}

func NewRecurrenceRuleRepository(db *bun.DB) calModels.RecurrenceRuleRepository {
	repo := base.NewRepository[*calModels.RecurrenceRule](db, tableRecurrenceRules, "RecurrenceRule")
	repo.TenantScoped = true
	return &RecurrenceRuleRepository{Repository: repo}
}

func (r *RecurrenceRuleRepository) FindByAppointmentID(ctx context.Context, appointmentID int64) (*calModels.RecurrenceRule, error) {
	row := new(calModels.RecurrenceRule)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(row).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id = ?`, appointmentID).
		Limit(1)
	if where, val, ok := base.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find calendar recurrence rule: %w", err)
	}
	return row, nil
}

func (r *RecurrenceRuleRepository) FindByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*calModels.RecurrenceRule, error) {
	if len(appointmentIDs) == 0 {
		return []*calModels.RecurrenceRule{}, nil
	}

	var rows []*calModels.RecurrenceRule
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id IN (?)`, bun.In(appointmentIDs))
	if where, val, ok := base.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recurrence rules: %w", err)
	}
	return rows, nil
}

func (r *RecurrenceRuleRepository) DeleteByAppointmentID(ctx context.Context, appointmentID int64) error {
	query := base.GetDB(ctx, r.DB).NewDelete().
		Model((*calModels.RecurrenceRule)(nil)).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id = ?`, appointmentID)
	if where, val, ok := base.TenantWhere(ctx, "recurrence_rule"); ok {
		query = query.Where(where, val)
	}
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete calendar recurrence rule: %w", err)
	}
	return nil
}

type AppointmentRecipientRepository struct {
	db *bun.DB
}

func NewAppointmentRecipientRepository(db *bun.DB) calModels.AppointmentRecipientRepository {
	return &AppointmentRecipientRepository{db: db}
}

func (r *AppointmentRecipientRepository) CreateMany(ctx context.Context, recipients []*calModels.AppointmentRecipient) error {
	if len(recipients) == 0 {
		return nil
	}
	for _, recipient := range recipients {
		if err := recipient.Validate(); err != nil {
			return err
		}
		base.EnsureTenantID(ctx, recipient)
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&recipients).
		ModelTableExpr(tableAppointmentRecipients).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create calendar appointment recipients: %w", err)
	}
	return nil
}

func (r *AppointmentRecipientRepository) ReplaceForAppointment(ctx context.Context, appointmentID int64, recipients []*calModels.AppointmentRecipient) error {
	deleteQuery := base.GetDB(ctx, r.db).NewDelete().
		Model((*calModels.AppointmentRecipient)(nil)).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".appointment_id = ?`, appointmentID)
	if where, val, ok := base.TenantWhere(ctx, "appointment_recipient"); ok {
		deleteQuery = deleteQuery.Where(where, val)
	}
	if _, err := deleteQuery.Exec(ctx); err != nil {
		return fmt.Errorf("replace calendar recipients: delete existing: %w", err)
	}
	for _, recipient := range recipients {
		recipient.AppointmentID = appointmentID
	}
	return r.CreateMany(ctx, recipients)
}

func (r *AppointmentRecipientRepository) FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*calModels.AppointmentRecipient, error) {
	var rows []*calModels.AppointmentRecipient
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".appointment_id = ?`, appointmentID).
		OrderExpr(`"appointment_recipient".recipient_type ASC, "appointment_recipient".id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_recipient"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recipients: %w", err)
	}
	return rows, nil
}

func (r *AppointmentRecipientRepository) UpdateResponse(ctx context.Context, recipientID int64, status string) error {
	switch status {
	case calModels.ResponseStatusPending, calModels.ResponseStatusAccepted, calModels.ResponseStatusDeclined, calModels.ResponseStatusInfo:
	default:
		return fmt.Errorf("invalid calendar recipient response status %q", status)
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*calModels.AppointmentRecipient)(nil)).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Set(`status = ?`, status).
		Where(`"appointment_recipient".id = ?`, recipientID)
	if status == calModels.ResponseStatusAccepted || status == calModels.ResponseStatusDeclined {
		query = query.Set(`responded_at = ?`, time.Now())
	} else {
		query = query.Set(`responded_at = NULL`)
	}
	if where, val, ok := base.TenantWhere(ctx, "appointment_recipient"); ok {
		query = query.Where(where, val)
	}
	res, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update calendar recipient response: %w", err)
	}
	return base.AssertRowsAffected(res, 1, "update calendar recipient response")
}

type AppointmentRecipientStudentRepository struct {
	db *bun.DB
}

func NewAppointmentRecipientStudentRepository(db *bun.DB) calModels.AppointmentRecipientStudentRepository {
	return &AppointmentRecipientStudentRepository{db: db}
}

func (r *AppointmentRecipientStudentRepository) CreateMany(ctx context.Context, links []*calModels.AppointmentRecipientStudent) error {
	if len(links) == 0 {
		return nil
	}
	for _, link := range links {
		base.EnsureTenantID(ctx, link)
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&links).
		ModelTableExpr(tableAppointmentRecipientStudent).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create calendar recipient-student links: %w", err)
	}
	return nil
}

func (r *AppointmentRecipientStudentRepository) FindByRecipientIDs(ctx context.Context, recipientIDs []int64) ([]*calModels.AppointmentRecipientStudent, error) {
	if len(recipientIDs) == 0 {
		return []*calModels.AppointmentRecipientStudent{}, nil
	}
	var rows []*calModels.AppointmentRecipientStudent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_recipient_students AS "appointment_recipient_student"`).
		Where(`"appointment_recipient_student".recipient_id IN (?)`, bun.In(recipientIDs)).
		OrderExpr(`"appointment_recipient_student".recipient_id ASC, "appointment_recipient_student".student_id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_recipient_student"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recipient-student links: %w", err)
	}
	return rows, nil
}

type AppointmentTargetRepository struct {
	db *bun.DB
}

func NewAppointmentTargetRepository(db *bun.DB) calModels.AppointmentTargetRepository {
	return &AppointmentTargetRepository{db: db}
}

func (r *AppointmentTargetRepository) ReplaceForAppointment(ctx context.Context, appointmentID int64, targets []*calModels.AppointmentTarget) error {
	deleteQuery := base.GetDB(ctx, r.db).NewDelete().
		Model((*calModels.AppointmentTarget)(nil)).
		ModelTableExpr(`calendar.appointment_targets AS "appointment_target"`).
		Where(`"appointment_target".appointment_id = ?`, appointmentID)
	if where, val, ok := base.TenantWhere(ctx, "appointment_target"); ok {
		deleteQuery = deleteQuery.Where(where, val)
	}
	if _, err := deleteQuery.Exec(ctx); err != nil {
		return fmt.Errorf("replace calendar targets: delete existing: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}
	for _, target := range targets {
		target.AppointmentID = appointmentID
		base.EnsureTenantID(ctx, target)
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&targets).
		ModelTableExpr(tableAppointmentTargets).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("replace calendar targets: insert: %w", err)
	}
	return nil
}

func (r *AppointmentTargetRepository) FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*calModels.AppointmentTarget, error) {
	var rows []*calModels.AppointmentTarget
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_targets AS "appointment_target"`).
		Where(`"appointment_target".appointment_id = ?`, appointmentID).
		OrderExpr(`"appointment_target".id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_target"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar targets: %w", err)
	}
	return rows, nil
}

type AppointmentOccurrenceOverrideRepository struct {
	*base.Repository[*calModels.AppointmentOccurrenceOverride]
}

func NewAppointmentOccurrenceOverrideRepository(db *bun.DB) calModels.AppointmentOccurrenceOverrideRepository {
	repo := base.NewRepository[*calModels.AppointmentOccurrenceOverride](db, tableAppointmentOverrides, "AppointmentOccurrenceOverride")
	repo.TenantScoped = true
	return &AppointmentOccurrenceOverrideRepository{Repository: repo}
}

func (r *AppointmentOccurrenceOverrideRepository) FindByAppointmentIDsInRange(ctx context.Context, appointmentIDs []int64, from, to timezone.Date) ([]*calModels.AppointmentOccurrenceOverride, error) {
	if len(appointmentIDs) == 0 {
		return []*calModels.AppointmentOccurrenceOverride{}, nil
	}
	var rows []*calModels.AppointmentOccurrenceOverride
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.In(appointmentIDs)).
		Where(`"appointment_occurrence_override".occurrence_date >= ?`, from).
		Where(`"appointment_occurrence_override".occurrence_date <= ?`, to).
		OrderExpr(`"appointment_occurrence_override".occurrence_date ASC, "appointment_occurrence_override".id ASC`)
	if where, val, ok := base.TenantWhere(ctx, "appointment_occurrence_override"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar occurrence overrides: %w", err)
	}
	return rows, nil
}
