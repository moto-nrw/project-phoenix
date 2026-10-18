package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
	"github.com/uptrace/bun"
)

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

type appointmentRecipientRow struct {
	bun.BaseModel     `bun:"table:calendar.appointment_recipients,alias:appointment_recipient"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	CreatedAt         time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	AppointmentID     int64      `bun:"appointment_id,notnull"`
	RecipientType     string     `bun:"recipient_type,notnull"`
	StaffID           *int64     `bun:"staff_id"`
	GuardianProfileID *int64     `bun:"guardian_profile_id"`
	Status            string     `bun:"status,notnull"`
	RespondedAt       *time.Time `bun:"responded_at"`
}

type appointmentRecipientStudentRow struct {
	bun.BaseModel `bun:"table:calendar.appointment_recipient_students,alias:appointment_recipient_student"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	RecipientID   int64     `bun:"recipient_id,notnull"`
	StudentID     int64     `bun:"student_id,notnull"`
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

func appointmentRecipientToDomain(row appointmentRecipientRow) domain.AppointmentRecipient {
	return domain.AppointmentRecipient{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		AppointmentID: row.AppointmentID, RecipientType: row.RecipientType,
		StaffID: row.StaffID, GuardianProfileID: row.GuardianProfileID,
		Status: row.Status, RespondedAt: row.RespondedAt,
	}
}

func appointmentRecipientsToDomain(rows []appointmentRecipientRow) []domain.AppointmentRecipient {
	result := make([]domain.AppointmentRecipient, 0, len(rows))
	for _, row := range rows {
		result = append(result, appointmentRecipientToDomain(row))
	}
	return result
}

func appointmentRecipientStudentToDomain(row appointmentRecipientStudentRow) domain.AppointmentRecipientStudent {
	return domain.AppointmentRecipientStudent{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		RecipientID: row.RecipientID, StudentID: row.StudentID,
	}
}

func appointmentRecipientStudentsToDomain(rows []appointmentRecipientStudentRow) []domain.AppointmentRecipientStudent {
	result := make([]domain.AppointmentRecipientStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, appointmentRecipientStudentToDomain(row))
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
