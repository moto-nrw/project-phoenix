package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
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
	SchoolEventType    *string     `bun:"school_event_type"`
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

func applyAppointmentFields(row *appointmentRow, fields appointments.AppointmentFields) {
	row.OrganizerStaffID = fields.OrganizerStaffID
	row.Title = fields.Title
	row.Description = fields.Description
	row.Location = fields.Location
	row.StartDate = domain.Date(fields.StartDate)
	row.EndDate = domain.Date(fields.EndDate)
	row.StartTime = fields.StartTime
	row.EndTime = fields.EndTime
	row.AllDay = fields.AllDay
	row.DeliveryMode = fields.DeliveryMode
	row.OverviewVisibility = fields.OverviewVisibility
	row.NotifyGuardians = fields.NotifyGuardians
	row.SchoolEventType = fields.SchoolEventType
}

func appointmentToPublic(row appointmentRow) *appointments.Appointment {
	return &appointments.Appointment{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		OrganizerStaffID: row.OrganizerStaffID, Title: row.Title, Description: row.Description, Location: row.Location,
		StartDate: appointments.Date(row.StartDate), EndDate: appointments.Date(row.EndDate), StartTime: row.StartTime, EndTime: row.EndTime,
		AllDay: row.AllDay, DeliveryMode: row.DeliveryMode, OverviewVisibility: row.OverviewVisibility,
		CancelledAt: row.CancelledAt, DeletedAt: row.DeletedAt, NotifyGuardians: row.NotifyGuardians,
		SchoolEventType: row.SchoolEventType, Revision: row.Revision}
}

func appointmentsToPublic(rows []appointmentRow) []*appointments.Appointment {
	result := make([]*appointments.Appointment, 0, len(rows))
	for _, row := range rows {
		result = append(result, appointmentToPublic(row))
	}
	return result
}

func targetsToPublic(rows []targetRow) []*appointments.AppointmentTarget {
	result := make([]*appointments.AppointmentTarget, 0, len(rows))
	for _, row := range rows {
		result = append(result, &appointments.AppointmentTarget{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			AppointmentID: row.AppointmentID, TargetType: row.TargetType, TargetID: row.TargetID, TargetValue: row.TargetValue})
	}
	return result
}

func recurrenceRuleFromPublic(value appointments.RecurrenceRule) recurrenceRuleRow {
	return recurrenceRuleRow{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AppointmentID: value.AppointmentID, Frequency: value.Frequency, IntervalCount: value.IntervalCount,
		Weekdays: value.Weekdays, MonthDays: value.MonthDays, EndsOn: storageDate(value.EndsOn),
		OccurrenceCount: value.OccurrenceCount,
	}
}

func recurrenceRuleToPublic(row recurrenceRuleRow) *appointments.RecurrenceRule {
	return &appointments.RecurrenceRule{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		AppointmentID: row.AppointmentID, Frequency: row.Frequency, IntervalCount: row.IntervalCount,
		Weekdays: row.Weekdays, MonthDays: row.MonthDays, EndsOn: publicDate(row.EndsOn),
		OccurrenceCount: row.OccurrenceCount,
	}
}

func recurrenceRulesToPublic(rows []recurrenceRuleRow) []*appointments.RecurrenceRule {
	result := make([]*appointments.RecurrenceRule, 0, len(rows))
	for _, row := range rows {
		result = append(result, recurrenceRuleToPublic(row))
	}
	return result
}

func occurrenceOverrideFromPublic(value appointments.AppointmentOccurrenceOverride) occurrenceOverrideRow {
	return occurrenceOverrideRow{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AppointmentID: value.AppointmentID, OccurrenceDate: domain.Date(value.OccurrenceDate), Cancelled: value.Cancelled,
		Title: value.Title, Description: value.Description, Location: value.Location,
		StartDate: storageDate(value.StartDate), EndDate: storageDate(value.EndDate),
		StartTime: normalizedWallClockPtr(value.StartTime), EndTime: normalizedWallClockPtr(value.EndTime), AllDay: value.AllDay,
	}
}

func occurrenceOverrideToPublic(row occurrenceOverrideRow) *appointments.AppointmentOccurrenceOverride {
	return &appointments.AppointmentOccurrenceOverride{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		AppointmentID: row.AppointmentID, OccurrenceDate: appointments.Date(row.OccurrenceDate), Cancelled: row.Cancelled,
		Title: row.Title, Description: row.Description, Location: row.Location,
		StartDate: publicDate(row.StartDate), EndDate: publicDate(row.EndDate),
		StartTime: row.StartTime, EndTime: row.EndTime, AllDay: row.AllDay,
	}
}

func occurrenceOverridesToPublic(rows []occurrenceOverrideRow) []*appointments.AppointmentOccurrenceOverride {
	result := make([]*appointments.AppointmentOccurrenceOverride, 0, len(rows))
	for _, row := range rows {
		result = append(result, occurrenceOverrideToPublic(row))
	}
	return result
}

func appointmentRecipientToPublic(row appointmentRecipientRow) *appointments.AppointmentRecipient {
	return &appointments.AppointmentRecipient{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		AppointmentID: row.AppointmentID, RecipientType: row.RecipientType,
		StaffID: row.StaffID, GuardianProfileID: row.GuardianProfileID,
		Status: row.Status, RespondedAt: row.RespondedAt,
	}
}

func appointmentRecipientsToPublic(rows []appointmentRecipientRow) []*appointments.AppointmentRecipient {
	result := make([]*appointments.AppointmentRecipient, 0, len(rows))
	for _, row := range rows {
		result = append(result, appointmentRecipientToPublic(row))
	}
	return result
}

func appointmentRecipientStudentsToPublic(rows []appointmentRecipientStudentRow) []*appointments.AppointmentRecipientStudent {
	result := make([]*appointments.AppointmentRecipientStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, &appointments.AppointmentRecipientStudent{
			ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			RecipientID: row.RecipientID, StudentID: row.StudentID,
		})
	}
	return result
}

func storageDate(value *appointments.Date) *domain.Date {
	if value == nil {
		return nil
	}
	date := domain.Date(*value)
	return &date
}

func storageDates(values []appointments.Date) []domain.Date {
	result := make([]domain.Date, 0, len(values))
	for _, value := range values {
		result = append(result, domain.Date(value))
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
