package domain

const (
	// MinSickNoteRetentionDays is the lower bound of a school's retention
	// window for sick notes (AU-Bescheinigungen, #3961). A shorter window is
	// refused before anything is deleted; the database function refuses it as
	// well.
	MinSickNoteRetentionDays = 90
	// MaxSickNoteRetentionDays is the upper bound of a school's retention
	// window for sick notes. A longer window is refused so a malformed stored
	// setting cannot retain health data beyond the configured frame.
	MaxSickNoteRetentionDays = 1095
)
