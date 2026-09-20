package careplan

import (
	"errors"
	"unicode/utf8"
)

func validateExceptionAuthor(source string, createdBy int64, createdByGuardian *int64) error {
	switch source {
	case "", ExceptionSourceStaff:
		if createdBy <= 0 {
			return errors.New("created_by is required")
		}
	case ExceptionSourceGuardian:
		if createdByGuardian == nil || *createdByGuardian <= 0 {
			return errors.New("created_by_guardian is required for guardian-authored exceptions")
		}
	default:
		return errors.New("invalid exception source")
	}
	return nil
}

func (s *PickupSchedule) Validate() error {
	if s.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if s.Weekday < 1 || s.Weekday > 5 {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if s.PickupTime.IsZero() {
		return errors.New("pickup_time is required")
	}
	if s.CreatedBy <= 0 && s.Source != ScheduleSourceCareOffering {
		return errors.New("created_by is required")
	}
	if s.Notes != nil && len(*s.Notes) > 500 {
		return errors.New("notes cannot exceed 500 characters")
	}
	switch s.Source {
	case "":
		s.Source = ScheduleSourceStaff
	case ScheduleSourceStaff:
		// no offering reference required
	case ScheduleSourceCareOffering:
		if s.CareOfferingID == nil || *s.CareOfferingID <= 0 {
			return errors.New("care_offering_id is required for care_offering-sourced rows")
		}
	default:
		return errors.New("invalid pickup schedule source")
	}
	return nil
}

func (e *PickupException) Validate() error {
	if e.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if e.ExceptionDate.IsZero() {
		return errors.New("exception_date is required")
	}
	if e.Reason != nil && utf8.RuneCountInString(*e.Reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	if e.ExcusedReason != nil && utf8.RuneCountInString(*e.ExcusedReason) > 255 {
		return errors.New("excused_reason cannot exceed 255 characters")
	}
	if err := e.validateExcusalFields(); err != nil {
		return err
	}
	if err := validateExceptionAuthor(e.Source, e.CreatedBy, e.CreatedByGuardian); err != nil {
		return err
	}
	return nil
}

func (n *PickupNote) Validate() error {
	if n.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if n.NoteDate.IsZero() {
		return errors.New("note_date is required")
	}
	if n.Content == "" {
		return errors.New("content is required")
	}
	if len(n.Content) > 500 {
		return errors.New("content cannot exceed 500 characters")
	}
	if n.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	return nil
}

func (r *PickupSchedule) SetTenantID(id int64)  { r.TenantID = id }
func (r *PickupException) SetTenantID(id int64) { r.TenantID = id }
func (r *PickupNote) SetTenantID(id int64)      { r.TenantID = id }

func (e *PickupException) validateExcusalFields() error {
	switch {
	case e.ExcusedFrom == nil:
		if e.ExcusedReason != nil || e.ExcusedCreatedBy != nil || e.ExcusedOwnsPickupTime || e.ExcusedAuto {
			return errors.New("partial absence metadata requires excused_from")
		}
	case e.ExcusedAuto:
		if e.ExcusedCreatedBy != nil {
			return errors.New("auto partial absences must not carry excused_created_by")
		}
		if e.ExcusedOwnsPickupTime {
			return errors.New("auto partial absences never own the pickup time")
		}
		if e.PickupTime == nil {
			return errors.New("auto partial absences require a pickup time")
		}
	case e.ExcusedCreatedBy == nil || *e.ExcusedCreatedBy <= 0:
		return errors.New("excused_created_by is required for partial absences")
	case e.ExcusedOwnsPickupTime && e.PickupTime == nil:
		return errors.New("owned partial-absence pickup time is required")
	}
	return nil
}
