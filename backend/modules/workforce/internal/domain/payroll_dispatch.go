package domain

import "time"

// PayrollDispatch is one row of active.payroll_dispatches (#4805): one file
// for the payroll office. Sequence numbers the closes of the month;
// Correction counts the files already on their way before it, so 0 is the
// first file and 1, 2, ... are corrections.
type PayrollDispatch struct {
	ID             int64
	Year, Month    int
	Sequence       int
	Correction     int
	Channel        string
	Format         string
	Recipient      string
	ActorAccountID int64
	ActorName      string
	State          string
	Attempts       int
	NextAttemptAt  time.Time
	// LeaseToken fences one worker pass; it is set by a claim only.
	LeaseToken string
	Reason     string
	Filename   string
	// ContentType, ByteSize and FileData belong to an e-mail dispatch whose
	// link is still open.
	ContentType       string
	ByteSize          int64
	FileData          []byte
	DownloadTokenHash string
	DownloadExpiresAt *time.Time
	DownloadedAt      *time.Time
	MailOutboxID      *int64
	FinishedAt        *time.Time
	CreatedAt         time.Time
}

// Period names the month as YYYY-MM.
func (d PayrollDispatch) Period() string {
	return time.Date(d.Year, time.Month(d.Month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
}
