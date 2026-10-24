//go:build testhooks

package schedulerecords

import "errors"

// Validate ensures arrival note data is valid
func (n *StudentArrivalNote) Validate() error {
	if n.StudentID <= 0 {
		return errors.New(errMsgArrivalStudentIDRequired)
	}
	if n.NoteDate.IsZero() {
		return errors.New("note_date is required")
	}
	if n.Content == "" {
		return errors.New("content is required")
	}
	if len(n.Content) > scheduleNotesMaxLength {
		return errors.New("content cannot exceed 500 characters")
	}
	if n.CreatedBy <= 0 {
		return errors.New(errMsgArrivalCreatedByRequired)
	}
	return nil
}
