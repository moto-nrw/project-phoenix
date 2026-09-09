package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Identity is the authenticated caller as the HTTP platform resolved it. The
// composition root derives it from the request's session claims; this package
// never reads the token itself.
type Identity struct {
	// AccountID is 0 when no principal is attached to the request.
	AccountID   int64
	Roles       []string
	Permissions []string
	// DisplayName is the actor's full name, falling back to the username. It
	// is snapshotted into the export transfer journal.
	DisplayName string
}

// IdentityFunc resolves the caller of a request.
type IdentityFunc func(ctx context.Context) Identity

// Export transfer (#3050): the manual SFTP hand-over of the cross-staff time
// export. The capability lives in the Export Transfer platform module; this
// port keeps its credential-free status and outcome shapes on the wire.

const (
	exportTransferKindStaffTimeTracking = "staff_time_tracking"
	exportTransferReasonNotConfigured   = "not_configured"
)

// ExportTransferStatus is the credential-free view of the school's transfer
// configuration; it has no password field at all.
type ExportTransferStatus struct {
	Enabled         bool     `json:"enabled"`
	Ready           bool     `json:"ready"`
	Host            string   `json:"host,omitempty"`
	Port            int      `json:"port,omitempty"`
	RemoteDirectory string   `json:"remote_directory,omitempty"`
	MissingSettings []string `json:"missing_settings,omitempty"`
}

// ExportTransferOutcome is the result of one attempt. A failed attempt is a
// normal result, so it is a value, not an error; callers render Transferred.
type ExportTransferOutcome struct {
	Transferred     bool   `json:"transferred"`
	Filename        string `json:"filename"`
	ByteSize        int64  `json:"byte_size"`
	TargetHost      string `json:"target_host,omitempty"`
	TargetDirectory string `json:"target_directory,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// ExportTransferRequest is one transfer: which export, which file, on whose
// behalf.
type ExportTransferRequest struct {
	Kind           string
	Format         string
	Filename       string
	Data           []byte
	ActorAccountID int64
	ActorName      string
}

// ExportTransfer is the consumer-owned port to the Export Transfer
// capability.
type ExportTransfer interface {
	Status(ctx context.Context) (ExportTransferStatus, error)
	Transfer(ctx context.Context, request ExportTransferRequest) (ExportTransferOutcome, error)
}

// scheduleReader is the part of the Workforce query the schedule views need:
// the assigned template and the running schedule version. workforce.Query
// satisfies it.
type scheduleReader interface {
	FindWorkTimeModel(ctx context.Context, id int64) (workforce.WorkTimeModel, error)
	CurrentStaffSchedule(ctx context.Context, staffID int64) ([]workforce.StaffWorkSchedule, error)
}

// optionalDate turns a capability calendar day ("" = none) into the wire's
// nullable date.
func optionalDate(value string) *timezone.Date {
	if value == "" {
		return nil
	}
	date := timezone.Date(value)
	return &date
}

// optionalDateString turns a nullable wire date into the capability's
// optional calendar day.
func optionalDateString(value *timezone.Date) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}
