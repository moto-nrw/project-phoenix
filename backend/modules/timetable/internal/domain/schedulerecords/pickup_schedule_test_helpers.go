//go:build testhooks

package schedulerecords

import "errors"

// Validate ensures pickup note data is valid
func (n *StudentPickupNote) Validate() error {
	if n.StudentID <= 0 {
		return errors.New(errMsgStudentIDRequired)
	}
	if n.Weekday == 0 && n.NoteDate.IsZero() {
		return errors.New("note_date is required")
	}
	if n.Weekday != 0 && !n.NoteDate.IsZero() {
		return errors.New("a note is either dated or recurring, not both")
	}
	if n.Weekday != 0 && (n.Weekday < WeekdayMonday || n.Weekday > WeekdayFriday) {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if n.Content == "" {
		return errors.New("content is required")
	}
	if len(n.Content) > scheduleNotesMaxLength {
		return errors.New("content cannot exceed 500 characters")
	}
	if n.CreatedBy <= 0 {
		return errors.New(errMsgCreatedByRequired)
	}
	return nil
}
