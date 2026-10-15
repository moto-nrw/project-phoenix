package domain

// MinSickNoteRetentionDays is the lower bound of a school's retention window
// for sick notes (AU-Bescheinigungen, #3961). A shorter window is refused
// before anything is deleted; the database function refuses it as well.
const MinSickNoteRetentionDays = 90
