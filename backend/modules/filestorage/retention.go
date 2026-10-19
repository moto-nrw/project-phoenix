package filestorage

import "context"

// CleanupRetention deletes the technical remains of the school in context
// after the fixed windows of the retention list. DeleteFinishedCleanups
// removes finished cleanup intents (#4046): an intent of the file storage or
// of an announcement attachment goes 30 days after its bytes were removed;
// intents still waiting for their bytes stay. DeleteRemovedFiles removes the
// rows of files the school deleted (#4045): such a row keeps the display
// name and the accounts that uploaded and deleted the file until its bytes
// are gone, and goes 30 days after that. Files in use stay until the school
// deletes them.
type CleanupRetention interface {
	// DeleteFinishedCleanups reports the intents it deleted and the window in
	// days.
	DeleteFinishedCleanups(ctx context.Context) (deleted, retentionDays int, err error)
	// DeleteRemovedFiles reports the rows it deleted and the window in days.
	DeleteRemovedFiles(ctx context.Context) (deleted, retentionDays int, err error)
}
