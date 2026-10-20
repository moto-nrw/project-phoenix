package appointments

import (
	"errors"
	"strings"
	"time"
)

// ValidateRecipientFields checks one resolved audience member: its type, the
// one subject ID that type needs and its response status.
func ValidateRecipientFields(fields AppointmentRecipientFields) error {
	switch fields.RecipientType {
	case RecipientTypeStaff:
		if fields.StaffID == nil || *fields.StaffID <= 0 || fields.GuardianProfileID != nil {
			return invalid("staff recipient requires staff_id only")
		}
		if len(fields.StudentIDs) > 0 {
			return invalid("staff recipient cannot have student IDs")
		}
	case RecipientTypeGuardianProfile:
		if fields.GuardianProfileID == nil || *fields.GuardianProfileID <= 0 || fields.StaffID != nil {
			return invalid("guardian recipient requires guardian_profile_id only")
		}
	default:
		return invalid("invalid recipient_type")
	}
	if !ValidResponseStatus(fields.Status) {
		return invalid("invalid recipient status")
	}
	return nil
}

// ValidResponseStatus reports whether status is one of the ResponseStatus
// values.
func ValidResponseStatus(status string) bool {
	switch status {
	case ResponseStatusPending, ResponseStatusAccepted, ResponseStatusDeclined, ResponseStatusInfo:
		return true
	default:
		return false
	}
}

// ValidateAppointmentFields checks the fields of an appointment and returns
// them normalized: the title trimmed and the overview visibility defaulted to
// the organizer.
func ValidateAppointmentFields(fields AppointmentFields) (AppointmentFields, error) {
	if fields.OrganizerStaffID <= 0 {
		return fields, invalid("organizer_staff_id is required")
	}
	fields.Title = strings.TrimSpace(fields.Title)
	if fields.Title == "" {
		return fields, invalid("title is required")
	}
	if fields.StartDate.IsZero() {
		return fields, invalid("start_date is required")
	}
	if fields.EndDate.IsZero() {
		return fields, invalid("end_date is required")
	}
	if fields.EndDate.Before(fields.StartDate) {
		return fields, invalid("end_date must be on or after start_date")
	}
	if !fields.AllDay && !normalizeWallClock(fields.EndTime).After(normalizeWallClock(fields.StartTime)) && fields.StartDate == fields.EndDate {
		return fields, invalid("end_time must be after start_time on same-day appointments")
	}
	switch fields.DeliveryMode {
	case DeliveryModeRSVPRequired, DeliveryModeInformational:
	default:
		return fields, invalid("delivery_mode must be rsvp_required or informational")
	}
	if fields.OverviewVisibility == "" {
		fields.OverviewVisibility = OverviewVisibilityOrganizer
	}
	switch fields.OverviewVisibility {
	case OverviewVisibilityOrganizer, OverviewVisibilityStaff, OverviewVisibilityAll:
		return fields, nil
	default:
		return fields, invalid("overview_visibility must be organizer, staff, or all")
	}
}

func fieldsFromAppointment(value *Appointment) AppointmentFields {
	return AppointmentFields{
		OrganizerStaffID: value.OrganizerStaffID, Title: value.Title, Description: value.Description,
		Location: value.Location, StartDate: value.StartDate, EndDate: value.EndDate,
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
		DeliveryMode: value.DeliveryMode, OverviewVisibility: value.OverviewVisibility,
		NotifyGuardians: value.NotifyGuardians,
	}
}

func applyFields(value *Appointment, fields AppointmentFields) {
	value.OrganizerStaffID = fields.OrganizerStaffID
	value.Title = fields.Title
	value.Description = fields.Description
	value.Location = fields.Location
	value.StartDate = fields.StartDate
	value.EndDate = fields.EndDate
	value.StartTime = fields.StartTime
	value.EndTime = fields.EndTime
	value.AllDay = fields.AllDay
	value.DeliveryMode = fields.DeliveryMode
	value.OverviewVisibility = fields.OverviewVisibility
	value.NotifyGuardians = fields.NotifyGuardians
}

func normalizeWallClock(value time.Time) time.Time {
	return time.Date(1, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

func normalizeWeekdays(values []string) ([]string, error) {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if !validRecurrenceWeekdays[normalized] {
			return nil, invalid("weekdays must be valid day names (monday–sunday)")
		}
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result, nil
}

func normalizeMonthDays(values []int) ([]int, error) {
	seen := make(map[int]bool, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value < 1 || value > 31 {
			return nil, invalid("month_days must be between 1 and 31")
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result, nil
}

func invalid(reason string) error { return &InvalidError{Reason: reason} }

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrAppointmentNotFound), errors.Is(err, ErrAppointmentRecipientNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidAppointment):
		return "invalid"
	case errors.Is(err, ErrAppointmentLifecycleConflict):
		return "lifecycle_conflict"
	default:
		return "internal_error"
	}
}
