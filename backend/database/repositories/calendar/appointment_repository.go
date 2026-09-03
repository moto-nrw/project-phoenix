package calendar

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/uptrace/bun"
)

const (
	tableRecurrenceRules             = "calendar.recurrence_rules"
	tableAppointmentRecipients       = "calendar.appointment_recipients"
	tableAppointmentRecipientStudent = "calendar.appointment_recipient_students"
	tableAppointmentOverrides        = "calendar.appointment_occurrence_overrides"
)

type RecurrenceRuleRepository struct {
	runtime Runtime
}

func NewRecurrenceRuleRepository(runtime Runtime) calModels.RecurrenceRuleRepository {
	runtime.validate()
	return &RecurrenceRuleRepository{runtime: runtime}
}

func (r *RecurrenceRuleRepository) Create(ctx context.Context, rule *calModels.RecurrenceRule) error {
	if rule == nil {
		return errors.New("recurrence rule cannot be nil")
	}
	if err := rule.Validate(); err != nil {
		return err
	}
	ensureTenantID(r.runtime, ctx, &rule.TenantID)
	if _, err := r.runtime.Database(ctx).NewInsert().
		Model(rule).
		ModelTableExpr(tableRecurrenceRules).
		Exec(ctx); err != nil {
		return fmt.Errorf("create calendar recurrence rule: %w", err)
	}
	return nil
}

func (r *RecurrenceRuleRepository) FindByAppointmentID(ctx context.Context, appointmentID int64) (*calModels.RecurrenceRule, error) {
	row := new(calModels.RecurrenceRule)
	query := r.runtime.Database(ctx).NewSelect().
		Model(row).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id = ?`, appointmentID).
		Limit(1)
	query = withTenantFilter(r.runtime, ctx, query, "recurrence_rule")

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
	query := r.runtime.Database(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id IN (?)`, bun.List(appointmentIDs))
	query = withTenantFilter(r.runtime, ctx, query, "recurrence_rule")
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recurrence rules: %w", err)
	}
	return rows, nil
}

func (r *RecurrenceRuleRepository) DeleteByAppointmentID(ctx context.Context, appointmentID int64) error {
	query := r.runtime.Database(ctx).NewDelete().
		Model((*calModels.RecurrenceRule)(nil)).
		ModelTableExpr(`calendar.recurrence_rules AS "recurrence_rule"`).
		Where(`"recurrence_rule".appointment_id = ?`, appointmentID)
	query = withTenantFilter(r.runtime, ctx, query, "recurrence_rule")
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete calendar recurrence rule: %w", err)
	}
	return nil
}

type AppointmentRecipientRepository struct {
	runtime Runtime
}

func NewAppointmentRecipientRepository(runtime Runtime) calModels.AppointmentRecipientRepository {
	runtime.validate()
	return &AppointmentRecipientRepository{runtime: runtime}
}

func (r *AppointmentRecipientRepository) CreateMany(ctx context.Context, recipients []*calModels.AppointmentRecipient) error {
	if len(recipients) == 0 {
		return nil
	}
	for _, recipient := range recipients {
		if err := recipient.Validate(); err != nil {
			return err
		}
		ensureTenantID(r.runtime, ctx, &recipient.TenantID)
	}
	_, err := r.runtime.Database(ctx).NewInsert().
		Model(&recipients).
		ModelTableExpr(tableAppointmentRecipients).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create calendar appointment recipients: %w", err)
	}
	return nil
}

func (r *AppointmentRecipientRepository) FindByID(ctx context.Context, id int64) (*calModels.AppointmentRecipient, error) {
	row := new(calModels.AppointmentRecipient)
	query := r.runtime.Database(ctx).NewSelect().
		Model(row).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".id = ?`, id)
	query = withTenantFilter(r.runtime, ctx, query, "appointment_recipient")
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find calendar recipient: %w", err)
	}
	return row, nil
}

func (r *AppointmentRecipientRepository) FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*calModels.AppointmentRecipient, error) {
	var rows []*calModels.AppointmentRecipient
	query := r.runtime.Database(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".appointment_id = ?`, appointmentID).
		OrderExpr(`"appointment_recipient".recipient_type ASC, "appointment_recipient".id ASC`)
	query = withTenantFilter(r.runtime, ctx, query, "appointment_recipient")
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recipients: %w", err)
	}
	return rows, nil
}

func (r *AppointmentRecipientRepository) FindByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*calModels.AppointmentRecipient, error) {
	if len(appointmentIDs) == 0 {
		return nil, nil
	}
	var rows []*calModels.AppointmentRecipient
	query := r.runtime.Database(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Where(`"appointment_recipient".appointment_id IN (?)`, bun.List(appointmentIDs)).
		OrderExpr(`"appointment_recipient".appointment_id ASC, "appointment_recipient".recipient_type ASC, "appointment_recipient".id ASC`)
	query = withTenantFilter(r.runtime, ctx, query, "appointment_recipient")
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recipients by appointments: %w", err)
	}
	return rows, nil
}

