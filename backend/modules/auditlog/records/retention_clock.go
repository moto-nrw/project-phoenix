package records

import "time"

// RetentionClockMargin keeps a retention cutoff inside the window the
// database function accepts when the database clock runs slightly behind the
// application's. A row is deleted at most one run later for it.
const RetentionClockMargin = time.Minute
