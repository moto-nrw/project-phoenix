package domain

import "time"

// CleanupRetentionWindow is the fixed retention of a finished cleanup intent
// (#4046): once its bytes are gone, the intent keeps only the stored file
// name and goes 30 days later.
const CleanupRetentionWindow = 30 * 24 * time.Hour
