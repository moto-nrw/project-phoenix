package workforce

import (
	"context"
	"log/slog"
	"time"
)

// The ports of the payroll dispatch (#4805): what Workforce needs from the
// Settings Platform, Export Transfer, Delivery, Organisation & Tenancy and
// the People Directory. Workforce owns them; the root binds each to one
// owner.

// PayrollDispatchConfig is the Lohnbüro of the school.
type PayrollDispatchConfig struct {
	Auto    bool
	Channel string
	Format  string
	Email   string
}

// PayrollDispatchSettings answers the Lohnbüro settings of the tenant in the
// context. The root binds it to the Settings Platform.
type PayrollDispatchSettings interface {
	PayrollDispatchConfig(ctx context.Context) (PayrollDispatchConfig, error)
}

// PayrollTransferRequest is one SFTP send of the month file. Prepare builds
// the file in the transfer's own transaction, with its access audit.
type PayrollTransferRequest struct {
	Format         string
	Period         string
	Prepare        func(context.Context) (filename string, data []byte, err error)
	ActorAccountID int64
	ActorName      string
}

// PayrollTransferOutcome is the result of one SFTP attempt.
type PayrollTransferOutcome struct {
	Transferred bool
	// Uncertain means the remote operation may have succeeded, but its journal
	// settlement did not persist. It must never be retried automatically.
	Uncertain bool
	// NotConfigured: the counterpart is off or incomplete; nothing was sent
	// and nothing was journaled.
	NotConfigured bool
	Reason        string
}

// PayrollEmailResult is one settled e-mail send for the transfer journal.
type PayrollEmailResult struct {
	Format, Period, Filename string
	ByteSize                 int64
	Recipient                string
	Delivered                bool
	ActorAccountID           int64
	ActorName                string
}

// PayrollTransfer is the Export Transfer journal and its SFTP transport.
type PayrollTransfer interface {
	Transfer(context.Context, PayrollTransferRequest) (PayrollTransferOutcome, error)
	RecordEmail(context.Context, PayrollEmailResult) error
}

// PayrollExport builds the month file; it is Workforce's own time export.
type PayrollExport interface {
	ExportStaffTime(ctx context.Context, request TimeExportRequest, actorAccountID int64, actorRole string) (*ExportFile, error)
	DatevReport(ctx context.Context, request TimeExportRequest) (*DatevExportReport, error)
}

// PayrollTokens creates download link tokens and their fingerprints.
type PayrollTokens interface {
	NewToken() (raw, fingerprint string, err error)
	Fingerprint(raw string) string
}

// PayrollMailMessage is one rendered-on-send mail for Delivery's outbox.
type PayrollMailMessage struct {
	TenantID       int64
	ToName         string
	ToAddress      string
	Subject        string
	Template       string
	Content        PayrollMailContent
	Type           string
	ReferenceID    int64
	IdempotencyKey string
	// ExpiresAt drops a mail that was not sent before its link expired.
	ExpiresAt   time.Time
	MaxAttempts int
	Backoff     []time.Duration
}

// PayrollMailContent is what the two payroll mail templates show. The link
// mail uses the school, the month, the link and its expiry; the notice the
// first name, the month and the reason.
type PayrollMailContent struct {
	LogoURL    string
	SchoolName string
	FirstName  string
	MonthLabel string
	Correction bool
	LinkURL    string
	ExpiresAt  string
	ReasonText string
}

// Mail states Delivery's answer maps to.
const (
	PayrollMailPending = "pending"
	PayrollMailSent    = "sent"
	PayrollMailFailed  = "failed"
)

// PayrollMail is Delivery's durable outbox. Enqueue joins the caller's
// transaction; State reads the mail's state by its outbox ID.
type PayrollMail interface {
	Enqueue(context.Context, PayrollMailMessage) (outboxID int64, err error)
	State(ctx context.Context, outboxID int64) (string, error)
}

// PayrollSchool is what the mails name of the school.
type PayrollSchool struct {
	Name      string
	Subdomain string
}

// PayrollSchools reads the school of a tenant.
type PayrollSchools interface {
	School(ctx context.Context, tenantID int64) (PayrollSchool, error)
}

// PayrollContact is one person the notice goes to.
type PayrollContact struct {
	StaffID   int64
	FirstName string
	LastName  string
	Email     string
}

// PayrollManagers lists the staff who manage time tracking.
type PayrollManagers interface {
	TimeTrackingManagers(context.Context) ([]PayrollContact, error)
}

// PayrollDispatchPorts are the collaborators of the payroll dispatch.
// Every port is required; Now is optional and pins the clock in tests.
type PayrollDispatchPorts struct {
	Settings PayrollDispatchSettings
	Export   PayrollExport
	Transfer PayrollTransfer
	Mail     PayrollMail
	Schools  PayrollSchools
	Managers PayrollManagers
	Tokens   PayrollTokens
	// FrontendURL is the base of the download link and the notice's link.
	FrontendURL string
	Logger      *slog.Logger
	Now         func() time.Time
}
