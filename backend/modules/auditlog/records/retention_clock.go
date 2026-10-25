package records

import "time"

// RetentionClockMargin keeps a retention cutoff inside the window the
// database function accepts when the database clock runs slightly behind the
// application's. A row is deleted at most one run later for it.
const RetentionClockMargin = time.Minute

// The database functions of the fixed audit windows evaluate their
// INTERVAL in UTC (migration 1.15.534), whatever the timezone of the process
// or of the session. The cutoffs below compute the same window in UTC, so a
// clock change in Berlin never lengthens the window the database enforces
// beyond the one the application applies.

// RetentionDaysCutoff is the newest cutoff the database accepts for a window
// of days: days times 24 hours before now, less the clock margin.
func RetentionDaysCutoff(now time.Time, days int) time.Time {
	return now.Add(-time.Duration(days)*24*time.Hour - RetentionClockMargin)
}

// RetentionYearsCutoff is the newest cutoff the database accepts for a
// window of years: the same UTC clock time years before now, less the clock
// margin. Like PostgreSQL's interval arithmetic, and unlike time.AddDate, it
// keeps a day past the end of the target month on the month's last day, so
// 29 February goes back to 28 February rather than ahead to 1 March.
func RetentionYearsCutoff(now time.Time, years int) time.Time {
	utc := now.UTC()
	year, month := utc.Year()-years, utc.Month()
	day := min(utc.Day(), time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day())
	then := time.Date(year, month, day, utc.Hour(), utc.Minute(), utc.Second(), utc.Nanosecond(), time.UTC)
	return then.Add(-RetentionClockMargin)
}
