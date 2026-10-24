package ports

import (
	"context"
	"time"
)

// SickNoteRetentionStore deletes the school's sick notes uploaded before the
// instant together with their files (#3961). It runs on the school's tenant
// transaction.
type SickNoteRetentionStore interface {
	DeleteStaffSickNotesUploadedBefore(ctx context.Context, before time.Time) (int, error)
}

// SickNoteRetentionSettings resolves the retention window of the school in
// context in days.
type SickNoteRetentionSettings interface {
	StaffSickNoteRetentionDays(ctx context.Context) (int, error)
}