func (r *AppointmentRecipientRepository) UpdateResponse(ctx context.Context, recipientID int64, status string) error {
	switch status {
	case calModels.ResponseStatusPending, calModels.ResponseStatusAccepted, calModels.ResponseStatusDeclined, calModels.ResponseStatusInfo:
	default:
		return fmt.Errorf("invalid calendar recipient response status %q", status)
	}

	query := r.runtime.Database(ctx).NewUpdate().
		Model((*calModels.AppointmentRecipient)(nil)).
		ModelTableExpr(`calendar.appointment_recipients AS "appointment_recipient"`).
		Set(`status = ?`, status).
		Where(`"appointment_recipient".id = ?`, recipientID)
	if status == calModels.ResponseStatusAccepted || status == calModels.ResponseStatusDeclined {
		query = query.Set(`responded_at = ?`, time.Now())
	} else {
		query = query.Set(`responded_at = NULL`)
	}
	query = withTenantFilter(r.runtime, ctx, query, "appointment_recipient")
	res, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update calendar recipient response: %w", err)
	}
	return assertRowsAffected(res, 1, "update calendar recipient response")
}

func (r *AppointmentRecipientRepository) ClaimReminderPush(ctx context.Context, appointmentID int64, revision int, occurrenceDate calModels.Date, guardianProfileID int64) (bool, error) {
	if appointmentID <= 0 || revision < 0 || guardianProfileID <= 0 || occurrenceDate.IsZero() {
		return false, errors.New("appointment id, revision, occurrence date, and guardian profile id are required")
	}
	tenantID := r.runtime.TenantID(ctx)
	if tenantID <= 0 {
		return false, errors.New("tenant id is required")
	}

	var claimed bool
	err := r.runtime.Database(ctx).NewRaw(`
		SELECT calendar.claim_appointment_reminder_push_delivery(?, ?, ?, ?)
	`, appointmentID, revision, occurrenceDate, guardianProfileID).Scan(ctx, &claimed)
	if err != nil {
		return false, fmt.Errorf("claim calendar appointment reminder push: %w", err)
	}
	return claimed, nil
}

func (r *AppointmentRecipientRepository) ReleaseReminderPush(ctx context.Context, appointmentID int64, revision int, occurrenceDate calModels.Date, guardianProfileID int64) error {
	if appointmentID <= 0 || revision < 0 || guardianProfileID <= 0 || occurrenceDate.IsZero() {
		return errors.New("appointment id, revision, occurrence date, and guardian profile id are required")
	}
	tenantID := r.runtime.TenantID(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}

	_, err := r.runtime.Database(ctx).NewRaw(`
		SELECT calendar.release_appointment_reminder_push_delivery(?, ?, ?, ?)
	`, appointmentID, revision, occurrenceDate, guardianProfileID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("release calendar appointment reminder push delivery: %w", err)
	}
	return nil
}

type AppointmentRecipientStudentRepository struct {
	runtime Runtime
}

func NewAppointmentRecipientStudentRepository(runtime Runtime) calModels.AppointmentRecipientStudentRepository {
	runtime.validate()
	return &AppointmentRecipientStudentRepository{runtime: runtime}
}

func (r *AppointmentRecipientStudentRepository) CreateMany(ctx context.Context, links []*calModels.AppointmentRecipientStudent) error {
	if len(links) == 0 {
		return nil
	}
	for _, link := range links {
		ensureTenantID(r.runtime, ctx, &link.TenantID)
	}
	_, err := r.runtime.Database(ctx).NewInsert().
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
	query := r.runtime.Database(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_recipient_students AS "appointment_recipient_student"`).
		Where(`"appointment_recipient_student".recipient_id IN (?)`, bun.List(recipientIDs)).
		OrderExpr(`"appointment_recipient_student".recipient_id ASC, "appointment_recipient_student".student_id ASC`)
	query = withTenantFilter(r.runtime, ctx, query, "appointment_recipient_student")
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar recipient-student links: %w", err)
	}
	return rows, nil
}

type AppointmentOccurrenceOverrideRepository struct {
	runtime Runtime
}

func NewAppointmentOccurrenceOverrideRepository(runtime Runtime) calModels.AppointmentOccurrenceOverrideRepository {
	runtime.validate()
	return &AppointmentOccurrenceOverrideRepository{runtime: runtime}
}

func (r *AppointmentOccurrenceOverrideRepository) Create(ctx context.Context, override *calModels.AppointmentOccurrenceOverride) error {
	if override == nil {
		return errors.New("appointment occurrence override cannot be nil")
	}
	ensureTenantID(r.runtime, ctx, &override.TenantID)
	if _, err := r.runtime.Database(ctx).NewInsert().
		Model(override).
		ModelTableExpr(tableAppointmentOverrides).
		Exec(ctx); err != nil {
		return fmt.Errorf("create calendar occurrence override: %w", err)
	}
	return nil
}

func (r *AppointmentOccurrenceOverrideRepository) Update(ctx context.Context, override *calModels.AppointmentOccurrenceOverride) error {
	if override == nil {
		return errors.New("appointment occurrence override cannot be nil")
	}
	query := r.runtime.Database(ctx).NewUpdate().
		Model(override).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		WherePK()
	query = withTenantFilter(r.runtime, ctx, query, "appointment_occurrence_override")
	result, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update calendar occurrence override: %w", err)
	}
	return assertRowsAffected(result, 1, "update calendar occurrence override")
}

func (r *AppointmentOccurrenceOverrideRepository) Delete(ctx context.Context, id any) error {
	query := r.runtime.Database(ctx).NewDelete().
		Model((*calModels.AppointmentOccurrenceOverride)(nil)).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".id = ?`, id)
	query = withTenantFilter(r.runtime, ctx, query, "appointment_occurrence_override")
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete calendar occurrence override: %w", err)
	}
	return nil
}

