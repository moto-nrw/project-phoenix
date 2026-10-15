package domain

import "time"

// CleanupRetentionWindow is the fixed retention of a finished staff cleanup
// job (#4046): once it is done, the job keeps only the staff id and the
// stored file name and goes 30 days later.
const CleanupRetentionWindow = 30 * 24 * time.Hour
