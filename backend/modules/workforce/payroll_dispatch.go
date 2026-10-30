package workforce

import (
	"context"
	"errors"
	"time"
)

// Export ans Lohnbüro (#4805): after the Monatsabschluss the month file goes
// to the school's payroll office on its own. The close queues one dispatch in
// its transaction; the worker builds the file and sends it by SFTP or as an
// e-mail with a one-time download link.

// Dispatch states. pending waits for the worker, mailing waits for the mail
// server's answer; sent, blocked and failed are final.
const (
	PayrollDispatchPending = "pending"
	PayrollDispatchMailing = "mailing"
	PayrollDispatchSent    = "sent"
	PayrollDispatchBlocked = "blocked"
	PayrollDispatchFailed  = "failed"
)

// Dispatch channels and formats; the values of the payroll.dispatch_*
// settings.
const (
	PayrollChannelSFTP  = "sftp"
	PayrollChannelEmail = "email"
)

// Reasons of a blocked or failed dispatch. Stable codes the /payroll page
// maps to German sentences. A failed SFTP attempt carries the transfer
// reason of the Export Transfer journal (connection_failed, host_key_mismatch,
// ...) instead.
const (
	// PayrollReasonDatevIncomplete: the DATEV check found staff without a
	// Personalnummer, Lohnarten without a number or a missing DATEV client.
	PayrollReasonDatevIncomplete = "datev_incomplete"
	// PayrollReasonSFTPNotConfigured: the SFTP way is chosen, but the
	// counterpart in the settings is off or incomplete.
	PayrollReasonSFTPNotConfigured = "sftp_not_configured"
	// PayrollReasonEmailMissing: the e-mail way is chosen without an address.
	PayrollReasonEmailMissing = "email_missing"
	// PayrollReasonMailFailed: the mail server did not accept the link mail.
	PayrollReasonMailFailed = "mail_failed"
	// PayrollReasonInternal: the file could not be built or recorded.
	PayrollReasonInternal = "internal_error"
)

// PayrollDispatchMaxAttempts is the first send and up to three retries.
const PayrollDispatchMaxAttempts = 4

// PayrollDownloadLifetime is how long the e-mailed link works.
const PayrollDownloadLifetime = 14 * 24 * time.Hour

// ErrPayrollDownloadUnavailable marks a download link that is unknown,
// expired or already used. The three are not told apart: the answer must not
// help guessing links.
var ErrPayrollDownloadUnavailable = errors.New("payroll download link is not available")

// PayrollDispatchTrigger is the committed month close that may queue a
// dispatch. The actor is the person who closed the month; the file's access
// audit names them.
type PayrollDispatchTrigger struct {
	Year, Month    int
	ClosedStaff    int
	ActorAccountID int64
	ActorName      string
}

// PayrollDispatchView is one dispatch as /payroll shows it.
type PayrollDispatchView struct {
	Year              int        `json:"year"`
	Month             int        `json:"month"`
	Correction        bool       `json:"correction"`
	Channel           string     `json:"channel"`
	Format            string     `json:"format"`
	Recipient         string     `json:"recipient,omitempty"`
	State             string     `json:"state"`
	Reason            string     `json:"reason,omitempty"`
	Attempts          int        `json:"attempts"`
	Filename          string     `json:"filename,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	DownloadExpiresAt *time.Time `json:"download_expires_at,omitempty"`
	DownloadedAt      *time.Time `json:"downloaded_at,omitempty"`
}

// PayrollDispatchOverview is the dispatch block of /payroll: the newest
// dispatch, nil before the first one.
type PayrollDispatchOverview struct {
	Latest *PayrollDispatchView `json:"latest"`
}

// PayrollDownload is the file behind a download link.
type PayrollDownload struct {
	Filename    string
	ContentType string
	Data        []byte
}

// PayrollDispatch is the automatic send of the month file.
type PayrollDispatch interface {
	// QueueAfterMonthClose records a dispatch in the close's transaction when
	// the school switched the automatic send on and the close froze at least
	// one person. A later close of the same month queues a correction.
	QueueAfterMonthClose(context.Context, PayrollDispatchTrigger) error
	// Overview reads the newest dispatch of the school.
	Overview(context.Context) (PayrollDispatchOverview, error)
	// RunDue sends the due dispatches of the tenant in the context, settles
	// mailed links and purges expired files. It reports the dispatches it
	// finished.
	RunDue(context.Context) (int, error)
	// Download opens a link once. tenantID comes from the link itself.
	Download(ctx context.Context, tenantID int64, token string) (PayrollDownload, error)
}
