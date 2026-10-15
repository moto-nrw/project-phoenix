package filestorage

import "context"

// CleanupRetention deletes the finished cleanup intents of the school in
// context after the fixed window of the retention list (#4046): an intent of
// the file storage or of an announcement attachment goes 30 days after its
// bytes were removed. Intents still waiting for their bytes stay.
type CleanupRetention interface {
	// DeleteFinishedCleanups reports the intents it deleted and the window in
	// days.
	DeleteFinishedCleanups(ctx context.Context) (deleted, retentionDays int, err error)
}
