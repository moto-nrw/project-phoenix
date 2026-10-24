package workforce

import "context"

// SickNoteRetention deletes the sick notes (AU-Bescheinigungen) of the school
// in context after the window the moto team records for the school (#3961),
// counted from the upload. Their files go through the staff document file
// cleanup; documents of other categories stay until the staff member leaves.
type SickNoteRetention interface {
	// DeleteExpiredStaffSickNotes reports the sick notes it deleted and the
	// window in days.
	DeleteExpiredStaffSickNotes(ctx context.Context) (deleted, retentionDays int, err error)
}
