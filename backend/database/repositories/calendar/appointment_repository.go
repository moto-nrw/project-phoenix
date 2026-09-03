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
	tableAppointmentRecipients       = "calendar.appointment_recipients"
	tableAppointmentRecipientStudent = "calendar.appointment_recipient_students"
)

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