func (r *AppointmentOccurrenceOverrideRepository) FindCancelledByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*calModels.AppointmentOccurrenceOverride, error) {
	if len(appointmentIDs) == 0 {
		return []*calModels.AppointmentOccurrenceOverride{}, nil
	}
	var rows []*calModels.AppointmentOccurrenceOverride
	query := r.runtime.Database(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".cancelled = ?`, true).
		OrderExpr(`"appointment_occurrence_override".occurrence_date ASC, "appointment_occurrence_override".id ASC`)
	query = withTenantFilter(r.runtime, ctx, query, "appointment_occurrence_override")
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find cancelled calendar occurrence overrides: %w", err)
	}
	return rows, nil
}

// CancelOccurrence marks the occurrence cancelled via an INSERT ... ON CONFLICT
// DO UPDATE, so concurrent cancellations of the same occurrence converge on
// cancelled=true instead of one violating the unique constraint and returning a
// 500.
func (r *AppointmentOccurrenceOverrideRepository) CancelOccurrence(ctx context.Context, appointmentID int64, occurrenceDate calModels.Date) error {
	override := &calModels.AppointmentOccurrenceOverride{
		AppointmentID:  appointmentID,
		OccurrenceDate: occurrenceDate,
		Cancelled:      true,
	}
	ensureTenantID(r.runtime, ctx, &override.TenantID)
	if _, err := r.runtime.Database(ctx).NewInsert().
		Model(override).
		ModelTableExpr(`calendar.appointment_occurrence_overrides`).
		On("CONFLICT (tenant_id, appointment_id, occurrence_date) DO UPDATE").
		Set("cancelled = EXCLUDED.cancelled").
		Set("updated_at = NOW()").
		Exec(ctx); err != nil {
		return fmt.Errorf("cancel calendar occurrence: %w", err)
	}
	return nil
}

func (r *AppointmentOccurrenceOverrideRepository) DeleteByAppointmentID(ctx context.Context, appointmentID int64) error {
	query := r.runtime.Database(ctx).NewDelete().
		Model((*calModels.AppointmentOccurrenceOverride)(nil)).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id = ?`, appointmentID)
	query = withTenantFilter(r.runtime, ctx, query, "appointment_occurrence_override")
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete calendar occurrence overrides: %w", err)
	}
	return nil
}

func (r *AppointmentOccurrenceOverrideRepository) FindByAppointmentIDsAndOccurrenceDates(ctx context.Context, appointmentIDs []int64, occurrenceDates []calModels.Date) ([]*calModels.AppointmentOccurrenceOverride, error) {
	if len(appointmentIDs) == 0 || len(occurrenceDates) == 0 {
		return []*calModels.AppointmentOccurrenceOverride{}, nil
	}
	var rows []*calModels.AppointmentOccurrenceOverride
	query := r.runtime.Database(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".occurrence_date IN (?)`, bun.List(occurrenceDates)).
		OrderExpr(`"appointment_occurrence_override".occurrence_date ASC, "appointment_occurrence_override".id ASC`)
	query = withTenantFilter(r.runtime, ctx, query, "appointment_occurrence_override")
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar occurrence overrides: %w", err)
	}
	return rows, nil
}

func (r *AppointmentOccurrenceOverrideRepository) FindByAppointmentIDsAndStartDates(ctx context.Context, appointmentIDs []int64, startDates []calModels.Date) ([]*calModels.AppointmentOccurrenceOverride, error) {
	if len(appointmentIDs) == 0 || len(startDates) == 0 {
		return []*calModels.AppointmentOccurrenceOverride{}, nil
	}
	var rows []*calModels.AppointmentOccurrenceOverride
	query := r.runtime.Database(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`calendar.appointment_occurrence_overrides AS "appointment_occurrence_override"`).
		Where(`"appointment_occurrence_override".appointment_id IN (?)`, bun.List(appointmentIDs)).
		Where(`"appointment_occurrence_override".start_date IN (?)`, bun.List(startDates)).
		OrderExpr(`"appointment_occurrence_override".start_date ASC, "appointment_occurrence_override".id ASC`)
	query = withTenantFilter(r.runtime, ctx, query, "appointment_occurrence_override")
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find calendar occurrence overrides by start date: %w", err)
	}
	return rows, nil
}
