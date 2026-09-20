package careplan

import (
	"errors"
	"unicode/utf8"
)

func (s *ArrivalSchedule) Validate() error {
	if s.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if s.Weekday < 1 || s.Weekday > 5 {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if s.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	if s.Notes != nil && len(*s.Notes) > 500 {
		return errors.New("notes cannot exceed 500 characters")
	}
	return nil
}

func (e *ArrivalException) Validate() error {
	if e.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if e.ExceptionDate.IsZero() {
		return errors.New("exception_date is required")
	}
	if e.Reason != nil && utf8.RuneCountInString(*e.Reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	if err := validateExceptionAuthor(e.Source, e.CreatedBy, e.CreatedByGuardian); err != nil {
		return err
	}
	return nil
}

func (n *ArrivalNote) Validate() error {
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
func (r *ArrivalSchedule) SetTenantID(id int64) { r.TenantID = id }

func (r *ArrivalException) SetTenantID(id int64) { r.TenantID = id }

func (r *ArrivalNote) SetTenantID(id int64) { r.TenantID = id }
